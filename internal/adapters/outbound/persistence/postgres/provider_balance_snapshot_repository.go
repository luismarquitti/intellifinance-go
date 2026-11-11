package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.ProviderBalanceSnapshotRepository = (*ProviderBalanceSnapshotRepository)(nil)

// ProviderBalanceSnapshotRepository implements ports.ProviderBalanceSnapshotRepository.
type ProviderBalanceSnapshotRepository struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

// NewProviderBalanceSnapshotRepository constructs a new repository for balance snapshots.
func NewProviderBalanceSnapshotRepository(pool *pgxpool.Pool) *ProviderBalanceSnapshotRepository {
	return &ProviderBalanceSnapshotRepository{pool: pool}
}

// WithTx binds the repository instance to an active transaction.
func (r *ProviderBalanceSnapshotRepository) WithTx(tx pgx.Tx) *ProviderBalanceSnapshotRepository {
	return &ProviderBalanceSnapshotRepository{pool: r.pool, tx: tx}
}

func (r *ProviderBalanceSnapshotRepository) getExecutor(ctx context.Context) dbExecutor {
	if r.tx != nil {
		return r.tx
	}
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

// Save persists an external provider balance snapshot.
func (r *ProviderBalanceSnapshotRepository) Save(ctx context.Context, s *domain.ProviderBalanceSnapshot) error {
	if s == nil {
		return errors.New("snapshot cannot be nil")
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, s.TenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).Save(ctx, s)
			})
		}
	}

	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return fmt.Errorf("failed marshaling snapshot metadata: %w", err)
	}

	const insertSQL = `
		INSERT INTO provider_balance_snapshots (
			id, tenant_id, account_id, provider, balance,
			currency, observed_at, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
	`

	_, err = r.getExecutor(ctx).Exec(ctx, insertSQL,
		s.ID, s.TenantID, s.AccountID, s.Provider,
		s.Balance, s.Currency, s.ObservedAt, metadataJSON, s.CreatedAt,
	)
	return err
}

// GetLatestByAccount retrieves the most recent balance snapshot for an account.
func (r *ProviderBalanceSnapshotRepository) GetLatestByAccount(ctx context.Context, tenantID, accountID uuid.UUID) (*domain.ProviderBalanceSnapshot, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var snap *domain.ProviderBalanceSnapshot
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var txErr error
				snap, txErr = r.WithTx(tx).GetLatestByAccount(ctx, tenantID, accountID)
				return txErr
			})
			return snap, err
		}
	}

	const querySQL = `
		SELECT id, tenant_id, account_id, provider, balance,
		       currency, observed_at, metadata, created_at
		FROM provider_balance_snapshots
		WHERE tenant_id = $1 AND account_id = $2
		ORDER BY observed_at DESC
		LIMIT 1;
	`
	row := r.getExecutor(ctx).QueryRow(ctx, querySQL, tenantID, accountID)
	return scanSnapshot(row)
}

// ListByAccount lists recent balance snapshots for an account up to limit.
func (r *ProviderBalanceSnapshotRepository) ListByAccount(ctx context.Context, tenantID, accountID uuid.UUID, limit int) ([]*domain.ProviderBalanceSnapshot, error) {
	if limit <= 0 {
		limit = 50
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var list []*domain.ProviderBalanceSnapshot
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var txErr error
				list, txErr = r.WithTx(tx).ListByAccount(ctx, tenantID, accountID, limit)
				return txErr
			})
			return list, err
		}
	}

	const querySQL = `
		SELECT id, tenant_id, account_id, provider, balance,
		       currency, observed_at, metadata, created_at
		FROM provider_balance_snapshots
		WHERE tenant_id = $1 AND account_id = $2
		ORDER BY observed_at DESC
		LIMIT $3;
	`
	rows, err := r.getExecutor(ctx).Query(ctx, querySQL, tenantID, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.ProviderBalanceSnapshot
	for rows.Next() {
		s, scanErr := scanSnapshot(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, s)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return result, nil
}

func scanSnapshot(scanner rowScanner) (*domain.ProviderBalanceSnapshot, error) {
	var (
		s            domain.ProviderBalanceSnapshot
		metadataJSON []byte
	)

	err := scanner.Scan(
		&s.ID, &s.TenantID, &s.AccountID, &s.Provider,
		&s.Balance, &s.Currency, &s.ObservedAt, &metadataJSON, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &s.Metadata); err != nil {
			s.Metadata = make(map[string]any)
		}
	} else {
		s.Metadata = make(map[string]any)
	}

	return &s, nil
}
