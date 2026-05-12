-- name: CreateSession :one
INSERT INTO webauthn_sessions (id, user_id, session_data, purpose, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ConsumeSession :one
UPDATE webauthn_sessions
SET consumed_at = now()
WHERE id = $1
  AND purpose = $2
  AND consumed_at IS NULL
  AND expires_at > now()
RETURNING *;
