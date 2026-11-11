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

var _ ports.InvoiceRepository = (*InvoiceRepository)(nil)

// InvoiceRepository implements ports.InvoiceRepository using PostgreSQL.
type InvoiceRepository struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

// NewInvoiceRepository constructs a new invoice repository.
func NewInvoiceRepository(pool *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{pool: pool}
}

// WithTx binds the repository instance to an active database transaction.
func (r *InvoiceRepository) WithTx(tx pgx.Tx) *InvoiceRepository {
	return &InvoiceRepository{pool: r.pool, tx: tx}
}

func (r *InvoiceRepository) getExecutor(ctx context.Context) dbExecutor {
	if r.tx != nil {
		return r.tx
	}
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

// Save persists or updates a credit card invoice within tenant isolation.
func (r *InvoiceRepository) Save(ctx context.Context, inv *domain.CreditCardInvoice) error {
	if inv == nil {
		return errors.New("invoice cannot be nil")
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, inv.TenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).Save(ctx, inv)
			})
		}
	}

	metadataJSON, err := json.Marshal(inv.Metadata)
	if err != nil {
		return fmt.Errorf("failed marshaling invoice metadata: %w", err)
	}

	const upsertSQL = `
		INSERT INTO credit_card_invoices (
			id, tenant_id, account_id, external_id, due_date, closing_date,
			total_amount, minimum_payment, currency, status, allows_installments,
			metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		ON CONFLICT (tenant_id, account_id, due_date) DO UPDATE SET
			external_id = COALESCE(EXCLUDED.external_id, credit_card_invoices.external_id),
			closing_date = EXCLUDED.closing_date,
			total_amount = EXCLUDED.total_amount,
			minimum_payment = EXCLUDED.minimum_payment,
			currency = EXCLUDED.currency,
			status = EXCLUDED.status,
			allows_installments = EXCLUDED.allows_installments,
			metadata = EXCLUDED.metadata,
			updated_at = NOW();
	`

	_, err = r.getExecutor(ctx).Exec(ctx, upsertSQL,
		inv.ID, inv.TenantID, inv.AccountID, inv.ExternalID, inv.DueDate,
		inv.ClosingDate, inv.TotalAmount, inv.MinimumPayment, inv.Currency,
		inv.Status, inv.AllowsInstallments, metadataJSON, inv.CreatedAt, inv.UpdatedAt,
	)
	return err
}

// FindByID retrieves an invoice by its unique ID within tenant boundaries.
func (r *InvoiceRepository) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.CreditCardInvoice, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var inv *domain.CreditCardInvoice
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var txErr error
				inv, txErr = r.WithTx(tx).FindByID(ctx, tenantID, id)
				return txErr
			})
			return inv, err
		}
	}

	const querySQL = `
		SELECT id, tenant_id, account_id, external_id, due_date, closing_date,
		       total_amount, minimum_payment, currency, status, allows_installments,
		       metadata, created_at, updated_at
		FROM credit_card_invoices
		WHERE tenant_id = $1 AND id = $2;
	`
	row := r.getExecutor(ctx).QueryRow(ctx, querySQL, tenantID, id)
	return scanInvoice(row)
}

// FindByExternalID retrieves an invoice by provider external ID within tenant boundaries.
func (r *InvoiceRepository) FindByExternalID(ctx context.Context, tenantID uuid.UUID, externalID string) (*domain.CreditCardInvoice, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var inv *domain.CreditCardInvoice
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var txErr error
				inv, txErr = r.WithTx(tx).FindByExternalID(ctx, tenantID, externalID)
				return txErr
			})
			return inv, err
		}
	}

	const querySQL = `
		SELECT id, tenant_id, account_id, external_id, due_date, closing_date,
		       total_amount, minimum_payment, currency, status, allows_installments,
		       metadata, created_at, updated_at
		FROM credit_card_invoices
		WHERE tenant_id = $1 AND external_id = $2;
	`
	row := r.getExecutor(ctx).QueryRow(ctx, querySQL, tenantID, externalID)
	return scanInvoice(row)
}

// List returns invoices for a tenant, optionally filtered by accountID, sorted by due date descending.
func (r *InvoiceRepository) List(ctx context.Context, tenantID uuid.UUID, accountID *uuid.UUID) ([]*domain.CreditCardInvoice, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var invs []*domain.CreditCardInvoice
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var txErr error
				invs, txErr = r.WithTx(tx).List(ctx, tenantID, accountID)
				return txErr
			})
			return invs, err
		}
	}

	const querySQL = `
		SELECT id, tenant_id, account_id, external_id, due_date, closing_date,
		       total_amount, minimum_payment, currency, status, allows_installments,
		       metadata, created_at, updated_at
		FROM credit_card_invoices
		WHERE tenant_id = $1 AND ($2::uuid IS NULL OR account_id = $2::uuid)
		ORDER BY due_date DESC;
	`
	rows, err := r.getExecutor(ctx).Query(ctx, querySQL, tenantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.CreditCardInvoice
	for rows.Next() {
		inv, scanErr := scanInvoice(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, inv)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return result, nil
}

// ListByAccount returns all invoices for an account sorted by due date descending.
func (r *InvoiceRepository) ListByAccount(ctx context.Context, tenantID, accountID uuid.UUID) ([]*domain.CreditCardInvoice, error) {
	return r.List(ctx, tenantID, &accountID)
}

func scanInvoice(scanner rowScanner) (*domain.CreditCardInvoice, error) {
	var (
		inv          domain.CreditCardInvoice
		metadataJSON []byte
	)

	err := scanner.Scan(
		&inv.ID, &inv.TenantID, &inv.AccountID, &inv.ExternalID, &inv.DueDate,
		&inv.ClosingDate, &inv.TotalAmount, &inv.MinimumPayment, &inv.Currency,
		&inv.Status, &inv.AllowsInstallments, &metadataJSON, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &inv.Metadata); err != nil {
			inv.Metadata = make(map[string]any)
		}
	} else {
		inv.Metadata = make(map[string]any)
	}

	return &inv, nil
}
