package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres/db"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

var _ ports.TransactionRepository = (*TransactionRepository)(nil)

// TransactionRepository implements ports.TransactionRepository using PostgreSQL via sqlc and pgx/v5.
type TransactionRepository struct {
	pool    *pgxpool.Pool
	tx      pgx.Tx
	queries *db.Queries
}

// NewTransactionRepository creates a new TransactionRepository instance.
func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

// WithTx returns a new TransactionRepository instance bound to a specific pgx.Tx transaction.
func (r *TransactionRepository) WithTx(tx pgx.Tx) *TransactionRepository {
	return &TransactionRepository{
		pool:    r.pool,
		tx:      tx,
		queries: db.New(tx),
	}
}

func (r *TransactionRepository) getDB(ctx context.Context) db.DBTX {
	if r.tx != nil {
		return r.tx
	}
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

func (r *TransactionRepository) getQueries(ctx context.Context) *db.Queries {
	if r.tx != nil {
		return r.queries
	}
	if tx, ok := TxFromContext(ctx); ok {
		return db.New(tx)
	}
	return r.queries
}

// Create persists a single transaction with idempotent ON CONFLICT (tenant_id, fingerprint) DO NOTHING policy.
func (r *TransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	if tx.TransferPeerTransactionID != nil {
		return domain.ErrTransferAlreadyLinked
	}
	if err := validateGenericTransactionProvenance(tx); err != nil {
		return err
	}
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var userID uuid.UUID
			if tx.CreatedByUserID != nil {
				userID = *tx.CreatedByUserID
			}
			return WithTenantTx(ctx, r.pool, tx.TenantID, userID, func(pgTx pgx.Tx) error {
				return r.WithTx(pgTx).Create(ctx, tx)
			})
		}
	}

	params, err := toUpsertParams(tx)
	if err != nil {
		return err
	}

	q := r.getQueries(ctx)
	if err := q.UpsertTransaction(ctx, params); err != nil {
		return fmt.Errorf("failed to upsert transaction: %w", err)
	}
	if tx.BillID != nil && *tx.BillID != uuid.Nil {
		const setBillSQL = `UPDATE transactions SET bill_id = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2;`
		if _, err := r.getDB(ctx).Exec(ctx, setBillSQL, tx.ID, tx.TenantID, *tx.BillID); err != nil {
			return fmt.Errorf("failed to set bill_id on transaction create: %w", err)
		}
	}

	return nil
}

// BatchCreate persists multiple transactions atomically or in high-throughput batches
// with idempotent ON CONFLICT (tenant_id, fingerprint) DO NOTHING policy.
func (r *TransactionRepository) BatchCreate(ctx context.Context, txs []*domain.Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	for _, tx := range txs {
		if tx.TransferPeerTransactionID != nil {
			return domain.ErrTransferAlreadyLinked
		}
		if err := validateGenericTransactionProvenance(tx); err != nil {
			return err
		}
		if err := tx.Validate(); err != nil {
			return fmt.Errorf("invalid transaction in batch: %w", err)
		}
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			tenantID := txs[0].TenantID
			var userID uuid.UUID
			if txs[0].CreatedByUserID != nil {
				userID = *txs[0].CreatedByUserID
			}
			return WithTenantTx(ctx, r.pool, tenantID, userID, func(pgTx pgx.Tx) error {
				return r.WithTx(pgTx).BatchCreate(ctx, txs)
			})
		}
	}

	dbtx := r.getDB(ctx)

	type batchSender interface {
		SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
	}

	if sender, ok := dbtx.(batchSender); ok {
		batch := &pgx.Batch{}
		const upsertSQL = `-- name: UpsertTransaction :exec
INSERT INTO transactions (
    id,
    tenant_id,
    account_id,
    category_id,
    created_by_user_id,
    transacted_at,
    date,
    amount,
    description,
    clean_description,
    type,
    status,
    visibility,
    source,
    external_id,
    sequence_index,
    fingerprint,
    is_ai_categorized,
    ai_confidence,
    ai_model_version,
    is_manually_verified,
    transfer_peer_transaction_id,
    notes,
    metadata,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26
) ON CONFLICT (tenant_id, fingerprint) DO NOTHING;`

		queued := 0
		for _, tx := range txs {
			params, err := toUpsertParams(tx)
			if err != nil {
				return err
			}
			batch.Queue(upsertSQL,
				params.ID,
				params.TenantID,
				params.AccountID,
				params.CategoryID,
				params.CreatedByUserID,
				params.TransactedAt,
				params.Date,
				params.Amount,
				params.Description,
				params.CleanDescription,
				params.Type,
				params.Status,
				params.Visibility,
				params.Source,
				params.ExternalID,
				params.SequenceIndex,
				params.Fingerprint,
				params.IsAiCategorized,
				params.AiConfidence,
				params.AiModelVersion,
				params.IsManuallyVerified,
				params.TransferPeerTransactionID,
				params.Notes,
				params.Metadata,
				params.CreatedAt,
				params.UpdatedAt,
			)
			queued++
			if tx.BillID != nil && *tx.BillID != uuid.Nil {
				batch.Queue(`UPDATE transactions SET bill_id = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2;`, tx.ID, tx.TenantID, *tx.BillID)
				queued++
			}
		}

		br := sender.SendBatch(ctx, batch)
		defer func() {
			_ = br.Close()
		}()

		for i := 0; i < queued; i++ {
			if _, err := br.Exec(); err != nil {
				return fmt.Errorf("failed executing batch upsert: %w", err)
			}
		}
		return nil
	}

	q := r.getQueries(ctx)
	for _, tx := range txs {
		params, err := toUpsertParams(tx)
		if err != nil {
			return err
		}
		if err := q.UpsertTransaction(ctx, params); err != nil {
			return fmt.Errorf("failed to upsert transaction in loop: %w", err)
		}
		if tx.BillID != nil && *tx.BillID != uuid.Nil {
			const setBillSQL = `UPDATE transactions SET bill_id = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2;`
			if _, err := dbtx.Exec(ctx, setBillSQL, tx.ID, tx.TenantID, *tx.BillID); err != nil {
				return fmt.Errorf("failed to set bill_id in batch create: %w", err)
			}
		}
	}

	return nil
}

// GetByID retrieves a transaction by ID within a tenant boundary.
func (r *TransactionRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var result *domain.Transaction
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				result, err = r.WithTx(tx).GetByID(ctx, tenantID, id)
				return err
			})
			return result, err
		}
	}

	q := r.getQueries(ctx)
	row, err := q.GetTransactionByID(ctx, db.GetTransactionByIDParams{
		ID:       id,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get transaction by ID: %w", err)
	}
	return dbToDomainTransaction(&row)
}

// List queries paginated transactions and returns items and total count.
func (r *TransactionRepository) List(ctx context.Context, tenantID uuid.UUID, filter ports.TransactionFilter) ([]*domain.Transaction, int64, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var items []*domain.Transaction
			var total int64
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				items, total, err = r.WithTx(tx).List(ctx, tenantID, filter)
				return err
			})
			return items, total, err
		}
	}

	q := r.getQueries(ctx)
	limit := int32(50)
	if filter.Limit > 0 && filter.Limit <= 1000 {
		limit = int32(filter.Limit) // #nosec G115 -- guarded range
	}
	page := int32(1)
	if filter.Page > 0 && filter.Page <= 1_000_000 {
		page = int32(filter.Page) // #nosec G115 -- guarded range
	}
	offset := (page - 1) * limit

	var txType db.NullTransactionType
	if filter.Type != nil {
		txType = db.NullTransactionType{TransactionType: db.TransactionType(*filter.Type), Valid: true}
	}

	var fromDate, toDate pgtype.Date
	if filter.FromDate != nil {
		fromDate = pgtype.Date{Time: *filter.FromDate, Valid: true}
	}
	if filter.ToDate != nil {
		toDate = pgtype.Date{Time: *filter.ToDate, Valid: true}
	}

	var search *string
	if strings.TrimSpace(filter.Search) != "" {
		s := strings.TrimSpace(filter.Search)
		search = &s
	}

	sortBy := "date"
	if filter.SortBy == "amount" || filter.SortBy == "description" || filter.SortBy == "date" {
		sortBy = filter.SortBy
	}

	sortOrder := "desc"
	if strings.ToLower(filter.SortOrder) == "asc" {
		sortOrder = "asc"
	}

	total, err := q.CountTransactionsFiltered(ctx, db.CountTransactionsFilteredParams{
		TenantID:        tenantID,
		AccountID:       filter.AccountID,
		CategoryID:      filter.CategoryID,
		IsUncategorized: filter.Uncategorized,
		Type:            txType,
		FromDate:        fromDate,
		ToDate:          toDate,
		Search:          search,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	rows, err := q.ListTransactionsFiltered(ctx, db.ListTransactionsFilteredParams{
		TenantID:        tenantID,
		Limit:           limit,
		Offset:          offset,
		AccountID:       filter.AccountID,
		CategoryID:      filter.CategoryID,
		IsUncategorized: filter.Uncategorized,
		Type:            txType,
		FromDate:        fromDate,
		ToDate:          toDate,
		Search:          search,
		SortBy:          sortBy,
		SortOrder:       sortOrder,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list transactions: %w", err)
	}

	items := make([]*domain.Transaction, len(rows))
	for i := range rows {
		tx, err := dbToDomainTransaction(&rows[i])
		if err != nil {
			return nil, 0, err
		}
		items[i] = tx
	}

	return items, total, nil
}

// Summarize aggregates KPI metrics matching the filter criteria, including initial and final period balances.
func (r *TransactionRepository) Summarize(ctx context.Context, tenantID uuid.UUID, filter ports.TransactionFilter) (*ports.TransactionSummary, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var summary *ports.TransactionSummary
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				summary, err = r.WithTx(tx).Summarize(ctx, tenantID, filter)
				return err
			})
			return summary, err
		}
	}

	dbtx := r.getDB(ctx)

	scopeLiquidOnly := false
	if filter.AccountID == nil {
		var liquidCount int64
		const liquidCountSQL = `
			SELECT COUNT(*)::bigint
			FROM accounts
			WHERE tenant_id = $1
			  AND type <> 'CREDIT_CARD'
			  AND is_active = TRUE
			  AND deleted_at IS NULL;`
		if err := dbtx.QueryRow(ctx, liquidCountSQL, tenantID).Scan(&liquidCount); err != nil {
			return nil, fmt.Errorf("failed to count liquid accounts: %w", err)
		}
		scopeLiquidOnly = liquidCount > 0
	}

	var fromDate, toDate pgtype.Date
	if filter.FromDate != nil {
		fromDate = pgtype.Date{Time: *filter.FromDate, Valid: true}
	}
	if filter.ToDate != nil {
		toDate = pgtype.Date{Time: *filter.ToDate, Valid: true}
	}

	var currentBalance, deltaFromStart decimal.Decimal
	const balanceAnchorSQL = `
		WITH target_accounts AS (
			SELECT id, balance
			FROM accounts
			WHERE tenant_id = $1
			  AND deleted_at IS NULL
			  AND (
				  ($2::uuid IS NOT NULL AND id = $2::uuid)
				  OR ($2::uuid IS NULL AND is_active = TRUE AND ($3::boolean = FALSE OR type <> 'CREDIT_CARD'))
			  )
		)
		SELECT
			COALESCE((SELECT SUM(balance) FROM target_accounts), 0)::numeric(15,2) AS current_balance,
			COALESCE((
				SELECT SUM(t.amount)
				FROM transactions t
				JOIN target_accounts ta ON ta.id = t.account_id
				WHERE t.tenant_id = $1
				  AND t.status IN ('COMPLETED', 'RECONCILED')
				  AND ($4::date IS NULL OR t.date >= $4::date)
			), 0)::numeric(15,2) AS delta_from_start;`

	if err := dbtx.QueryRow(ctx, balanceAnchorSQL,
		tenantID,
		filter.AccountID,
		scopeLiquidOnly,
		fromDate,
	).Scan(&currentBalance, &deltaFromStart); err != nil {
		return nil, fmt.Errorf("failed to compute opening balance: %w", err)
	}

	initialBalance := currentBalance.Sub(deltaFromStart).Round(2)

	var txType db.NullTransactionType
	if filter.Type != nil {
		txType = db.NullTransactionType{TransactionType: db.TransactionType(*filter.Type), Valid: true}
	}

	var txStatus db.NullTransactionStatus
	if filter.Status != nil {
		txStatus = db.NullTransactionStatus{TransactionStatus: db.TransactionStatus(*filter.Status), Valid: true}
	}

	var search *string
	if strings.TrimSpace(filter.Search) != "" {
		s := strings.TrimSpace(filter.Search)
		search = &s
	}

	var totalIncome, totalExpense, netBalance decimal.Decimal
	var count int64
	const summarySQL = `
		WITH target_accounts AS (
			SELECT id
			FROM accounts
			WHERE tenant_id = $1
			  AND deleted_at IS NULL
			  AND (
				  ($2::uuid IS NOT NULL AND id = $2::uuid)
				  OR ($2::uuid IS NULL AND is_active = TRUE AND ($3::boolean = FALSE OR type <> 'CREDIT_CARD'))
			  )
		)
		SELECT
			COALESCE(SUM(CASE WHEN peer_ta.id IS NULL AND t.amount > 0 THEN t.amount ELSE 0 END), 0)::numeric(15,2) AS total_income,
			COALESCE(SUM(CASE WHEN peer_ta.id IS NULL AND t.amount < 0 THEN ABS(t.amount) ELSE 0 END), 0)::numeric(15,2) AS total_expense,
			COALESCE(SUM(CASE WHEN peer_ta.id IS NULL THEN t.amount ELSE 0 END), 0)::numeric(15,2) AS net_balance,
			COUNT(*)::bigint AS count
		FROM transactions t
		JOIN target_accounts ta ON ta.id = t.account_id
		LEFT JOIN transactions peer ON peer.id = t.transfer_peer_transaction_id AND peer.tenant_id = t.tenant_id
		LEFT JOIN target_accounts peer_ta ON peer_ta.id = peer.account_id
		WHERE t.tenant_id = $1
		  AND (
			  ($10::transaction_status IS NOT NULL AND t.status = $10::transaction_status)
			  OR ($10::transaction_status IS NULL AND t.status IN ('COMPLETED', 'RECONCILED'))
		  )
		  AND ($4::uuid IS NULL OR t.category_id = $4::uuid)
		  AND ($5::boolean IS NULL OR ($5::boolean = TRUE AND t.category_id IS NULL))
		  AND ($6::transaction_type IS NULL OR t.type = $6::transaction_type)
		  AND ($7::date IS NULL OR t.date >= $7::date)
		  AND ($8::date IS NULL OR t.date <= $8::date)
		  AND ($9::text IS NULL OR t.description ILIKE '%' || $9::text || '%' OR t.notes ILIKE '%' || $9::text || '%');`

	if err := dbtx.QueryRow(ctx, summarySQL,
		tenantID,
		filter.AccountID,
		scopeLiquidOnly,
		filter.CategoryID,
		filter.Uncategorized,
		txType,
		fromDate,
		toDate,
		search,
		txStatus,
	).Scan(&totalIncome, &totalExpense, &netBalance, &count); err != nil {
		return nil, fmt.Errorf("failed to summarize transactions: %w", err)
	}

	var finalBalance decimal.Decimal
	hasSubLedgerFilter := filter.CategoryID != nil || filter.Uncategorized != nil || filter.Type != nil || search != nil || filter.Status != nil
	if !hasSubLedgerFilter {
		finalBalance = initialBalance.Add(totalIncome).Sub(totalExpense).Round(2)
	} else {
		finalBalance = initialBalance.Add(netBalance).Round(2)
	}

	nowBrazil := time.Now().In(domain.BrazilLocation)
	isCurrentMonth := false
	if filter.FromDate == nil && filter.ToDate == nil {
		isCurrentMonth = true
	} else if filter.FromDate != nil && filter.ToDate != nil {
		isCurrentMonth = filter.FromDate.Year() == nowBrazil.Year() && filter.FromDate.Month() == nowBrazil.Month()
	}

	return &ports.TransactionSummary{
		InitialBalance: initialBalance,
		TotalIncome:    totalIncome.Round(2),
		TotalExpense:   totalExpense.Round(2),
		NetBalance:     netBalance.Round(2),
		FinalBalance:   finalBalance,
		IsCurrentMonth: isCurrentMonth,
		Count:          count,
	}, nil
}

// BatchDelete removes multiple transactions by ID within a tenant boundary.
func (r *TransactionRepository) BatchDelete(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).BatchDelete(ctx, tenantID, ids)
			})
		}
	}

	if err := r.guardTransferDeletion(ctx, tenantID, ids); err != nil {
		return err
	}
	q := r.getQueries(ctx)
	if err := q.BatchDeleteTransactions(ctx, db.BatchDeleteTransactionsParams{
		TenantID: tenantID,
		Ids:      ids,
	}); err != nil {
		return fmt.Errorf("failed to batch delete transactions: %w", err)
	}

	return nil
}

// BatchUpdateCategory sets the category for multiple transactions within a tenant boundary.
func (r *TransactionRepository) BatchUpdateCategory(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, categoryID *uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).BatchUpdateCategory(ctx, tenantID, ids, categoryID)
			})
		}
	}

	q := r.getQueries(ctx)
	if err := q.BatchUpdateTransactionCategory(ctx, db.BatchUpdateTransactionCategoryParams{
		TenantID:   tenantID,
		CategoryID: categoryID,
		Ids:        ids,
	}); err != nil {
		return fmt.Errorf("failed to batch update transaction categories: %w", err)
	}

	return nil
}

// Update persists mutations to an existing transaction.
func (r *TransactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var userID uuid.UUID
			if tx.CreatedByUserID != nil {
				userID = *tx.CreatedByUserID
			}
			return WithTenantTx(ctx, r.pool, tx.TenantID, userID, func(pgTx pgx.Tx) error {
				return r.WithTx(pgTx).Update(ctx, tx)
			})
		}
	}

	if err := r.guardTransferUpdate(ctx, tx); err != nil {
		return err
	}
	if err := r.guardImportedProvenance(ctx, tx); err != nil {
		return err
	}
	params, err := toUpdateParams(tx)
	if err != nil {
		return err
	}

	q := r.getQueries(ctx)
	_, err = q.UpdateTransaction(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}

// Delete removes a transaction by ID.
func (r *TransactionRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).Delete(ctx, tenantID, id)
			})
		}
	}

	if err := r.guardTransferDeletion(ctx, tenantID, []uuid.UUID{id}); err != nil {
		return err
	}
	q := r.getQueries(ctx)
	err := q.DeleteTransaction(ctx, db.DeleteTransactionParams{
		ID:       id,
		TenantID: tenantID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}
	return nil
}

// FindExistingFingerprints returns a map of fingerprints already stored for the tenant.
func (r *TransactionRepository) FindExistingFingerprints(
	ctx context.Context,
	tenantID uuid.UUID,
	fingerprints []string,
) (map[string]bool, error) {
	if len(fingerprints) == 0 {
		return make(map[string]bool), nil
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var result map[string]bool
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				result, err = r.WithTx(tx).FindExistingFingerprints(ctx, tenantID, fingerprints)
				return err
			})
			return result, err
		}
	}

	q := r.getQueries(ctx)
	found, err := q.FindExistingFingerprints(ctx, db.FindExistingFingerprintsParams{
		TenantID:     tenantID,
		Fingerprints: fingerprints,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find existing fingerprints: %w", err)
	}

	result := make(map[string]bool, len(found))
	for _, fp := range found {
		result[fp] = true
	}
	return result, nil
}

// CreateSplits persists attribution splits for a transaction.
func (r *TransactionRepository) CreateSplits(ctx context.Context, tenantID, txID uuid.UUID, splits []domain.Split) error {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).CreateSplits(ctx, tenantID, txID, splits)
			})
		}
	}

	q := r.getQueries(ctx)
	for _, s := range splits {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("invalid split: %w", err)
		}

		var notes *string
		if s.Notes != "" {
			notesVal := s.Notes
			notes = &notesVal
		}
		now := time.Now().UTC()
		_, err := q.CreateTransactionSplit(ctx, db.CreateTransactionSplitParams{
			ID:            s.ID,
			TenantID:      tenantID,
			TransactionID: txID,
			UserID:        s.UserID,
			Amount:        s.Amount,
			Percentage:    s.Percentage,
			Notes:         notes,
			CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create transaction split: %w", err)
		}
	}
	return nil
}

// GetSplits retrieves all attribution splits for a transaction.
func (r *TransactionRepository) GetSplits(ctx context.Context, tenantID, txID uuid.UUID) ([]domain.Split, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var result []domain.Split
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				result, err = r.WithTx(tx).GetSplits(ctx, tenantID, txID)
				return err
			})
			return result, err
		}
	}

	q := r.getQueries(ctx)
	rows, err := q.GetTransactionSplits(ctx, db.GetTransactionSplitsParams{
		TenantID:      tenantID,
		TransactionID: txID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction splits: %w", err)
	}

	splits := make([]domain.Split, len(rows))
	for i, r := range rows {
		var notes string
		if r.Notes != nil {
			notes = *r.Notes
		}
		splits[i] = domain.Split{
			ID:            r.ID,
			TenantID:      r.TenantID,
			TransactionID: r.TransactionID,
			UserID:        r.UserID,
			Amount:        r.Amount,
			Percentage:    r.Percentage,
			Notes:         notes,
			CreatedAt:     r.CreatedAt.Time,
			UpdatedAt:     r.UpdatedAt.Time,
		}
	}
	return splits, nil
}

func toUpsertParams(tx *domain.Transaction) (db.UpsertTransactionParams, error) {
	var extID *string
	if tx.ExternalID != "" {
		extID = &tx.ExternalID
	}

	var aiModel *string
	if tx.AIModelVersion != "" {
		aiModel = &tx.AIModelVersion
	}

	var notes *string
	if tx.Notes != "" {
		notes = &tx.Notes
	}

	metaBytes := json.RawMessage(`{}`)
	if len(tx.Metadata) > 0 {
		b, err := json.Marshal(tx.Metadata)
		if err != nil {
			return db.UpsertTransactionParams{}, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metaBytes = json.RawMessage(b)
	}

	createdAt := tx.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := tx.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	seqIndex := int32(0)
	if tx.SequenceIndex > 0 && tx.SequenceIndex <= 1_000_000 {
		seqIndex = int32(tx.SequenceIndex) // #nosec G115 -- guarded range
	}

	return db.UpsertTransactionParams{
		ID:                        tx.ID,
		TenantID:                  tx.TenantID,
		AccountID:                 tx.AccountID,
		CategoryID:                tx.CategoryID,
		CreatedByUserID:           tx.CreatedByUserID,
		TransactedAt:              pgtype.Timestamptz{Time: tx.TransactedAt, Valid: !tx.TransactedAt.IsZero()},
		Date:                      pgtype.Date{Time: tx.Date, Valid: !tx.Date.IsZero()},
		Amount:                    tx.Amount,
		Description:               tx.Description,
		CleanDescription:          tx.CleanDescription,
		Type:                      db.TransactionType(tx.Type),
		Status:                    db.TransactionStatus(tx.Status),
		Visibility:                db.VisibilityType(tx.Visibility),
		Source:                    db.SourceType(tx.Source),
		ExternalID:                extID,
		SequenceIndex:             seqIndex,
		Fingerprint:               tx.Fingerprint,
		IsAiCategorized:           tx.IsAICategorized,
		AiConfidence:              tx.AIConfidence,
		AiModelVersion:            aiModel,
		IsManuallyVerified:        tx.IsManuallyVerified,
		TransferPeerTransactionID: tx.TransferPeerTransactionID,
		Notes:                     notes,
		Metadata:                  metaBytes,
		CreatedAt:                 pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt:                 pgtype.Timestamptz{Time: updatedAt, Valid: true},
	}, nil
}

func toUpdateParams(tx *domain.Transaction) (db.UpdateTransactionParams, error) {
	var extID *string
	if tx.ExternalID != "" {
		extID = &tx.ExternalID
	}

	var aiModel *string
	if tx.AIModelVersion != "" {
		aiModel = &tx.AIModelVersion
	}

	var notes *string
	if tx.Notes != "" {
		notes = &tx.Notes
	}

	metaBytes := json.RawMessage(`{}`)
	if len(tx.Metadata) > 0 {
		b, err := json.Marshal(tx.Metadata)
		if err != nil {
			return db.UpdateTransactionParams{}, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metaBytes = json.RawMessage(b)
	}

	seqIndex := int32(0)
	if tx.SequenceIndex > 0 && tx.SequenceIndex <= 1_000_000 {
		seqIndex = int32(tx.SequenceIndex) // #nosec G115 -- guarded range
	}

	return db.UpdateTransactionParams{
		ID:                        tx.ID,
		TenantID:                  tx.TenantID,
		AccountID:                 tx.AccountID,
		CategoryID:                tx.CategoryID,
		TransactedAt:              pgtype.Timestamptz{Time: tx.TransactedAt, Valid: !tx.TransactedAt.IsZero()},
		Date:                      pgtype.Date{Time: tx.Date, Valid: !tx.Date.IsZero()},
		Amount:                    tx.Amount,
		Description:               tx.Description,
		CleanDescription:          tx.CleanDescription,
		Type:                      db.TransactionType(tx.Type),
		Status:                    db.TransactionStatus(tx.Status),
		Visibility:                db.VisibilityType(tx.Visibility),
		Source:                    db.SourceType(tx.Source),
		ExternalID:                extID,
		SequenceIndex:             seqIndex,
		IsAiCategorized:           tx.IsAICategorized,
		AiConfidence:              tx.AIConfidence,
		AiModelVersion:            aiModel,
		IsManuallyVerified:        tx.IsManuallyVerified,
		TransferPeerTransactionID: tx.TransferPeerTransactionID,
		Notes:                     notes,
		Metadata:                  metaBytes,
	}, nil
}

func dbToDomainTransaction(row *db.Transaction) (*domain.Transaction, error) {
	var extID string
	if row.ExternalID != nil {
		extID = *row.ExternalID
	}

	var aiModel string
	if row.AiModelVersion != nil {
		aiModel = *row.AiModelVersion
	}

	var notes string
	if row.Notes != nil {
		notes = *row.Notes
	}

	meta := make(map[string]any)
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &meta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal transaction metadata: %w", err)
		}
	}

	return &domain.Transaction{
		ID:                        row.ID,
		TenantID:                  row.TenantID,
		AccountID:                 row.AccountID,
		CategoryID:                row.CategoryID,
		CreatedByUserID:           row.CreatedByUserID,
		TransactedAt:              row.TransactedAt.Time,
		Date:                      row.Date.Time,
		Amount:                    row.Amount,
		Description:               row.Description,
		CleanDescription:          row.CleanDescription,
		Type:                      domain.TransactionType(row.Type),
		Status:                    domain.TransactionStatus(row.Status),
		Visibility:                domain.Visibility(row.Visibility),
		Source:                    domain.SourceType(row.Source),
		ExternalID:                extID,
		SequenceIndex:             int(row.SequenceIndex),
		Fingerprint:               row.Fingerprint,
		IsAICategorized:           row.IsAiCategorized,
		AIConfidence:              row.AiConfidence,
		AIModelVersion:            aiModel,
		IsManuallyVerified:        row.IsManuallyVerified,
		TransferPeerTransactionID: row.TransferPeerTransactionID,
		Notes:                     notes,
		Metadata:                  meta,
		CreatedAt:                 row.CreatedAt.Time,
		UpdatedAt:                 row.UpdatedAt.Time,
	}, nil
}


// SetBillID associates a transaction with a credit card invoice within a tenant boundary.
func (r *TransactionRepository) SetBillID(ctx context.Context, tenantID, id, billID uuid.UUID) error {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).SetBillID(ctx, tenantID, id, billID)
			})
		}
	}

	const querySQL = `
		UPDATE transactions
		SET bill_id = $3, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2;
	`
	cmd, err := r.getDB(ctx).Exec(ctx, querySQL, id, tenantID, billID)
	if err != nil {
		return fmt.Errorf("failed to set bill_id: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListByBillID retrieves all transactions associated with a credit card invoice within a tenant boundary.
func (r *TransactionRepository) ListByBillID(ctx context.Context, tenantID, billID uuid.UUID) ([]*domain.Transaction, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var result []*domain.Transaction
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var txErr error
				result, txErr = r.WithTx(tx).ListByBillID(ctx, tenantID, billID)
				return txErr
			})
			return result, err
		}
	}

	const querySQL = `
		SELECT id, tenant_id, account_id, category_id, created_by_user_id,
		       transacted_at, date, amount, description, clean_description,
		       type, status, visibility, source, external_id, sequence_index,
		       fingerprint, is_ai_categorized, ai_confidence, ai_model_version,
		       is_manually_verified, transfer_peer_transaction_id, bill_id,
		       notes, metadata, created_at, updated_at
		FROM transactions
		WHERE tenant_id = $1 AND bill_id = $2
		ORDER BY date DESC, transacted_at DESC;
	`
	rows, err := r.getDB(ctx).Query(ctx, querySQL, tenantID, billID)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions by bill_id: %w", err)
	}
	defer rows.Close()

	var result []*domain.Transaction
	for rows.Next() {
		var (
			t             domain.Transaction
			transactedAt  time.Time
			date          time.Time
			amount        decimal.Decimal
			txType        string
			txStatus      string
			visibility    string
			source        string
			extID         *string
			aiModel       *string
			notes         *string
			metadataBytes []byte
			createdAt     time.Time
			updatedAt     time.Time
		)

		err := rows.Scan(
			&t.ID,
			&t.TenantID,
			&t.AccountID,
			&t.CategoryID,
			&t.CreatedByUserID,
			&transactedAt,
			&date,
			&amount,
			&t.Description,
			&t.CleanDescription,
			&txType,
			&txStatus,
			&visibility,
			&source,
			&extID,
			&t.SequenceIndex,
			&t.Fingerprint,
			&t.IsAICategorized,
			&t.AIConfidence,
			&aiModel,
			&t.IsManuallyVerified,
			&t.TransferPeerTransactionID,
			&t.BillID,
			&notes,
			&metadataBytes,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		t.TransactedAt = transactedAt
		t.Date = date
		t.Amount = amount
		t.Type = domain.TransactionType(txType)
		t.Status = domain.TransactionStatus(txStatus)
		t.Visibility = domain.Visibility(visibility)
		t.Source = domain.SourceType(source)
		if extID != nil {
			t.ExternalID = *extID
		}
		if aiModel != nil {
			t.AIModelVersion = *aiModel
		}
		if notes != nil {
			t.Notes = *notes
		}
		t.CreatedAt = createdAt
		t.UpdatedAt = updatedAt

		t.Metadata = make(map[string]any)
		if len(metadataBytes) > 0 {
			_ = json.Unmarshal(metadataBytes, &t.Metadata)
		}

		result = append(result, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during transaction rows iteration: %w", err)
	}

	return result, nil
}

// ReconcileExternalTransaction updates bill_id and transitions PENDING/COMPLETED status
// for an existing external transaction within a tenant boundary.
func (r *TransactionRepository) ReconcileExternalTransaction(
	ctx context.Context,
	tenantID uuid.UUID,
	externalID string,
	billID *uuid.UUID,
	status domain.TransactionStatus,
) error {
	if strings.TrimSpace(externalID) == "" {
		return nil
	}
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).ReconcileExternalTransaction(ctx, tenantID, externalID, billID, status)
			})
		}
	}

	dbtx := r.getDB(ctx)
	if billID != nil && *billID != uuid.Nil {
		const billSQL = `UPDATE transactions SET bill_id = $1, updated_at = NOW() WHERE tenant_id = $2 AND external_id = $3;`
		if _, err := dbtx.Exec(ctx, billSQL, *billID, tenantID, externalID); err != nil {
			return fmt.Errorf("failed to reconcile external transaction bill_id: %w", err)
		}
	}

	switch status {
	case domain.TransactionStatusPending:
		const toPendingSQL = `
			UPDATE transactions
			SET status = 'PENDING',
			    metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{is_future}', 'true'::jsonb),
			    updated_at = NOW()
			WHERE tenant_id = $1 AND external_id = $2 AND status = 'COMPLETED';`
		if _, err := dbtx.Exec(ctx, toPendingSQL, tenantID, externalID); err != nil {
			return fmt.Errorf("failed to transition external transaction to PENDING: %w", err)
		}
	case domain.TransactionStatusCompleted:
		const toCompletedSQL = `
			UPDATE transactions
			SET status = 'COMPLETED',
			    metadata = COALESCE(metadata, '{}'::jsonb) - 'is_future',
			    updated_at = NOW()
			WHERE tenant_id = $1 AND external_id = $2 AND status = 'PENDING';`
		if _, err := dbtx.Exec(ctx, toCompletedSQL, tenantID, externalID); err != nil {
			return fmt.Errorf("failed to transition external transaction to COMPLETED: %w", err)
		}
	}

	return nil
}
