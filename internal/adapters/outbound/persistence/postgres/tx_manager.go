package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres/db"
)

// TxManager manages PostgreSQL transaction lifecycles and tenant-isolated RLS contexts.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager constructs a new TxManager with the given pgx connection pool.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// Pool returns the underlying pgxpool.Pool.
func (m *TxManager) Pool() *pgxpool.Pool {
	return m.pool
}

// WithTenantTx executes the provided callback within an isolated PostgreSQL transaction
// where session variables app.current_tenant_id and app.current_user_id are strictly localized
// using set_config(..., true). Session settings automatically expire when the transaction commits or aborts.
func (m *TxManager) WithTenantTx(ctx context.Context, tenantID, userID uuid.UUID, fn func(tx pgx.Tx) error) error {
	return WithTenantTx(ctx, m.pool, tenantID, userID, fn)
}

// WithTenantQueries is a convenience helper that executes fn with a type-safe *db.Queries
// scoped to the tenant transaction.
func (m *TxManager) WithTenantQueries(ctx context.Context, tenantID, userID uuid.UUID, fn func(q *db.Queries) error) error {
	return m.WithTenantTx(ctx, tenantID, userID, func(tx pgx.Tx) error {
		return fn(db.New(tx))
	})
}

// WithTenantTx executes fn within an isolated transaction on pool with localized RLS context.
func WithTenantTx(
	ctx context.Context,
	pool *pgxpool.Pool,
	tenantID uuid.UUID,
	userID uuid.UUID,
	fn func(tx pgx.Tx) error,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var tenantVal string
	if tenantID != uuid.Nil {
		tenantVal = tenantID.String()
	}
	var userVal string
	if userID != uuid.Nil {
		userVal = userID.String()
	}

	const setSessionSQL = `
		SELECT set_config('app.current_tenant_id', $1, true),
		       set_config('app.current_user_id', $2, true)
	`
	if _, err := tx.Exec(ctx, setSessionSQL, tenantVal, userVal); err != nil {
		return fmt.Errorf("failed to set tenant RLS context: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

type txContextKey struct{}

// ContextWithTx injects an active pgx.Tx into context.Context.
func ContextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext extracts an active pgx.Tx from context.Context if present.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx, ok
}
