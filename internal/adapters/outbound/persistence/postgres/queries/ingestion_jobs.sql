-- name: CreateIngestionJob :one
INSERT INTO ingestion_jobs (
    id,
    tenant_id,
    account_id,
    initiated_by_user_id,
    source,
    status,
    source_filename,
    file_storage_path,
    file_mime_type,
    file_size_bytes,
    total_records,
    inserted_records,
    duplicate_records,
    failed_records,
    error_summary,
    error_details,
    created_at,
    completed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
) RETURNING *;

-- name: GetIngestionJobByID :one
SELECT * FROM ingestion_jobs
WHERE id = $1 AND tenant_id = $2
LIMIT 1;

-- name: ListIngestionJobs :many
SELECT * FROM ingestion_jobs
WHERE tenant_id = $1
ORDER BY created_at DESC;

-- name: UpdateIngestionJob :one
UPDATE ingestion_jobs
SET
    status = $3,
    total_records = $4,
    inserted_records = $5,
    duplicate_records = $6,
    failed_records = $7,
    error_summary = $8,
    error_details = $9,
    completed_at = $10
WHERE id = $1 AND tenant_id = $2
RETURNING *;
