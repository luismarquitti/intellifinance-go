package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("worker failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	// Application pipelines are delivered separately. Until they are wired,
	// valid jobs fail explicitly and remain eligible for retry/DLQ inspection.
	logger.Warn("application task services are not configured; valid tasks will fail and retry")
	server, err := NewWorkerServer(cfg, TaskHandlers{}, logger)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return server.Run(ctx)
}
