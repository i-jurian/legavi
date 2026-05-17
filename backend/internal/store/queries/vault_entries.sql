-- name: CreateVaultEntry :one
INSERT INTO vault_entries (
    id,
    user_id,
    preview,
    bundle,
    sort_order
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetVaultEntry :one
SELECT * FROM vault_entries
WHERE id = $1 AND user_id = $2;

-- name: ListUserVaultEntries :many
SELECT id, user_id, preview, sort_order, schema_version, created_at, updated_at, deleted_at
FROM vault_entries
WHERE user_id = sqlc.arg(user_id)
  AND (sqlc.arg(include_deleted)::boolean OR deleted_at IS NULL)
ORDER BY sort_order ASC, created_at ASC
LIMIT sqlc.arg(row_limit);

-- name: UpdateVaultEntry :one
UPDATE vault_entries
SET preview = $3,
    bundle = $4,
    sort_order = $5,
    updated_at = now()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteVaultEntry :one
UPDATE vault_entries
SET deleted_at = now(),
    updated_at = now()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: RestoreVaultEntry :one
UPDATE vault_entries
SET deleted_at = NULL,
    updated_at = now()
WHERE id = $1
  AND user_id = $2
  AND deleted_at IS NOT NULL
  AND deleted_at > now() - INTERVAL '30 days'
RETURNING *;

-- name: UpdateVaultEntrySortOrder :exec
UPDATE vault_entries
SET sort_order = $3
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;
