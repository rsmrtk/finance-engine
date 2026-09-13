-- +goose Up
-- Money moving between the user's own Monobank cards/jars (white <->
-- platinum <-> FOP <-> a jar) isn't real income or a real expense — it's
-- the same money changing pockets. Detected transactions get flagged
-- here instead of silently counted as both an expense and an income.
ALTER TABLE transactions
    ADD COLUMN is_internal_transfer BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX idx_transactions_transfer_match
    ON transactions (user_id, type, amount, currency, date)
    WHERE external_id <> '' AND is_internal_transfer = false;

-- +goose Down
DROP INDEX idx_transactions_transfer_match;
ALTER TABLE transactions DROP COLUMN is_internal_transfer;
