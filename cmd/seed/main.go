package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/bootstrap"
	"github.com/luismarquitti/intellifinance-go/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()

	ctx := context.Background()
	logger.Info("connecting to database for seeding...", "url", cfg.DatabaseURL)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("database ping failed", "err", err)
		os.Exit(1)
	}

	if err := bootstrap.AutoMigrate(ctx, pool, logger); err != nil {
		logger.Error("auto-migrate failed", "err", err)
		os.Exit(1)
	}

	if err := bootstrap.SeedDevAdmin(ctx, pool, cfg, logger); err != nil {
		logger.Error("dev admin seeding failed", "err", err)
		os.Exit(1)
	}

	fmt.Println("Seed completed successfully!")
}
