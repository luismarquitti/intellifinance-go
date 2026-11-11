package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// guardTransferUpdate serializes against confirmation/unlink and compares the
// persisted pair fields, not a handler's potentially stale transaction snapshot.
// Only the dedicated pair writer may add or remove a peer.
func (r *TransactionRepository) guardTransferUpdate(ctx context.Context, row *domain.Transaction) error {
	var peer *uuid.UUID
	var account uuid.UUID
	var amount decimal.Decimal
	var kind, status string
	err := r.getDB(ctx).QueryRow(ctx, `SELECT transfer_peer_transaction_id,account_id,amount,type::text,status::text FROM transactions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, row.TenantID, row.ID).Scan(&peer, &account, &amount, &kind, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("cannot inspect transfer linkage: %w", err)
	}
	if peer == nil {
		if row.TransferPeerTransactionID != nil {
			return domain.ErrTransferAlreadyLinked
		}
		return nil
	}
	if row.TransferPeerTransactionID == nil || *row.TransferPeerTransactionID != *peer || row.AccountID != account || !row.Amount.Equal(amount) || string(row.Type) != kind || string(row.Status) != status {
		return domain.ErrTransferAlreadyLinked
	}
	return nil
}

// guardTransferDeletion locks all selected records in the same order used by
// pair operations. A concurrent confirmation is observed before any deletion.
// Even deleting both legs requires explicit unlink first.
func (r *TransactionRepository) guardTransferDeletion(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) error {
	rows, err := r.getDB(ctx).Query(ctx, `SELECT transfer_peer_transaction_id FROM transactions WHERE tenant_id=$1 AND id=ANY($2::uuid[]) ORDER BY id FOR UPDATE`, tenant, ids)
	if err != nil {
		return fmt.Errorf("cannot inspect transfer deletion: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var peer *uuid.UUID
		if err = rows.Scan(&peer); err != nil {
			return err
		}
		if peer != nil {
			return domain.ErrTransferAlreadyLinked
		}
	}
	return rows.Err()
}
