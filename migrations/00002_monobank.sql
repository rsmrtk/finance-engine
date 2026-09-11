-- +goose Up
CREATE TABLE monobank_connections (
    user_id         UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    encrypted_token BYTEA NOT NULL,
    webhook_secret  TEXT NOT NULL UNIQUE,
    masked_pan      TEXT NOT NULL DEFAULT '',
    account_id      TEXT NOT NULL DEFAULT '',
    connected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_synced_at  TIMESTAMPTZ
);

CREATE INDEX idx_monobank_connections_webhook_secret ON monobank_connections (webhook_secret);

-- +goose Down
DROP TABLE monobank_connections;
