-- name: CreateAccount :one
INSERT INTO accounts (
    id,
    tenant_id,
    owner_user_id,
    name,
    type,
    currency,
    balance,
    visibility,
    institution_id,
    institution_name,
    account_number_masked,
    color_hex,
    icon_name,
    is_active,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
) RETURNING *;

-- name: GetAccountByID :one
SELECT * FROM accounts
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: ListAccounts :many
SELECT * FROM accounts
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY name ASC;

-- name: ListAccountsFiltered :many
SELECT * FROM accounts
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('owner_user_id')::uuid IS NULL OR owner_user_id = sqlc.narg('owner_user_id'))
  AND (sqlc.narg('type')::account_type IS NULL OR type = sqlc.narg('type')::account_type)
  AND (sqlc.narg('visibility')::visibility_type IS NULL OR visibility = sqlc.narg('visibility')::visibility_type)
  AND (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active'))
ORDER BY name ASC;

-- name: UpdateAccount :one
UPDATE accounts
SET
    name = $3,
    type = $4,
    visibility = $5,
    institution_id = $6,
    institution_name = $7,
    account_number_masked = $8,
    color_hex = $9,
    icon_name = $10,
    is_active = $11,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteAccount :exec
UPDATE accounts
SET
    deleted_at = NOW(),
    is_active = FALSE,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: DeleteAccount :exec
DELETE FROM accounts
WHERE id = $1 AND tenant_id = $2;

-- name: UpdateAccountBalance :one
UPDATE accounts
SET
    balance = balance + $3,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
RETURNING *;
