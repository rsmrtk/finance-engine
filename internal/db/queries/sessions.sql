-- name: SessionCreate :one
INSERT INTO sessions (user_id, refresh_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, refresh_hash, created_at, expires_at, revoked_at;

-- name: SessionGetByRefreshHash :one
SELECT id, user_id, refresh_hash, created_at, expires_at, revoked_at
FROM sessions
WHERE refresh_hash = $1;

-- name: SessionRevokeByID :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1;

-- name: SessionRevokeByRefreshHash :exec
UPDATE sessions SET revoked_at = now() WHERE refresh_hash = $1;
