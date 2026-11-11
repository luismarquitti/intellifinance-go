package bootstrap_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/bootstrap"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/stretchr/testify/require"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("TEST_DATABASE_URL")
	}
	require.NotEmpty(t, dbURL, "DATABASE_URL is required for migration tests")

	require.NoError(t, config.ValidateDisposableDatabase(dbURL, os.Getenv("ENVIRONMENT")))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	return pool
}

func TestAutoMigrate_VersionTrackingAndIdempotency(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// 1. Run AutoMigrate (should apply all pending migrations 000001, 000002, 000003)
	err := bootstrap.AutoMigrate(ctx, pool, logger)
	require.NoError(t, err)

	// 2. Run AutoMigrate again (must be completely idempotent and no-op)
	err = bootstrap.AutoMigrate(ctx, pool, logger)
	require.NoError(t, err)

	// 3. Verify schema_migrations versions
	rows, err := pool.Query(ctx, "SELECT version, dirty FROM schema_migrations ORDER BY version ASC")
	require.NoError(t, err)
	defer rows.Close()

	var versions []int64
	for rows.Next() {
		var v int64
		var dirty bool
		require.NoError(t, rows.Scan(&v, &dirty))
		require.False(t, dirty, "migration must not be left in dirty state")
		versions = append(versions, v)
	}
	require.NoError(t, rows.Err())
	require.Contains(t, versions, int64(1))
	require.Contains(t, versions, int64(2))
	require.Contains(t, versions, int64(3))

	// 4. Verify that new tables exist
	expectedTables := []string{
		"provider_balance_snapshots",
		"credit_card_invoices",
		"pending_actions",
	}
	for _, table := range expectedTables {
		var exists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, table).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "table %s should exist after migrations", table)
	}

	// 5. Test CurrentVersion and GetStatus
	ver, dirty, err := bootstrap.CurrentVersion(ctx, pool)
	require.NoError(t, err)
	require.False(t, dirty)
	require.GreaterOrEqual(t, ver, int64(3))

	statuses, err := bootstrap.GetStatus(ctx, pool)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(statuses), 3)
	for _, s := range statuses {
		require.True(t, s.Applied)
		require.False(t, s.Dirty)
		require.NotEmpty(t, s.Filename)
	}
}
