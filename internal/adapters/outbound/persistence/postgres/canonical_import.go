package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres/db"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.CanonicalImportRepository = (*TransactionRepository)(nil)

// ImportCanonical serializes an account's canonical imports. Original IDs are
// matched against retained original row hashes, not editable ledger fields.
// New legitimate repetitions receive a stable sequence allocated in the ledger;
// canonical fingerprints use exactly the project's six-field formula.
func (r *TransactionRepository) ImportCanonical(ctx context.Context, rows []*domain.Transaction) (*ports.CanonicalImportResult, error) {
	result := &ports.CanonicalImportResult{Parsed: len(rows)}
	if len(rows) == 0 {
		return result, nil
	}
	first := rows[0]
	if first == nil || first.CreatedByUserID == nil {
		return nil, errors.New("fixed import identity required")
	}
	for _, row := range rows {
		if row == nil || row.TenantID != first.TenantID || row.AccountID != first.AccountID || row.CreatedByUserID == nil || *row.CreatedByUserID != *first.CreatedByUserID || row.ExternalID == "" {
			return nil, errors.New("canonical import must have one fixed tenant, user and account")
		}
		if err := row.Validate(); err != nil {
			return nil, errors.New("invalid canonical transaction")
		}
	}
	err := WithTenantTx(ctx, r.pool, first.TenantID, *first.CreatedByUserID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, first.TenantID.String()+"|"+first.AccountID.String()); err != nil {
			return errors.New("cannot lock canonical import")
		}
		var cashAccount bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE tenant_id=$1 AND id=$2 AND type IN ('CHECKING','SAVINGS','CASH') AND deleted_at IS NULL)`, first.TenantID, first.AccountID).Scan(&cashAccount); err != nil || !cashAccount {
			return errors.New("canonical cash import requires an existing visible cash account")
		}
		for index, row := range rows {
			provenance, ok := row.Metadata["provenance"].(map[string]any)
			if !ok {
				return errors.New("original provenance required")
			}
			hash, ok := provenance["row_sha256"].(string)
			if !ok || len(hash) != 64 {
				return errors.New("original row hash required")
			}
			if _, err := hex.DecodeString(hash); err != nil {
				return errors.New("invalid original row hash")
			}
			var count int
			var existingHash string
			if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(min(metadata->'provenance'->>'row_sha256'),'') FROM transactions WHERE tenant_id=$1 AND account_id=$2 AND external_id=$3`, row.TenantID, row.AccountID, row.ExternalID).Scan(&count, &existingHash); err != nil {
				return errors.New("cannot inspect original import identity")
			}
			if count > 0 {
				if count != 1 || existingHash != hash {
					return fmt.Errorf("original identity conflict at import record %d; no changes committed", index+1)
				}
				result.Duplicates++
				continue
			}
			if err := tx.QueryRow(ctx, `SELECT COALESCE(max(sequence_index),-1)+1 FROM transactions WHERE tenant_id=$1 AND account_id=$2 AND date=$3 AND amount=$4 AND clean_description=$5`, row.TenantID, row.AccountID, row.Date, row.Amount.StringFixed(2), row.CleanDescription).Scan(&row.SequenceIndex); err != nil {
				return errors.New("cannot allocate canonical sequence")
			}
			row.Fingerprint = domain.ComputeFingerprint(row.TenantID, row.AccountID, row.Date, row.Amount, row.CleanDescription, row.SequenceIndex)
			params, err := toUpsertParams(row)
			if err != nil {
				return errors.New("cannot encode canonical row")
			}
			if _, err := db.New(tx).CreateTransaction(ctx, db.CreateTransactionParams(params)); err != nil {
				return fmt.Errorf("canonical insert failed at record %d; no changes committed", index+1)
			}
			result.Inserted++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
