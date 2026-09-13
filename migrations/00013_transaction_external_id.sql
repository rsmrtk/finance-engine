-- +goose Up
-- Lets a Monobank import (webhook or manual "sync now" backfill) run
-- twice over the same time range without creating duplicate
-- transactions — external_id is Monobank's own statementItem.id.
-- Empty for manually-entered transactions, which never need dedup.
ALTER TABLE transactions
    ADD COLUMN external_id TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX idx_transactions_user_external_id
    ON transactions (user_id, external_id)
    WHERE external_id <> '';

-- +goose Down
DROP INDEX idx_transactions_user_external_id;
ALTER TABLE transactions DROP COLUMN external_id;
