-- +goose Up
ALTER TABLE users ADD COLUMN subscription_status TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN trial_ends_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN liqpay_order_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN liqpay_order_id;
ALTER TABLE users DROP COLUMN trial_ends_at;
ALTER TABLE users DROP COLUMN subscription_status;
