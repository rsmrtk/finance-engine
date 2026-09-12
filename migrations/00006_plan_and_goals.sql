-- +goose Up

-- `plan` backs the pricing tiers shown in the web UI (free/pro/max/enterprise).
-- No billing integration exists yet — this column exists so the account
-- has somewhere to live once one does; everyone defaults to 'free'.
ALTER TABLE users ADD COLUMN plan TEXT NOT NULL DEFAULT 'free';

-- Free-form financial goals/plans the user writes for themselves — a
-- single text field, not a structured goals table, since nothing so far
-- asked for tracked progress/deadlines, just a place to write them down.
ALTER TABLE users ADD COLUMN goals TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN goals;
ALTER TABLE users DROP COLUMN plan;
