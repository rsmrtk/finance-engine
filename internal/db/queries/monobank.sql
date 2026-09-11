-- name: MonobankConnectionUpsert :one
INSERT INTO monobank_connections (user_id, encrypted_token, webhook_secret, masked_pan, account_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE
SET encrypted_token = EXCLUDED.encrypted_token,
    webhook_secret  = EXCLUDED.webhook_secret,
    masked_pan      = EXCLUDED.masked_pan,
    account_id      = EXCLUDED.account_id,
    connected_at    = now(),
    last_synced_at  = NULL
RETURNING user_id, encrypted_token, webhook_secret, masked_pan, account_id, connected_at, last_synced_at;

-- name: MonobankConnectionGetByUserID :one
SELECT user_id, encrypted_token, webhook_secret, masked_pan, account_id, connected_at, last_synced_at
FROM monobank_connections
WHERE user_id = $1;

-- name: MonobankConnectionGetByWebhookSecret :one
SELECT user_id, encrypted_token, webhook_secret, masked_pan, account_id, connected_at, last_synced_at
FROM monobank_connections
WHERE webhook_secret = $1;

-- name: MonobankConnectionDelete :exec
DELETE FROM monobank_connections
WHERE user_id = $1;

-- name: MonobankConnectionTouchSync :exec
UPDATE monobank_connections
SET last_synced_at = now()
WHERE user_id = $1;
