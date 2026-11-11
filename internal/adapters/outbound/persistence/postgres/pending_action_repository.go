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

var _ ports.PendingActionRepository = (*PendingActionRepository)(nil)

// PendingActionRepository implements ports.PendingActionRepository.
type PendingActionRepository struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

// NewPendingActionRepository constructs a new HITL pending action repository.
func NewPendingActionRepository(pool *pgxpool.Pool) *PendingActionRepository {
	return &PendingActionRepository{pool: pool}
}

// WithTx binds the repository instance to an active transaction.
func (r *PendingActionRepository) WithTx(tx pgx.Tx) *PendingActionRepository {
	return &PendingActionRepository{pool: r.pool, tx: tx}
}

func (r *PendingActionRepository) getExecutor(ctx context.Context) dbExecutor {
	if r.tx != nil {
		return r.tx
	}
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

// Save persists a new pending action.
func (r *PendingActionRepository) Save(ctx context.Context, action *domain.PendingAction) error {
	if action == nil {
		return errors.New("pending action cannot be nil")
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, action.TenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).Save(ctx, action)
			})
		}
	}

	payloadJSON, err := json.Marshal(action.Payload)
	if err != nil {
		return fmt.Errorf("failed marshaling payload: %w", err)
	}

	metadataJSON, err := json.Marshal(action.Metadata)
	if err != nil {
		return fmt.Errorf("failed marshaling metadata: %w", err)
	}

	const insertSQL = `
		INSERT INTO pending_actions (
			id, tenant_id, created_by_agent, action_type, payload,
			status, rejection_reason, decided_by_user_id, decided_at,
			executed_at, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);
	`

	_, err = r.getExecutor(ctx).Exec(ctx, insertSQL,
		action.ID, action.TenantID, action.CreatedByAgent, action.ActionType,
		payloadJSON, action.Status, action.RejectionReason, action.DecidedByUserID,
		action.DecidedAt, action.ExecutedAt, metadataJSON, action.CreatedAt, action.UpdatedAt,
	)
	return err
}

// FindByID retrieves a pending action by its ID within tenant boundaries.
func (r *PendingActionRepository) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.PendingAction, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var a *domain.PendingAction
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var txErr error
				a, txErr = r.WithTx(tx).FindByID(ctx, tenantID, id)
				return txErr
			})
			return a, err
		}
	}

	const querySQL = `
		SELECT id, tenant_id, created_by_agent, action_type, payload,
		       status, rejection_reason, decided_by_user_id, decided_at,
		       executed_at, metadata, created_at, updated_at
		FROM pending_actions
		WHERE tenant_id = $1 AND id = $2;
	`
	row := r.getExecutor(ctx).QueryRow(ctx, querySQL, tenantID, id)
	return scanPendingAction(row)
}

// ListPending retrieves all pending actions awaiting review in the tenant.
func (r *PendingActionRepository) ListPending(ctx context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var list []*domain.PendingAction
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var txErr error
				list, txErr = r.WithTx(tx).ListPending(ctx, tenantID)
				return txErr
			})
			return list, err
		}
	}

	const querySQL = `
		SELECT id, tenant_id, created_by_agent, action_type, payload,
		       status, rejection_reason, decided_by_user_id, decided_at,
		       executed_at, metadata, created_at, updated_at
		FROM pending_actions
		WHERE tenant_id = $1 AND status = 'PENDING'
		ORDER BY created_at ASC;
	`
	rows, err := r.getExecutor(ctx).Query(ctx, querySQL, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.PendingAction
	for rows.Next() {
		a, scanErr := scanPendingAction(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, a)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return result, nil
}

// UpdateDecision records human approval or rejection on a pending action.
func (r *PendingActionRepository) UpdateDecision(
	ctx context.Context,
	tenantID, id uuid.UUID,
	status domain.PendingActionStatus,
	decidedBy *uuid.UUID,
	reason *string,
) error {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).UpdateDecision(ctx, tenantID, id, status, decidedBy, reason)
			})
		}
	}

	const updateSQL = `
		UPDATE pending_actions
		SET status = $1, decided_by_user_id = $2, rejection_reason = $3, decided_at = NOW(), updated_at = NOW()
		WHERE tenant_id = $4 AND id = $5 AND status = 'PENDING';
	`

	cmdTag, err := r.getExecutor(ctx).Exec(ctx, updateSQL, status, decidedBy, reason, tenantID, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkExecuted marks an approved pending action as executed.
func (r *PendingActionRepository) MarkExecuted(ctx context.Context, tenantID, id uuid.UUID) error {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).MarkExecuted(ctx, tenantID, id)
			})
		}
	}

	const updateSQL = `
		UPDATE pending_actions
		SET status = 'EXECUTED', executed_at = NOW(), updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND status = 'APPROVED';
	`

	cmdTag, err := r.getExecutor(ctx).Exec(ctx, updateSQL, tenantID, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanPendingAction(scanner rowScanner) (*domain.PendingAction, error) {
	var (
		a            domain.PendingAction
		payloadJSON  []byte
		metadataJSON []byte
	)

	err := scanner.Scan(
		&a.ID, &a.TenantID, &a.CreatedByAgent, &a.ActionType, &payloadJSON,
		&a.Status, &a.RejectionReason, &a.DecidedByUserID, &a.DecidedAt,
		&a.ExecutedAt, &metadataJSON, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &a.Payload); err != nil {
			a.Payload = make(map[string]any)
		}
	} else {
		a.Payload = make(map[string]any)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &a.Metadata); err != nil {
			a.Metadata = make(map[string]any)
		}
	} else {
		a.Metadata = make(map[string]any)
	}

	return &a, nil
}
