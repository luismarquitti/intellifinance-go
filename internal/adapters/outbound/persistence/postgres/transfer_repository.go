package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres/db"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.TransferRepository = (*TransferRepository)(nil)

func validateTransferCurrencies(ctx context.Context, tx pgx.Tx, tenantID, a, b uuid.UUID) error {
	rows, err := tx.Query(ctx, `SELECT currency FROM accounts WHERE tenant_id=$1 AND id IN ($2,$3) AND deleted_at IS NULL ORDER BY id ASC FOR NO KEY UPDATE`, tenantID, a, b)
	if err != nil {
		return err
	}
	defer rows.Close()
	currencies := []string{}
	for rows.Next() {
		var currency string
		if err := rows.Scan(&currency); err != nil {
			return err
		}
		currencies = append(currencies, currency)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(currencies) != 2 {
		return domain.ErrNotFound
	}
	if currencies[0] != currencies[1] {
		return domain.ErrInvalidCurrency
	}
	return nil
}

// TransferRepository implements ports.TransferRepository with PostgreSQL transaction-scoped RLS and row-level locks.
type TransferRepository struct {
	pool    *pgxpool.Pool
	tx      pgx.Tx
	queries *db.Queries
}

// NewTransferRepository constructs a TransferRepository instance.
func NewTransferRepository(pool *pgxpool.Pool) *TransferRepository {
	return &TransferRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

// WithTx returns a new TransferRepository bound to a transaction.
func (r *TransferRepository) WithTx(tx pgx.Tx) *TransferRepository {
	return &TransferRepository{
		pool:    r.pool,
		tx:      tx,
		queries: db.New(tx),
	}
}

// withTenantTx borrows an existing transaction without committing or rolling it
// back. The caller owns its tenant-local RLS settings and atomic boundary.
func (r *TransferRepository) withTenantTx(ctx context.Context, tenantID, userID uuid.UUID, fn func(pgx.Tx) error) error {
	if r.tx != nil {
		return fn(r.tx)
	}
	if tx, ok := TxFromContext(ctx); ok {
		return fn(tx)
	}
	return WithTenantTx(ctx, r.pool, tenantID, userID, fn)
}

// ConfirmTransfer links two existing transactions as a confirmed internal transfer pair inside an isolated transaction.
func (r *TransferRepository) ConfirmTransfer(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	if tenantID == uuid.Nil || legAID == uuid.Nil || legBID == uuid.Nil {
		return nil, nil, domain.ErrInvalidID
	}
	if legAID == legBID {
		return nil, nil, domain.ErrInvalidTransferPair
	}

	var resA, resB *domain.Transaction

	err := r.withTenantTx(ctx, tenantID, uuid.Nil, func(tx pgx.Tx) error {
		// Advisory lock or ordered locking to avoid deadlocks
		firstID, secondID := legAID, legBID
		if firstID.String() > secondID.String() {
			firstID, secondID = secondID, firstID
		}

		const selectSQL = `
			SELECT id, tenant_id, account_id, category_id, created_by_user_id, transacted_at, date, amount,
			       description, clean_description, type, status, visibility, source, external_id, sequence_index,
			       fingerprint, is_ai_categorized, ai_confidence, ai_model_version, is_manually_verified,
			       transfer_peer_transaction_id, notes, metadata, created_at, updated_at
			FROM transactions
			WHERE tenant_id = $1 AND id IN ($2, $3)
			ORDER BY id ASC
			FOR UPDATE
		`
		rows, err := tx.Query(ctx, selectSQL, tenantID, firstID, secondID)
		if err != nil {
			return fmt.Errorf("failed to lock transfer legs: %w", err)
		}
		defer rows.Close()

		var txs []*domain.Transaction
		for rows.Next() {
			var row db.Transaction
			if scanErr := rows.Scan(
				&row.ID,
				&row.TenantID,
				&row.AccountID,
				&row.CategoryID,
				&row.CreatedByUserID,
				&row.TransactedAt,
				&row.Date,
				&row.Amount,
				&row.Description,
				&row.CleanDescription,
				&row.Type,
				&row.Status,
				&row.Visibility,
				&row.Source,
				&row.ExternalID,
				&row.SequenceIndex,
				&row.Fingerprint,
				&row.IsAiCategorized,
				&row.AiConfidence,
				&row.AiModelVersion,
				&row.IsManuallyVerified,
				&row.TransferPeerTransactionID,
				&row.Notes,
				&row.Metadata,
				&row.CreatedAt,
				&row.UpdatedAt,
			); scanErr != nil {
				return fmt.Errorf("failed to scan transaction row: %w", scanErr)
			}
			domTx, mapErr := dbToDomainTransaction(&row)
			if mapErr != nil {
				return mapErr
			}
			txs = append(txs, domTx)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return rowsErr
		}

		if len(txs) != 2 {
			return domain.ErrNotFound
		}

		leg1, leg2 := txs[0], txs[1]
		if leg1.ID == legAID {
			resA = leg1
			resB = leg2
		} else {
			resA = leg2
			resB = leg1
		}

		// Domain validation and mutal linking
		if confirmErr := domain.ConfirmTransferPair(resA, resB); confirmErr != nil {
			return confirmErr
		}

		if err := validateTransferCurrencies(ctx, tx, tenantID, resA.AccountID, resB.AccountID); err != nil {
			return err
		}

		// Update both legs
		const updateSQL = `
			UPDATE transactions
			SET type = $3,
			    transfer_peer_transaction_id = $4,
			    updated_at = NOW()
			WHERE tenant_id = $1 AND id = $2
		`
		if _, err := tx.Exec(ctx, updateSQL, tenantID, resA.ID, resA.Type, resA.TransferPeerTransactionID); err != nil {
			return fmt.Errorf("failed to update leg A: %w", err)
		}
		if _, err := tx.Exec(ctx, updateSQL, tenantID, resB.ID, resB.Type, resB.TransferPeerTransactionID); err != nil {
			return fmt.Errorf("failed to update leg B: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}
	return resA, resB, nil
}

// CreateTransfer persists both outbound and inbound legs of an internal transfer atomically.
func (r *TransferRepository) CreateTransfer(ctx context.Context, outflow, inflow *domain.Transaction) error {
	if outflow == nil || inflow == nil {
		return domain.ErrInvalidTransferPair
	}
	if err := domain.ValidateTransferPair(outflow, inflow); err != nil {
		return err
	}
	if !domain.IsRepresentableInCents(outflow.Amount) || !domain.IsRepresentableInCents(inflow.Amount) ||
		outflow.Amount.Abs().GreaterThan(domain.MaxMonetaryAmount) || inflow.Amount.Abs().GreaterThan(domain.MaxMonetaryAmount) {
		return domain.ErrInvalidAmount
	}
	if !outflow.Amount.IsNegative() || !inflow.Amount.IsPositive() || outflow.Type != domain.TransactionTypeTransfer || inflow.Type != domain.TransactionTypeTransfer || outflow.Status != domain.TransactionStatusCompleted || inflow.Status != domain.TransactionStatusCompleted || outflow.TransferPeerTransactionID == nil || *outflow.TransferPeerTransactionID != inflow.ID || inflow.TransferPeerTransactionID == nil || *inflow.TransferPeerTransactionID != outflow.ID {
		return domain.ErrInvalidTransferPair
	}

	var userID uuid.UUID
	if outflow.CreatedByUserID != nil {
		userID = *outflow.CreatedByUserID
	}

	return r.withTenantTx(ctx, outflow.TenantID, userID, func(tx pgx.Tx) error {
		key, ok := outflow.Metadata["transfer_idempotency_key"].(string)
		if !ok || strings.TrimSpace(key) == "" || len(key) > 200 || inflow.Metadata["transfer_idempotency_key"] != key {
			return domain.ErrInvalidTransferPair
		}
		// Serialize the tenant-wide key before account locks, including divergent payloads.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, outflow.TenantID.String()+"|transfer|"+key); err != nil {
			return err
		}
		var existingID uuid.UUID
		lookupErr := tx.QueryRow(ctx, `SELECT id FROM transactions WHERE tenant_id=$1 AND metadata->>'transfer_idempotency_key'=$2 AND amount<0`, outflow.TenantID, key).Scan(&existingID)
		if lookupErr == nil {
			ledger := NewTransactionRepository(r.pool).WithTx(tx)
			a, err := ledger.GetByID(ctx, outflow.TenantID, existingID)
			if err != nil {
				return err
			}
			if a.TransferPeerTransactionID == nil {
				return domain.ErrDuplicateTransaction
			}
			b, err := ledger.GetByID(ctx, outflow.TenantID, *a.TransferPeerTransactionID)
			if err != nil {
				return err
			}
			if a.AccountID != outflow.AccountID || b.AccountID != inflow.AccountID || !a.Amount.Equal(outflow.Amount) || !b.Amount.Equal(inflow.Amount) || a.Date.Format("2006-01-02") != outflow.Date.Format("2006-01-02") || b.Date.Format("2006-01-02") != inflow.Date.Format("2006-01-02") || a.Description != outflow.Description || b.Description != inflow.Description || a.Type != domain.TransactionTypeTransfer || b.Type != domain.TransactionTypeTransfer || b.TransferPeerTransactionID == nil || *b.TransferPeerTransactionID != a.ID {
				return domain.ErrDuplicateTransaction
			}
			*outflow = *a
			*inflow = *b
			return nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return lookupErr
		}
		// Share allocation locks with canonical import, in deterministic account order.
		accountIDs := []uuid.UUID{outflow.AccountID, inflow.AccountID}
		if accountIDs[0].String() > accountIDs[1].String() {
			accountIDs[0], accountIDs[1] = accountIDs[1], accountIDs[0]
		}
		for _, accountID := range accountIDs {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, outflow.TenantID.String()+"|"+accountID.String()); err != nil {
				return err
			}
		}
		if err := validateTransferCurrencies(ctx, tx, outflow.TenantID, outflow.AccountID, inflow.AccountID); err != nil {
			return err
		}
		// Insert strictly (never ON CONFLICT DO NOTHING), then link after both FKs exist.
		for _, leg := range []*domain.Transaction{outflow, inflow} {
			if err := tx.QueryRow(ctx, `SELECT COALESCE(max(sequence_index),-1)+1 FROM transactions WHERE tenant_id=$1 AND account_id=$2 AND date=$3 AND amount=$4 AND clean_description=$5`, leg.TenantID, leg.AccountID, leg.Date, leg.Amount.StringFixed(2), leg.CleanDescription).Scan(&leg.SequenceIndex); err != nil {
				return err
			}
			leg.Fingerprint = domain.ComputeFingerprint(leg.TenantID, leg.AccountID, leg.Date, leg.Amount, leg.CleanDescription, leg.SequenceIndex)
			copyLeg := *leg
			copyLeg.TransferPeerTransactionID = nil
			params, err := toUpsertParams(&copyLeg)
			if err != nil {
				return err
			}
			if _, err := db.New(tx).CreateTransaction(ctx, db.CreateTransactionParams(params)); err != nil {
				return err
			}
		}
		for _, leg := range []*domain.Transaction{outflow, inflow} {
			tag, err := tx.Exec(ctx, `UPDATE transactions SET transfer_peer_transaction_id=$3 WHERE tenant_id=$1 AND id=$2`, leg.TenantID, leg.ID, leg.TransferPeerTransactionID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
		}
		ledger := NewTransactionRepository(r.pool).WithTx(tx)
		storedOut, err := ledger.GetByID(ctx, outflow.TenantID, outflow.ID)
		if err != nil {
			return err
		}
		storedIn, err := ledger.GetByID(ctx, inflow.TenantID, inflow.ID)
		if err != nil {
			return err
		}
		*outflow = *storedOut
		*inflow = *storedIn
		return nil
	})
}

// UnlinkTransfer atomically unlinks a transfer pair without deleting either leg.
func (r *TransferRepository) UnlinkTransfer(ctx context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	if tenantID == uuid.Nil || txID == uuid.Nil {
		return nil, nil, domain.ErrInvalidID
	}

	var resA, resB *domain.Transaction

	err := r.withTenantTx(ctx, tenantID, uuid.Nil, func(tx pgx.Tx) error {
		var peerID *uuid.UUID
		// Discover the peer without taking an endpoint lock. The ordered pair
		// read below revalidates mutual linkage after concurrent unlink/relink.
		err := tx.QueryRow(ctx, `
			SELECT transfer_peer_transaction_id
			FROM transactions
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, txID).Scan(&peerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("failed to query transaction: %w", err)
		}
		if peerID == nil {
			return domain.ErrTransferNotLinked
		}

		firstID, secondID := txID, *peerID
		if firstID.String() > secondID.String() {
			firstID, secondID = secondID, firstID
		}

		const selectSQL = `
			SELECT id, tenant_id, account_id, category_id, created_by_user_id, transacted_at, date, amount,
			       description, clean_description, type, status, visibility, source, external_id, sequence_index,
			       fingerprint, is_ai_categorized, ai_confidence, ai_model_version, is_manually_verified,
			       transfer_peer_transaction_id, notes, metadata, created_at, updated_at
			FROM transactions
			WHERE tenant_id = $1 AND id IN ($2, $3)
			ORDER BY id ASC
			FOR UPDATE
		`
		rows, err := tx.Query(ctx, selectSQL, tenantID, firstID, secondID)
		if err != nil {
			return fmt.Errorf("failed to lock transfer legs: %w", err)
		}
		defer rows.Close()

		var txs []*domain.Transaction
		for rows.Next() {
			var row db.Transaction
			if scanErr := rows.Scan(
				&row.ID,
				&row.TenantID,
				&row.AccountID,
				&row.CategoryID,
				&row.CreatedByUserID,
				&row.TransactedAt,
				&row.Date,
				&row.Amount,
				&row.Description,
				&row.CleanDescription,
				&row.Type,
				&row.Status,
				&row.Visibility,
				&row.Source,
				&row.ExternalID,
				&row.SequenceIndex,
				&row.Fingerprint,
				&row.IsAiCategorized,
				&row.AiConfidence,
				&row.AiModelVersion,
				&row.IsManuallyVerified,
				&row.TransferPeerTransactionID,
				&row.Notes,
				&row.Metadata,
				&row.CreatedAt,
				&row.UpdatedAt,
			); scanErr != nil {
				return fmt.Errorf("failed to scan transaction row: %w", scanErr)
			}
			domTx, mapErr := dbToDomainTransaction(&row)
			if mapErr != nil {
				return mapErr
			}
			txs = append(txs, domTx)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return rowsErr
		}

		if len(txs) != 2 {
			return domain.ErrNotFound
		}

		if txs[0].ID == txID {
			resA = txs[0]
			resB = txs[1]
		} else {
			resA = txs[1]
			resB = txs[0]
		}

		if unlinkErr := domain.UnlinkTransferPair(resA, resB); unlinkErr != nil {
			return unlinkErr
		}

		const updateSQL = `
			UPDATE transactions
			SET type = $3,
			    transfer_peer_transaction_id = NULL,
			    updated_at = NOW()
			WHERE tenant_id = $1 AND id = $2
		`
		if _, err := tx.Exec(ctx, updateSQL, tenantID, resA.ID, resA.Type); err != nil {
			return fmt.Errorf("failed to update leg A: %w", err)
		}
		if _, err := tx.Exec(ctx, updateSQL, tenantID, resB.ID, resB.Type); err != nil {
			return fmt.Errorf("failed to update leg B: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}
	return resA, resB, nil
}
