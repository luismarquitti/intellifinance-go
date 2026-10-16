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

-- name: CountTransactions :one
SELECT COUNT(*) FROM transactions
WHERE tenant_id = $1;

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
