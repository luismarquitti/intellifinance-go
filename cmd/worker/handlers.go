package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// ErrHandlerUnavailable prevents acknowledging work whose service is not wired.
var ErrHandlerUnavailable = errors.New("application task handler is not configured")

// TaskHandlers translates queue messages into typed application calls.
// Injected services must support concurrent calls and honor context deadlines.
type TaskHandlers struct {
	Ingestion      ports.IngestStatementHandler
	OpenFinance    ports.SyncOpenFinanceHandler
	Categorization ports.CategorizeTransactionHandler
}

// HandleIngestStatement validates a statement task and delegates its execution.
func (h TaskHandlers) HandleIngestStatement(ctx context.Context, task *asynq.Task) error {
	var payload ports.IngestStatementPayload
	if err := decodePayload(task, &payload); err != nil {
		return err
	}
	if h.Ingestion == nil {
		return ErrHandlerUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	return applicationError("ingest statement", h.Ingestion.IngestStatement(ctx, payload))
}

// HandleSyncOpenFinance validates a banking sync task and delegates execution.
func (h TaskHandlers) HandleSyncOpenFinance(ctx context.Context, task *asynq.Task) error {
	var payload ports.SyncOpenFinancePayload
	if err := decodePayload(task, &payload); err != nil {
		return err
	}
	if h.OpenFinance == nil {
		return ErrHandlerUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	return applicationError("sync open finance", h.OpenFinance.SyncOpenFinance(ctx, payload))
}

// HandleCategorizeTransaction validates a batch task and delegates execution.
func (h TaskHandlers) HandleCategorizeTransaction(ctx context.Context, task *asynq.Task) error {
	var payload ports.CategorizeTransactionPayload
	if err := decodePayload(task, &payload); err != nil {
		return err
	}
	if h.Categorization == nil {
		return ErrHandlerUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	return applicationError("categorize transaction", h.Categorization.CategorizeTransaction(ctx, payload))
}

type taskPayload interface {
	Validate() error
}

func decodePayload(task *asynq.Task, payload taskPayload) error {
	if task == nil {
		return fmt.Errorf("%w: %w: missing task", asynq.SkipRetry, ports.ErrInvalidTaskPayload)
	}
	if err := json.Unmarshal(task.Payload(), payload); err != nil {
		// Do not propagate decoder messages that may contain sensitive input.
		return fmt.Errorf("%w: %w: malformed JSON", asynq.SkipRetry, ports.ErrInvalidTaskPayload)
	}
	if err := payload.Validate(); err != nil {
		return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
	}
	return nil
}

func applicationError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ports.ErrInvalidTaskPayload) {
		return fmt.Errorf("%s: %w: %w", operation, asynq.SkipRetry, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
