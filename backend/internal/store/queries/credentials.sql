-- name: CreateCredential :one
INSERT INTO credentials (
    id,
    user_id,
    public_key,
    sign_count,
    transports,
    aaguid,
    age_recipient,
    nickname
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: CountUserCredentials :one
SELECT count(*) FROM credentials
WHERE user_id = $1 AND deleted_at IS NULL;

-- name: ListUserCredentials :many
SELECT * FROM credentials
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: UpdateCredentialUsage :exec
UPDATE credentials
SET sign_count = $2,
    last_used_at = now()
WHERE id = $1 AND deleted_at IS NULL;
