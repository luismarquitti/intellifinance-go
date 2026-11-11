package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// validateGenericTransactionProvenance reserves original evidence to the
// canonical writer, which retains and validates the original before insertion.
func validateGenericTransactionProvenance(row *domain.Transaction) error {
	for _, key := range []string{"provenance", "transfer_idempotency_key"} {
		if _, exists := row.Metadata[key]; exists {
			return domain.ErrReservedProvenance
		}
	}
	return nil
}

// guardImportedProvenance holds the persisted row lock through the update.
// Classification and notes are editable; original statement facts are not.
// Pair classification is handled by the dedicated atomic transfer writer.
func (r *TransactionRepository) guardImportedProvenance(ctx context.Context, row *domain.Transaction) error {
	var raw []byte
	var originalID, description, cleanDescription, source string
	var accountID uuid.UUID
	var date pgtype.Date
	var amount decimal.Decimal
	var sequence int32
	err := r.getDB(ctx).QueryRow(ctx, `SELECT metadata,COALESCE(external_id,''),account_id,date,amount,description,clean_description,source::text,sequence_index FROM transactions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, row.TenantID, row.ID).Scan(&raw, &originalID, &accountID, &date, &amount, &description, &cleanDescription, &source, &sequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return errors.New("cannot inspect retained provenance")
	}
	var metadata map[string]any
	if err = json.Unmarshal(raw, &metadata); err != nil {
		return errors.New("invalid retained provenance")
	}
	oldKey, oldExists := metadata["transfer_idempotency_key"]
	newKey, newExists := row.Metadata["transfer_idempotency_key"]
	oldJSON, oldErr := json.Marshal(oldKey)
	newJSON, newErr := json.Marshal(newKey)
	if oldExists != newExists || oldErr != nil || newErr != nil || !bytes.Equal(oldJSON, newJSON) {
		return domain.ErrReservedProvenance
	}
	original, ok := metadata["provenance"]
	if !ok {
		if _, introduced := row.Metadata["provenance"]; introduced {
			return domain.ErrReservedProvenance
		}
		return nil
	}
	expected, err := json.Marshal(original)
	if err != nil {
		return errors.New("invalid retained provenance")
	}
	provided, err := json.Marshal(row.Metadata["provenance"])
	if err != nil || !bytes.Equal(expected, provided) || row.ExternalID != originalID || row.AccountID != accountID || !row.Amount.Equal(amount) || !date.Valid || row.Date.Format("2006-01-02") != date.Time.Format("2006-01-02") || row.Description != description || row.CleanDescription != cleanDescription || string(row.Source) != source || row.SequenceIndex != int(sequence) {
		return domain.ErrImmutableImportedTransaction
	}
	return nil
}
