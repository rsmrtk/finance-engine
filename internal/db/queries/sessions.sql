-- name: SessionCreate :one
INSERT INTO sessions (user_id, refresh_hash, expires_at, user_agent)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, refresh_hash, created_at, expires_at, revoked_at, user_agent;

-- name: SessionGetByRefreshHash :one
SELECT id, user_id, refresh_hash, created_at, expires_at, revoked_at, user_agent
FROM sessions
WHERE refresh_hash = $1;

-- name: SessionRevokeByID :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1;

-- name: SessionRevokeByRefreshHash :exec
UPDATE sessions SET revoked_at = now() WHERE refresh_hash = $1;

-- name: SessionListActiveForUser :many
SELECT id, user_id, refresh_hash, created_at, expires_at, revoked_at, user_agent
FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: SessionRevokeForUser :execrows
UPDATE sessions SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;
