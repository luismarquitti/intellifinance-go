package ports

import (
	"context"
	"time"
)

// TaskType identifies the destination background task handler.
type TaskType string

const (
	// TaskTypeIngestStatement handles asynchronous parsing and deduplication of statement files.
	TaskTypeIngestStatement TaskType = "task:ingest_statement"
	// TaskTypeCategorizeTransaction handles asynchronous AI category inference.
	TaskTypeCategorizeTransaction TaskType = "task:categorize_transaction"
	// TaskTypeSyncOpenFinance handles Open Finance incremental or full banking sync.
	TaskTypeSyncOpenFinance TaskType = "task:sync_open_finance"
)

// Task represents a serializable job dispatched to an asynchronous queue driver.
type Task struct {
	ID         string            `json:"id"`
	Type       TaskType          `json:"type"`
	Payload    []byte            `json:"payload"`
	Headers    map[string]string `json:"headers,omitempty"`
	Delay      time.Duration     `json:"delay,omitempty"`
	MaxRetries int               `json:"max_retries,omitempty"`
	Timeout    time.Duration     `json:"timeout,omitempty"`
}

// QueueOption configures task dispatch properties.
type QueueOption func(*Task)

// WithDelay schedules the task to execute after a specified delay duration.
func WithDelay(d time.Duration) QueueOption {
	return func(t *Task) {
		t.Delay = d
	}
}

// WithMaxRetries sets the maximum retry attempts for the task upon transient errors.
func WithMaxRetries(retries int) QueueOption {
	return func(t *Task) {
		t.MaxRetries = retries
	}
}

// WithTimeout sets the execution deadline for processing the task.
func WithTimeout(timeout time.Duration) QueueOption {
	return func(t *Task) {
		t.Timeout = timeout
	}
}

// TaskQueue is the outbound port contract for background task submission.
type TaskQueue interface {
	// Enqueue submits a background task to the underlying queue engine (Asynq or Cloud Tasks).
	Enqueue(ctx context.Context, task *Task, opts ...QueueOption) error

	// Close terminates connections and releases resources held by the queue client.
	Close() error
}
