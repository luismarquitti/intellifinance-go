package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func testFixtures(t *testing.T) map[ports.TaskType]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile("../../test/fixtures/task_payloads.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[ports.TaskType]json.RawMessage
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Redis.Addr != "192.168.3.10:6379" || cfg.Concurrency != 10 || cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("unexpected defaults: addr=%s concurrency=%d shutdown=%v", cfg.Redis.Addr, cfg.Concurrency, cfg.ShutdownTimeout)
	}
	if cfg.Redis.DialTimeout <= 0 || cfg.Redis.ReadTimeout <= 0 || cfg.Redis.WriteTimeout <= 0 {
		t.Fatal("Redis I/O must have finite timeouts")
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	values := map[string]string{
		"ENVIRONMENT": "development", "REDIS_URL": "rediss://synthetic:uri-password@localhost:6380/2",
		"REDIS_PASSWORD": "synthetic-override", "REDIS_DB": "3", "WORKER_CONCURRENCY": "64", "WORKER_SHUTDOWN_TIMEOUT_SECONDS": "15",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Redis.Addr != "localhost:6380" || cfg.Redis.DB != 3 || cfg.Redis.Password != values["REDIS_PASSWORD"] || cfg.Redis.Username != "synthetic" || cfg.Redis.TLSConfig == nil || cfg.Concurrency != 64 || cfg.ShutdownTimeout != 15*time.Second {
		t.Fatal("environment overrides or TLS configuration were lost")
	}
	cfg, err = loadConfig(func(key string) string {
		if key == "REDIS_ADDR" {
			return "localhost:6379"
		}
		return ""
	})
	if err != nil || cfg.Redis.Addr != "localhost:6379" {
		t.Fatalf("REDIS_ADDR compatibility: %v", err)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"WORKER_CONCURRENCY", "0"}, {"WORKER_CONCURRENCY", "-1"}, {"WORKER_CONCURRENCY", "1025"}, {"WORKER_CONCURRENCY", "many"},
		{"WORKER_SHUTDOWN_TIMEOUT_SECONDS", "0"}, {"WORKER_SHUTDOWN_TIMEOUT_SECONDS", "31"},
		{"REDIS_DB", "-1"}, {"REDIS_DB", "not-an-int"},
		{"REDIS_URL", "localhost"}, {"REDIS_URL", ":6379"}, {"REDIS_URL", "localhost:0"}, {"REDIS_URL", "localhost:65536"},
		{"REDIS_URL", "https://synthetic:secret@localhost:6379"}, {"REDIS_URL", "redis://synthetic:secret@localhost:6379/not-a-db"},
		{"ENVIRONMENT", "production"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := loadConfig(func(key string) string {
				if key == tc.key {
					return tc.value
				}
				return ""
			})
			if err == nil {
				t.Fatal("expected configuration error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("configuration error leaked Redis credentials")
			}
		})
	}
}

func TestServerConfiguration(t *testing.T) {
	cfg := Config{Concurrency: 10, ShutdownTimeout: 30 * time.Second}
	actual := serverConfig(cfg, testLogger())
	if !reflect.DeepEqual(actual.Queues, map[string]int{QueueCritical: 6, QueueDefault: 3, QueueLow: 1}) || actual.StrictPriority {
		t.Fatalf("unexpected queue policy: %v strict=%v", actual.Queues, actual.StrictPriority)
	}
	if actual.Concurrency != 10 || actual.ShutdownTimeout != 30*time.Second || actual.RetryDelayFunc == nil || actual.ErrorHandler == nil {
		t.Fatal("incomplete server policy")
	}
	if _, err := NewWorkerServer(Config{}, TaskHandlers{}, testLogger()); err == nil {
		t.Fatal("zero config must be rejected")
	}
	if _, err := NewWorkerServer(cfg, TaskHandlers{}, nil); err == nil {
		t.Fatal("nil logger must be rejected")
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	for _, tc := range []struct {
		n    int
		base time.Duration
	}{
		{-1, 5 * time.Second}, {0, 5 * time.Second}, {1, 10 * time.Second},
		{2, 20 * time.Second}, {3, 40 * time.Second}, {4, 80 * time.Second},
		{5, 160 * time.Second}, {6, 300 * time.Second}, {math.MaxInt, 300 * time.Second},
	} {
		t.Run(fmt.Sprint(tc.n), func(t *testing.T) {
			for range 100 {
				delay := CalculateRetryDelay(tc.n, errors.New("transient"), nil)
				maximum := tc.base + tc.base/5
				if maximum > 300*time.Second {
					maximum = 300 * time.Second
				}
				if delay < tc.base-tc.base/5 || delay > maximum {
					t.Fatalf("delay %v outside jitter/cap bounds for base %v", delay, tc.base)
				}
			}
		})
	}
	if delay := CalculateRetryDelay(0, fmt.Errorf("permanent: %w", asynq.SkipRetry), nil); delay != 0 {
		t.Fatalf("SkipRetry must not wait, got %v", delay)
	}
}

type applicationStub struct {
	call func(context.Context, ports.TaskType, any) error
}

func (s applicationStub) IngestStatement(ctx context.Context, payload ports.IngestStatementPayload) error {
	return s.call(ctx, ports.TaskTypeIngestStatement, payload)
}

func (s applicationStub) SyncOpenFinance(ctx context.Context, payload ports.SyncOpenFinancePayload) error {
	return s.call(ctx, ports.TaskTypeSyncOpenFinance, payload)
}

func (s applicationStub) CategorizeTransaction(ctx context.Context, payload ports.CategorizeTransactionPayload) error {
	return s.call(ctx, ports.TaskTypeCategorizeTransaction, payload)
}

func handlerFunctions(h TaskHandlers) map[ports.TaskType]asynq.HandlerFunc {
	return map[ports.TaskType]asynq.HandlerFunc{
		ports.TaskTypeIngestStatement:       h.HandleIngestStatement,
		ports.TaskTypeSyncOpenFinance:       h.HandleSyncOpenFinance,
		ports.TaskTypeCategorizeTransaction: h.HandleCategorizeTransaction,
	}
}

func TestHandlersRejectPermanentPayloadErrors(t *testing.T) {
	stub := applicationStub{call: func(context.Context, ports.TaskType, any) error {
		t.Error("application must not be called for invalid payloads")
		return nil
	}}
	for taskType, handler := range handlerFunctions(TaskHandlers{stub, stub, stub}) {
		t.Run(string(taskType), func(t *testing.T) {
			for _, payload := range []string{"", "{", "null", "[]", "{}", `{"tenant_id":"not-a-uuid"}`, `{} {}`} {
				err := handler(context.Background(), asynq.NewTask(string(taskType), []byte(payload)))
				if !errors.Is(err, asynq.SkipRetry) || !errors.Is(err, ports.ErrInvalidTaskPayload) {
					t.Errorf("payload %q must skip retries: %v", payload, err)
				}
			}
			if err := handler(context.Background(), nil); !errors.Is(err, asynq.SkipRetry) {
				t.Fatalf("nil task must skip retries: %v", err)
			}
		})
	}
}

func TestHandlersDelegateTypedPayloadAndTenant(t *testing.T) {
	fixtures := testFixtures(t)
	for taskType, fixture := range fixtures {
		t.Run(string(taskType), func(t *testing.T) {
			var calls int
			stub := applicationStub{call: func(ctx context.Context, actualType ports.TaskType, payload any) error {
				calls++
				if actualType != taskType {
					t.Errorf("wrong application port: %v", actualType)
				}
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 5*time.Minute {
					t.Error("application call requires a bounded context")
				}
				actual, err := json.Marshal(payload)
				if err != nil {
					return err
				}
				var wantMap, gotMap map[string]json.RawMessage
				if err := json.Unmarshal(fixture, &wantMap); err != nil {
					return err
				}
				if err := json.Unmarshal(actual, &gotMap); err != nil {
					return err
				}
				if !bytes.Equal(wantMap["tenant_id"], gotMap["tenant_id"]) {
					t.Error("tenant boundary changed during decoding")
				}
				return nil
			}}
			handler := handlerFunctions(TaskHandlers{stub, stub, stub})[taskType]
			if err := handler(context.Background(), asynq.NewTask(string(taskType), fixture)); err != nil || calls != 1 {
				t.Fatalf("expected one successful application call: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestHandlerErrorClassification(t *testing.T) {
	fixtures := testFixtures(t)
	for _, failure := range []error{errors.New("temporary failure"), ports.ErrInvalidTaskPayload, context.Canceled, context.DeadlineExceeded} {
		stub := applicationStub{call: func(context.Context, ports.TaskType, any) error { return failure }}
		for taskType, handler := range handlerFunctions(TaskHandlers{stub, stub, stub}) {
			err := handler(context.Background(), asynq.NewTask(string(taskType), fixtures[taskType]))
			if !errors.Is(err, failure) || errors.Is(err, asynq.SkipRetry) != errors.Is(failure, ports.ErrInvalidTaskPayload) {
				t.Errorf("incorrect error classification for %s: %v", taskType, err)
			}
		}
	}
	for taskType, handler := range handlerFunctions(TaskHandlers{}) {
		if err := handler(context.Background(), asynq.NewTask(string(taskType), fixtures[taskType])); !errors.Is(err, ErrHandlerUnavailable) {
			t.Errorf("missing service must never acknowledge a job: %v", err)
		}
	}
}

func TestHandlersHonorParentCancellation(t *testing.T) {
	fixtures := testFixtures(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stub := applicationStub{call: func(ctx context.Context, _ ports.TaskType, _ any) error { return ctx.Err() }}
	for taskType, handler := range handlerFunctions(TaskHandlers{stub, stub, stub}) {
		if err := handler(ctx, asynq.NewTask(string(taskType), fixtures[taskType])); !errors.Is(err, context.Canceled) {
			t.Errorf("parent cancellation was lost: %v", err)
		}
	}
}

func TestHandlersConcurrent(t *testing.T) {
	fixtures := testFixtures(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var calls atomic.Int64
	stub := applicationStub{call: func(ctx context.Context, _ ports.TaskType, _ any) error {
		calls.Add(1)
		return ctx.Err()
	}}
	handlers := handlerFunctions(TaskHandlers{stub, stub, stub})
	results := make(chan error, 64)
	for range 64 {
		go func() {
			for taskType, handler := range handlers {
				if err := handler(ctx, asynq.NewTask(string(taskType), fixtures[taskType])); err != nil {
					results <- err
					return
				}
				CalculateRetryDelay(4, errors.New("transient"), nil)
			}
			results <- nil
		}()
	}
	for range 64 {
		select {
		case err := <-results:
			if err != nil {
				t.Error(err)
			}
		case <-ctx.Done():
			t.Fatal("concurrent handlers did not finish within deadline")
		}
	}
	if calls.Load() != 192 {
		t.Fatalf("want 192 independent calls, got %d", calls.Load())
	}
}

func TestDLQLoggingDoesNotLeakPayloadOrError(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	cfg := serverConfig(Config{}, logger)
	task := asynq.NewTask(string(ports.TaskTypeIngestStatement), []byte("synthetic-sensitive-payload"))
	cfg.ErrorHandler.HandleError(context.Background(), task, fmt.Errorf("synthetic-sensitive-error: %w", asynq.SkipRetry))
	if !strings.Contains(output.String(), "[DLQ]") || strings.Contains(output.String(), "synthetic-sensitive") {
		t.Fatalf("DLQ event missing or sensitive input leaked: %s", output.String())
	}
	output.Reset()
	cfg.ErrorHandler.HandleError(context.Background(), task, errors.New("synthetic-sensitive-error"))
	if strings.Contains(output.String(), "[DLQ]") || strings.Contains(output.String(), "synthetic-sensitive") {
		t.Fatalf("transient event misclassified or error leaked: %s", output.String())
	}
}

type lifecycleStub struct {
	start    func(asynq.Handler) error
	stop     func()
	shutdown func()
}

func (s lifecycleStub) Start(handler asynq.Handler) error { return s.start(handler) }
func (s lifecycleStub) Stop()                             { s.stop() }
func (s lifecycleStub) Shutdown()                         { s.shutdown() }

func TestRunPropagatesStartupFailure(t *testing.T) {
	failure := errors.New("startup failure")
	server := &WorkerServer{logger: testLogger(), server: lifecycleStub{
		start: func(asynq.Handler) error { return failure },
	}}
	if err := server.Run(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("startup error was lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled worker must not start: %v", err)
	}
}

func TestGracefulShutdownDrainsInFlightTask(t *testing.T) {
	fixtures := testFixtures(t)
	guard, cancelGuard := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelGuard()
	runCtx, signalStop := context.WithCancel(guard)
	defer signalStop()
	started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	jobDone, runDone := make(chan error, 1), make(chan error, 1)
	var stopCalls atomic.Int64
	stub := applicationStub{call: func(ctx context.Context, _ ports.TaskType, _ any) error {
		close(started)
		select {
		case <-release:
			return ctx.Err()
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	mux := asynq.NewServeMux()
	mux.HandleFunc(string(ports.TaskTypeIngestStatement), TaskHandlers{Ingestion: stub}.HandleIngestStatement)
	server := &WorkerServer{logger: testLogger(), mux: mux, server: lifecycleStub{
		start: func(handler asynq.Handler) error {
			go func() {
				jobDone <- handler.ProcessTask(guard, asynq.NewTask(string(ports.TaskTypeIngestStatement), fixtures[ports.TaskTypeIngestStatement]))
			}()
			return nil
		},
		stop: func() { stopCalls.Add(1); close(stopped) },
		shutdown: func() {
			select {
			case err := <-jobDone:
				if err != nil {
					t.Errorf("in-flight task canceled before draining: %v", err)
				}
			case <-guard.Done():
				t.Error("shutdown did not drain the task")
			}
		},
	}}
	go func() { runDone <- server.Run(runCtx) }()
	select {
	case <-started:
	case <-guard.Done():
		t.Fatal("task did not start")
	}
	signalStop()
	select {
	case <-stopped:
	case <-guard.Done():
		t.Fatal("signal did not stop dequeueing")
	}
	select {
	case <-runDone:
		t.Fatal("worker exited before finishing the task")
	default:
	}
	close(release)
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-guard.Done():
		t.Fatal("worker did not finish draining")
	}
	server.Shutdown()
	if stopCalls.Load() != 1 {
		t.Fatal("shutdown must be idempotent")
	}
}
