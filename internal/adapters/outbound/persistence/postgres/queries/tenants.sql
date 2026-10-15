-- name: CreateTenant :one
INSERT INTO tenants (
    id,
    name,
    type,
    currency,
    settings,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetTenantByID :one
SELECT * FROM tenants
WHERE id = $1 LIMIT 1;

-- name: ListTenants :many
SELECT * FROM tenants
ORDER BY name ASC;

-- name: UpdateTenant :one
UPDATE tenants
SET
    name = $2,
    type = $3,
    currency = $4,
    settings = $5,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteTenant :exec
DELETE FROM tenants
WHERE id = $1;

-- name: AddTenantMember :one
INSERT INTO tenant_members (
    id,
    tenant_id,
    user_id,
    role,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetTenantMembers :many
SELECT * FROM tenant_members
WHERE tenant_id = $1
ORDER BY created_at ASC;

-- name: RemoveTenantMember :exec
DELETE FROM tenant_members
WHERE tenant_id = $1 AND user_id = $2;
