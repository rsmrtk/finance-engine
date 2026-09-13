-- name: MonobankConnectionUpsert :one
INSERT INTO monobank_connections (user_id, encrypted_token, webhook_secret, masked_pans, account_ids)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE
SET encrypted_token = EXCLUDED.encrypted_token,
    webhook_secret  = EXCLUDED.webhook_secret,
    masked_pans     = EXCLUDED.masked_pans,
    account_ids     = EXCLUDED.account_ids,
    connected_at    = now(),
    last_synced_at  = NULL
RETURNING user_id, encrypted_token, webhook_secret, masked_pans, account_ids, connected_at, last_synced_at;

-- name: MonobankConnectionGetByUserID :one
SELECT user_id, encrypted_token, webhook_secret, masked_pans, account_ids, connected_at, last_synced_at
FROM monobank_connections
WHERE user_id = $1;

-- name: MonobankConnectionGetByWebhookSecret :one
SELECT user_id, encrypted_token, webhook_secret, masked_pans, account_ids, connected_at, last_synced_at
FROM monobank_connections
WHERE webhook_secret = $1;

-- name: MonobankConnectionUpdateAccounts :one
UPDATE monobank_connections
SET account_ids = $2, masked_pans = $3
WHERE user_id = $1
RETURNING user_id, encrypted_token, webhook_secret, masked_pans, account_ids, connected_at, last_synced_at;

-- name: MonobankConnectionDelete :exec
DELETE FROM monobank_connections
WHERE user_id = $1;

-- name: MonobankConnectionTouchSync :exec
UPDATE monobank_connections
SET last_synced_at = now()
WHERE user_id = $1;
