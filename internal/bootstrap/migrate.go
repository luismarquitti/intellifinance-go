package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/migrations"
)

const migrationAdvisoryLockID = 847291048291

type migrationFile struct {
	version  int64
	filename string
}

// MigrationStatus reports the status of a specific migration.
type MigrationStatus struct {
	Version   int64     `json:"version"`
	Filename  string    `json:"filename"`
	Applied   bool      `json:"applied"`
	Dirty     bool      `json:"dirty"`
	AppliedAt time.Time `json:"applied_at,omitempty"`
}

func ensureSchemaMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	const createSchemaMigrationsSQL = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			dirty BOOLEAN NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	if _, err := pool.Exec(ctx, createSchemaMigrationsSQL); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	const legacyReconciliationSQL = `
		INSERT INTO schema_migrations (version, dirty, applied_at)
		SELECT 1, false, NOW()
		WHERE EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'tenants'
		) AND NOT EXISTS (
			SELECT 1 FROM schema_migrations WHERE version = 1
		);
	`
	if _, err := pool.Exec(ctx, legacyReconciliationSQL); err != nil {
		return fmt.Errorf("failed to reconcile legacy migration state: %w", err)
	}
	return nil
}

// GetStatus returns the migration status for all embedded migration files.
func GetStatus(ctx context.Context, pool *pgxpool.Pool) ([]MigrationStatus, error) {
	if err := ensureSchemaMigrationsTable(ctx, pool); err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, "SELECT version, dirty, applied_at FROM schema_migrations ORDER BY version ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to query schema_migrations: %w", err)
	}
	defer rows.Close()

	type record struct {
		dirty     bool
		appliedAt time.Time
	}
	appliedMap := make(map[int64]record)
	for rows.Next() {
		var v int64
		var d bool
		var at time.Time
		if scanErr := rows.Scan(&v, &d, &at); scanErr != nil {
			return nil, fmt.Errorf("failed to scan migration row: %w", scanErr)
		}
		appliedMap[v] = record{dirty: d, appliedAt: at}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("error reading schema_migrations: %w", rowsErr)
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed reading embedded migrations dir: %w", err)
	}

	var statuses []MigrationStatus
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		ver, parseErr := strconv.ParseInt(parts[0], 10, 64)
		if parseErr != nil {
			continue
		}

		rec, ok := appliedMap[ver]
		st := MigrationStatus{
			Version:  ver,
			Filename: name,
			Applied:  ok,
			Dirty:    ok && rec.dirty,
		}
		if ok {
			st.AppliedAt = rec.appliedAt
		}
		statuses = append(statuses, st)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Version < statuses[j].Version
	})

	return statuses, nil
}

// CurrentVersion returns the highest clean version applied to the database.
func CurrentVersion(ctx context.Context, pool *pgxpool.Pool) (int64, bool, error) {
	if err := ensureSchemaMigrationsTable(ctx, pool); err != nil {
		return 0, false, err
	}

	var version int64
	var dirty bool
	err := pool.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version, &dirty)
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get current version: %w", err)
	}
	return version, dirty, nil
}

// AutoMigrate applies all pending embedded migrations sequentially and idempotently,
// tracking applied versions in schema_migrations with an advisory lock.
func AutoMigrate(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	logger.InfoContext(ctx, "checking database schema migration status...")

	// 1. Acquire session-level advisory lock to serialize concurrent migrations
	if _, err := pool.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockID); err != nil {
		return fmt.Errorf("failed to acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID)
	}()

	// 2. Ensure schema_migrations control table exists
	if err := ensureSchemaMigrationsTable(ctx, pool); err != nil {
		return err
	}

	// 3. Query already applied migrations
	rows, err := pool.Query(ctx, "SELECT version, dirty FROM schema_migrations ORDER BY version ASC")
	if err != nil {
		return fmt.Errorf("failed to query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var v int64
		var dirty bool
		if scanErr := rows.Scan(&v, &dirty); scanErr != nil {
			return fmt.Errorf("failed to scan migration row: %w", scanErr)
		}
		if dirty {
			return fmt.Errorf("database is in a dirty migration state at version %d; manual intervention required", v)
		}
		applied[v] = true
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return fmt.Errorf("error reading schema_migrations: %w", rowsErr)
	}

	// 4. Discover all .up.sql files in embedded filesystem
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed reading embedded migrations dir: %w", err)
	}

	var pending []migrationFile
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		ver, parseErr := strconv.ParseInt(parts[0], 10, 64)
		if parseErr != nil {
			continue
		}
		if !applied[ver] {
			pending = append(pending, migrationFile{
				version:  ver,
				filename: name,
			})
		}
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].version < pending[j].version
	})

	if len(pending) == 0 {
		logger.InfoContext(ctx, "database schema is up-to-date (no pending migrations)")
		return nil
	}

	// 5. Apply pending migrations sequentially inside transactions
	for _, m := range pending {
		logger.InfoContext(ctx, "applying migration...", "version", m.version, "filename", m.filename)

		sqlBytes, readErr := migrations.FS.ReadFile(m.filename)
		if readErr != nil {
			return fmt.Errorf("failed reading migration file %s: %w", m.filename, readErr)
		}

		if txErr := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			// Mark version as dirty
			const markDirtySQL = `
				INSERT INTO schema_migrations (version, dirty, applied_at)
				VALUES ($1, true, NOW())
				ON CONFLICT (version) DO UPDATE SET dirty = true, applied_at = NOW();
			`
			if _, e := tx.Exec(ctx, markDirtySQL, m.version); e != nil {
				return fmt.Errorf("failed marking migration dirty: %w", e)
			}

			// Execute migration DDL
			if _, e := tx.Exec(ctx, string(sqlBytes)); e != nil {
				return fmt.Errorf("failed executing migration %s: %w", m.filename, e)
			}

			// Mark version as clean
			const markCleanSQL = `
				UPDATE schema_migrations SET dirty = false, applied_at = NOW() WHERE version = $1;
			`
			if _, e := tx.Exec(ctx, markCleanSQL, m.version); e != nil {
				return fmt.Errorf("failed marking migration clean: %w", e)
			}
			return nil
		}); txErr != nil {
			return fmt.Errorf("failed applying migration %s (version %d): %w", m.filename, m.version, txErr)
		}

		logger.InfoContext(ctx, "migration applied successfully", "version", m.version, "filename", m.filename)
	}

	logger.InfoContext(ctx, "all pending migrations applied successfully")
	return nil
}
