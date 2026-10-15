-- name: CreateCategory :one
INSERT INTO categories (
    id,
    tenant_id,
    parent_id,
    name,
    type,
    color_hex,
    icon_name,
    is_system,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetCategoryByID :one
SELECT * FROM categories
WHERE id = $1 AND tenant_id = $2
LIMIT 1;

-- name: ListCategories :many
SELECT * FROM categories
WHERE tenant_id = $1
ORDER BY name ASC;

-- name: UpdateCategory :one
UPDATE categories
SET
    parent_id = $3,
    name = $4,
    type = $5,
    color_hex = $6,
    icon_name = $7,
    is_system = $8,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM categories
WHERE id = $1 AND tenant_id = $2;

-- name: CountTransactionsByCategory :one
SELECT COUNT(*) FROM transactions
WHERE category_id = $1 AND tenant_id = $2;
