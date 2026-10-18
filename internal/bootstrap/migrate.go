package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/migrations"
)

// AutoMigrate verifies if database schema exists, and applies init migrations if needed.
func AutoMigrate(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	logger.InfoContext(ctx, "checking database schema status...")

	var exists bool
	const checkTableSQL = `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'tenants'
		);
	`
	if err := pool.QueryRow(ctx, checkTableSQL).Scan(&exists); err != nil {
		return fmt.Errorf("failed to inspect database tables: %w", err)
	}

	if exists {
		logger.InfoContext(ctx, "database schema is up-to-date (tenants table present)")
		return nil
	}

	logger.InfoContext(ctx, "applying initial schema migration (000001_init_schema.up.sql)...")
	sqlBytes, err := migrations.FS.ReadFile("000001_init_schema.up.sql")
	if err != nil {
		return fmt.Errorf("failed to read embedded migration: %w", err)
	}

	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("failed executing migration 000001_init_schema.up.sql: %w", err)
	}

	logger.InfoContext(ctx, "initial schema migration applied successfully")
	return nil
}
