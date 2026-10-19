package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// OpenFinanceTenantResolver resolves tenant IDs by querying the PostgreSQL SECURITY DEFINER
// function resolve_open_finance_tenant(p_item_id VARCHAR).
type OpenFinanceTenantResolver struct {
	pool *pgxpool.Pool
}

// NewOpenFinanceTenantResolver constructs a new OpenFinanceTenantResolver.
func NewOpenFinanceTenantResolver(pool *pgxpool.Pool) ports.OpenFinanceTenantResolver {
	return &OpenFinanceTenantResolver{pool: pool}
}

// ResolveTenantByItemID resolves the tenant ID associated with an Open Finance item ID.
func (r *OpenFinanceTenantResolver) ResolveTenantByItemID(ctx context.Context, itemID string) (uuid.UUID, error) {
	cleanItemID := strings.TrimSpace(itemID)
	if cleanItemID == "" {
		return uuid.Nil, fmt.Errorf("item ID cannot be empty")
	}

	callCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		callCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	var nullTenant uuid.NullUUID
	err := r.pool.QueryRow(callCtx, "SELECT resolve_open_finance_tenant($1)", cleanItemID).Scan(&nullTenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("tenant not found for item %s", cleanItemID)
		}
		return uuid.Nil, fmt.Errorf("failed to query resolve_open_finance_tenant for item %s: %w", cleanItemID, err)
	}

	if !nullTenant.Valid || nullTenant.UUID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("tenant not found for item %s", cleanItemID)
	}

	return nullTenant.UUID, nil
}
