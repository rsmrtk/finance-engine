-- +goose Up
-- Parallel to account_ids/masked_pans (same index = same account): the
-- Monobank "type" (white/black/platinum/fop/...) and currency of each
-- tracked account. Lets the importer check "is the card this description
-- names actually one I track" before treating a card-color mention
-- ("На платинову картку") as proof of an internal transfer — without
-- this, a card the user never selected to track could still get text-
-- matched and silently erase real money on the other side.
ALTER TABLE monobank_connections
    ADD COLUMN account_types TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN account_currencies TEXT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE monobank_connections
    DROP COLUMN account_types,
    DROP COLUMN account_currencies;
