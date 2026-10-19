package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/queue"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

const (
	// QueueCritical is the high priority banking sync queue.
	QueueCritical = queue.QueueCritical
	// QueueDefault is the statement ingestion queue.
	QueueDefault = queue.QueueDefault
	// QueueLow is the background categorization queue.
	QueueLow = queue.QueueLow
)

type serverLifecycle interface {
	Start(asynq.Handler) error
	Stop()
	Shutdown()
}

// WorkerServer owns the Asynq server and its immutable handler registry.
type WorkerServer struct {
	server  serverLifecycle
	mux     *asynq.ServeMux
	logger  *slog.Logger
	mu      sync.Mutex
	started bool
	closed  bool
	once    sync.Once
}

// NewWorkerServer binds application ports to all three canonical task types.
func NewWorkerServer(cfg Config, handlers TaskHandlers, logger *slog.Logger) (*WorkerServer, error) {
	if cfg.Concurrency < 1 || cfg.Concurrency > 1024 || cfg.ShutdownTimeout <= 0 || cfg.ShutdownTimeout > 30*time.Second {
		return nil, fmt.Errorf("invalid worker concurrency or shutdown timeout")
	}
	if logger == nil {
		return nil, fmt.Errorf("worker logger is required")
	}
	mux := asynq.NewServeMux()
	mux.HandleFunc(string(ports.TaskTypeIngestStatement), handlers.HandleIngestStatement)
	mux.HandleFunc(string(ports.TaskTypeSyncOpenFinance), handlers.HandleSyncOpenFinance)
	mux.HandleFunc(string(ports.TaskTypeCategorizeTransaction), handlers.HandleCategorizeTransaction)
	return &WorkerServer{
		server: asynq.NewServer(cfg.Redis, serverConfig(cfg, logger)),
		mux:    mux, logger: logger,
	}, nil
}

func serverConfig(cfg Config, logger *slog.Logger) asynq.Config {
	return asynq.Config{
		Concurrency:     cfg.Concurrency,
		Queues:          map[string]int{QueueCritical: 6, QueueDefault: 3, QueueLow: 1},
		StrictPriority:  false,
		ShutdownTimeout: cfg.ShutdownTimeout,
		RetryDelayFunc:  CalculateRetryDelay,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			retried, hasRetried := asynq.GetRetryCount(ctx)
			maxRetry, hasMax := asynq.GetMaxRetry(ctx)
			message := "task failed; retry scheduled"
			if errors.Is(err, asynq.SkipRetry) || (hasRetried && hasMax && retried >= maxRetry) {
				message = "[DLQ] task will be archived"
			}
			// Neither payload nor application errors are logged: both can carry PII.
			logger.ErrorContext(ctx, message, "task_type", task.Type(), "retry_count", retried, "max_retry", maxRetry)
		}),
	}
}

// Start starts processing once without installing signal handlers.
// It is safe to call concurrently with Start or Shutdown; a closed worker
// cannot be restarted.
func (s *WorkerServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("start worker: %w", asynq.ErrServerClosed)
	}
	if s.started {
		return fmt.Errorf("start worker: worker is already running")
	}
	if err := s.server.Start(s.mux); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	s.started = true
	return nil
}

// Run starts the server and drains in-flight jobs when ctx is canceled.
// Task contexts deliberately do not inherit ctx: a termination signal must
// stop dequeueing without prematurely canceling in-flight application work.
func (s *WorkerServer) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("run worker: %w", err)
	}
	if err := s.Start(); err != nil {
		return err
	}
	s.logger.Info("worker started")
	<-ctx.Done()
	s.Shutdown()
	return nil
}

// Shutdown stops fetching first, then lets Asynq drain within ShutdownTimeout.
// Concurrent calls wait for the same drain to finish.
func (s *WorkerServer) Shutdown() {
	s.once.Do(func() {
		s.stopFetching()
		s.logger.Info("worker draining")
		s.server.Shutdown()
		s.logger.Info("worker stopped")
	})
}

func (s *WorkerServer) stopFetching() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Serialize Stop with Start so no processors can start after Stop returns.
	// Mark closed even if Shutdown is called before the first Start.
	s.closed = true
	s.server.Stop()
}
