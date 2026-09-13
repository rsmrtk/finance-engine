-- +goose Up
-- A personal token can cover several cards/jars, and the user should be
-- able to track more than one — one row per user still, but now with
-- arrays of account ids/masked pans instead of a single pair.
ALTER TABLE monobank_connections
    ADD COLUMN account_ids TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN masked_pans TEXT[] NOT NULL DEFAULT '{}';

UPDATE monobank_connections
SET account_ids = ARRAY[account_id], masked_pans = ARRAY[masked_pan]
WHERE account_id <> '';

ALTER TABLE monobank_connections
    DROP COLUMN account_id,
    DROP COLUMN masked_pan;

-- +goose Down
ALTER TABLE monobank_connections
    ADD COLUMN account_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN masked_pan TEXT NOT NULL DEFAULT '';

UPDATE monobank_connections
SET account_id = COALESCE(account_ids[1], ''), masked_pan = COALESCE(masked_pans[1], '');

ALTER TABLE monobank_connections
    DROP COLUMN account_ids,
    DROP COLUMN masked_pans;
