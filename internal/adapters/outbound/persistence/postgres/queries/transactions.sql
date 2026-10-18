-- name: CreateTransaction :one
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
) ON CONFLICT (tenant_id, fingerprint) DO NOTHING
RETURNING *;

-- name: UpsertTransaction :exec
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
) ON CONFLICT (tenant_id, fingerprint) DO NOTHING;

-- name: GetTransactionByID :one
SELECT * FROM transactions
WHERE id = $1 AND tenant_id = $2
LIMIT 1;

-- name: ListTransactions :many
SELECT * FROM transactions
WHERE tenant_id = $1
ORDER BY date DESC, transacted_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTransactionsFiltered :many
SELECT * FROM transactions
WHERE tenant_id = $1
  AND (sqlc.narg('account_id')::uuid IS NULL OR account_id = sqlc.narg('account_id'))
  AND (sqlc.narg('category_id')::uuid IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('is_uncategorized')::boolean IS NULL OR (sqlc.narg('is_uncategorized')::boolean = TRUE AND category_id IS NULL))
  AND (sqlc.narg('type')::transaction_type IS NULL OR type = sqlc.narg('type')::transaction_type)
  AND (sqlc.narg('from_date')::date IS NULL OR date >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::date IS NULL OR date <= sqlc.narg('to_date'))
  AND (sqlc.narg('search')::text IS NULL OR description ILIKE '%' || sqlc.narg('search') || '%' OR notes ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY
  CASE WHEN sqlc.arg('sort_by')::text = 'date' AND sqlc.arg('sort_order')::text = 'asc' THEN date END ASC,
  CASE WHEN (sqlc.arg('sort_by')::text = 'date' OR sqlc.arg('sort_by')::text = '') AND (sqlc.arg('sort_order')::text = 'desc' OR sqlc.arg('sort_order')::text = '') THEN date END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'amount' AND sqlc.arg('sort_order')::text = 'asc' THEN amount END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'amount' AND sqlc.arg('sort_order')::text = 'desc' THEN amount END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'description' AND sqlc.arg('sort_order')::text = 'asc' THEN description END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'description' AND sqlc.arg('sort_order')::text = 'desc' THEN description END DESC,
  created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountTransactions :one
SELECT COUNT(*) FROM transactions
WHERE tenant_id = $1;

-- name: CountTransactionsFiltered :one
SELECT COUNT(*)::bigint FROM transactions
WHERE tenant_id = $1
  AND (sqlc.narg('account_id')::uuid IS NULL OR account_id = sqlc.narg('account_id'))
  AND (sqlc.narg('category_id')::uuid IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('is_uncategorized')::boolean IS NULL OR (sqlc.narg('is_uncategorized')::boolean = TRUE AND category_id IS NULL))
  AND (sqlc.narg('type')::transaction_type IS NULL OR type = sqlc.narg('type')::transaction_type)
  AND (sqlc.narg('from_date')::date IS NULL OR date >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::date IS NULL OR date <= sqlc.narg('to_date'))
  AND (sqlc.narg('search')::text IS NULL OR description ILIKE '%' || sqlc.narg('search') || '%' OR notes ILIKE '%' || sqlc.narg('search') || '%');

-- name: SummarizeTransactions :one
SELECT
    COALESCE(SUM(CASE WHEN type = 'INCOME' THEN amount ELSE 0 END), 0)::numeric(15,2) AS total_income,
    COALESCE(SUM(CASE WHEN type = 'EXPENSE' THEN ABS(amount) ELSE 0 END), 0)::numeric(15,2) AS total_expense,
    COALESCE(SUM(CASE WHEN type = 'INCOME' THEN amount WHEN type = 'EXPENSE' THEN amount ELSE 0 END), 0)::numeric(15,2) AS net_balance,
    COUNT(*)::bigint AS count
FROM transactions
WHERE tenant_id = $1
  AND (sqlc.narg('account_id')::uuid IS NULL OR account_id = sqlc.narg('account_id'))
  AND (sqlc.narg('category_id')::uuid IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('is_uncategorized')::boolean IS NULL OR (sqlc.narg('is_uncategorized')::boolean = TRUE AND category_id IS NULL))
  AND (sqlc.narg('type')::transaction_type IS NULL OR type = sqlc.narg('type')::transaction_type)
  AND (sqlc.narg('from_date')::date IS NULL OR date >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::date IS NULL OR date <= sqlc.narg('to_date'))
  AND (sqlc.narg('search')::text IS NULL OR description ILIKE '%' || sqlc.narg('search') || '%' OR notes ILIKE '%' || sqlc.narg('search') || '%');

-- name: UpdateTransaction :one
UPDATE transactions
SET
    account_id = $3,
    category_id = $4,
    transacted_at = $5,
    date = $6,
    amount = $7,
    description = $8,
    clean_description = $9,
    type = $10,
    status = $11,
    visibility = $12,
    source = $13,
    external_id = $14,
    sequence_index = $15,
    is_ai_categorized = $16,
    ai_confidence = $17,
    ai_model_version = $18,
    is_manually_verified = $19,
    transfer_peer_transaction_id = $20,
    notes = $21,
    metadata = $22,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: DeleteTransaction :exec
DELETE FROM transactions
WHERE id = $1 AND tenant_id = $2;

-- name: BatchDeleteTransactions :exec
DELETE FROM transactions
WHERE tenant_id = $1 AND id = ANY(@ids::uuid[]);

-- name: BatchUpdateTransactionCategory :exec
UPDATE transactions
SET category_id = sqlc.narg('category_id')::uuid, updated_at = NOW()
WHERE tenant_id = $1 AND id = ANY(@ids::uuid[]);

-- name: FindExistingFingerprints :many
SELECT fingerprint FROM transactions
WHERE tenant_id = $1 AND fingerprint = ANY(@fingerprints::text[]);

-- name: CreateTransactionSplit :one
INSERT INTO transaction_splits (
    id,
    tenant_id,
    transaction_id,
    user_id,
    amount,
    percentage,
    notes,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetTransactionSplits :many
SELECT * FROM transaction_splits
WHERE tenant_id = $1 AND transaction_id = $2
ORDER BY created_at ASC;

-- name: DeleteTransactionSplits :exec
DELETE FROM transaction_splits
WHERE tenant_id = $1 AND transaction_id = $2;
