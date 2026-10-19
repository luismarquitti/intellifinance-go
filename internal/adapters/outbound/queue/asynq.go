// Package queue implements the asynchronous task submission port using Redis.
package queue

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/hibiken/asynq"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

const (
	// QueueCritical carries Open Finance sync tasks.
	QueueCritical = "critical"
	// QueueDefault carries statement ingestion tasks.
	QueueDefault = "default"
	// QueueLow carries categorization tasks.
	QueueLow = "low"
	// MaxRetries bounds transient failures before archiving a task in the DLQ.
	MaxRetries = 5
)

type enqueueClient interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
	Close() error
}

// AsynqQueueAdapter routes application tasks to the worker's weighted queues.
type AsynqQueueAdapter struct {
	client enqueueClient
}

var _ ports.TaskQueue = (*AsynqQueueAdapter)(nil)

// NewAsynqQueueAdapter constructs a queue client without performing network I/O.
func NewAsynqQueueAdapter(redis asynq.RedisConnOpt) *AsynqQueueAdapter {
	return &AsynqQueueAdapter{client: asynq.NewClient(redis)}
}

// Enqueue copies the task before applying options, so callers can reuse it safely.
func (a *AsynqQueueAdapter) Enqueue(ctx context.Context, task *ports.Task, opts ...ports.QueueOption) error {
	if task == nil {
		return fmt.Errorf("enqueue: task is required")
	}
	copyTask := *task
	copyTask.Payload = append([]byte(nil), task.Payload...)
	copyTask.Headers = maps.Clone(task.Headers)
	for _, opt := range opts {
		if opt == nil {
			return fmt.Errorf("enqueue: nil queue option")
		}
		opt(&copyTask)
	}
	queueName, err := ForTask(copyTask.Type)
	if err != nil {
		return err
	}
	if copyTask.MaxRetries < 0 || copyTask.MaxRetries > MaxRetries || copyTask.Delay < 0 || copyTask.Timeout < 0 {
		return fmt.Errorf("enqueue: invalid retries, delay or timeout")
	}
	retries := copyTask.MaxRetries
	if retries == 0 {
		retries = MaxRetries
	}
	timeout := copyTask.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	options := []asynq.Option{asynq.Queue(queueName), asynq.MaxRetry(retries), asynq.Timeout(timeout)}
	if copyTask.ID != "" {
		options = append(options, asynq.TaskID(copyTask.ID))
	}
	if copyTask.Delay > 0 {
		options = append(options, asynq.ProcessIn(copyTask.Delay))
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := a.client.EnqueueContext(ctx, asynq.NewTask(string(copyTask.Type), copyTask.Payload), options...); err != nil {
		return fmt.Errorf("asynq enqueue %s: %w", copyTask.Type, err)
	}
	return nil
}

// Close releases the queue client's connections.
func (a *AsynqQueueAdapter) Close() error {
	if err := a.client.Close(); err != nil {
		return fmt.Errorf("close asynq client: %w", err)
	}
	return nil
}

// ForTask returns the canonical routing for a supported task type.
func ForTask(taskType ports.TaskType) (string, error) {
	switch taskType {
	case ports.TaskTypeSyncOpenFinance:
		return QueueCritical, nil
	case ports.TaskTypeIngestStatement:
		return QueueDefault, nil
	case ports.TaskTypeCategorizeTransaction:
		return QueueLow, nil
	default:
		return "", fmt.Errorf("unsupported task type")
	}
}
