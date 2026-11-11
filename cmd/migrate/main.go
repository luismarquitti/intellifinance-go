package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/bootstrap"
	"github.com/luismarquitti/intellifinance-go/internal/config"
)

func main() {
	var (
		statusFlag  = flag.Bool("status", false, "Display current migration status and history")
		upFlag      = flag.Bool("up", false, "Apply all pending embedded database migrations")
		versionFlag = flag.Bool("version", false, "Print current applied database schema version")
	)
	flag.Parse()

	if !*statusFlag && !*upFlag && !*versionFlag {
		fmt.Println("IntelliFinance Migration Engine")
		fmt.Println("Usage:")
		fmt.Println("  go run ./cmd/migrate -status   # Check applied and pending migrations")
		fmt.Println("  go run ./cmd/migrate -up       # Apply all pending migrations")
		fmt.Println("  go run ./cmd/migrate -version  # Show current database version")
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL environment variable is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

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

	if *versionFlag {
		ver, dirty, err := bootstrap.CurrentVersion(ctx, pool)
		if err != nil {
			logger.Error("failed checking database version", "err", err)
			os.Exit(1)
		}
		if dirty {
			fmt.Printf("Current schema version: %d (DIRTY)\n", ver)
		} else {
			fmt.Printf("Current schema version: %d (CLEAN)\n", ver)
		}
		return
	}

	if *statusFlag {
		statuses, err := bootstrap.GetStatus(ctx, pool)
		if err != nil {
			logger.Error("failed checking migration status", "err", err)
			os.Exit(1)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "VERSION\tSTATUS\tFILENAME\tAPPLIED AT")
		for _, s := range statuses {
			st := "PENDING"
			if s.Applied {
				if s.Dirty {
					st = "DIRTY"
				} else {
					st = "APPLIED"
				}
			}
			appliedAtStr := "-"
			if s.Applied && !s.AppliedAt.IsZero() {
				appliedAtStr = s.AppliedAt.Format("2006-01-02 15:04:05 UTC")
			}
			_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", s.Version, st, s.Filename, appliedAtStr)
		}
		_ = w.Flush()
		return
	}

	if *upFlag {
		if err := bootstrap.AutoMigrate(ctx, pool, logger); err != nil {
			logger.Error("failed applying migrations", "err", err)
			os.Exit(1)
		}
		fmt.Println("All pending migrations applied successfully.")
	}
}
