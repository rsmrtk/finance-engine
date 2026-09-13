-- +goose Up
-- Every LiqPay callback we receive, kept as an audit trail/receipt
-- history — never the card itself, LiqPay never sends us that. Only the
-- fields billing.Service already decodes from a callback.
CREATE TABLE payment_events (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    order_id           TEXT NOT NULL,
    plan               TEXT NOT NULL,
    action             TEXT NOT NULL DEFAULT '',
    status             TEXT NOT NULL,
    amount             NUMERIC(12, 2) NOT NULL DEFAULT 0,
    currency           TEXT NOT NULL DEFAULT '',
    error_description  TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_events_user_id ON payment_events (user_id, created_at DESC);

-- +goose Down
DROP TABLE payment_events;
