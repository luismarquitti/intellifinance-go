package ports_test

import (
	"testing"
	"time"

	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

func TestQueueOptions(t *testing.T) {
	task := &ports.Task{
		ID:   "task-123",
		Type: ports.TaskTypeIngestStatement,
	}

	delay := 10 * time.Second
	retries := 3
	timeout := 45 * time.Second

	ports.WithDelay(delay)(task)
	ports.WithMaxRetries(retries)(task)
	ports.WithTimeout(timeout)(task)

	if task.Delay != delay {
		t.Errorf("expected delay %v, got %v", delay, task.Delay)
	}
	if task.MaxRetries != retries {
		t.Errorf("expected max retries %d, got %d", retries, task.MaxRetries)
	}
	if task.Timeout != timeout {
		t.Errorf("expected timeout %v, got %v", timeout, task.Timeout)
	}
}
