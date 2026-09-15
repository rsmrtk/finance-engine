-- +goose Up
-- Tracks when a transaction row was last edited (e.g. via the new
-- currency picker in the edit modal), separate from `date` (the
-- transaction's own date, which drives list ordering) and `created_at`
-- (when it was first imported/entered) — editing a transaction must
-- never change its position in the list, only this new column.
ALTER TABLE transactions
    ADD COLUMN updated_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE transactions
    DROP COLUMN updated_at;
