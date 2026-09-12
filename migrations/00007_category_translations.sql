-- +goose Up

-- `name` stays the untouched source of truth (what the user typed, used
-- for editing/search/matching — no change to any existing logic). These
-- two columns are purely additive display translations, auto-populated
-- by the backend (pkg/translate) and safe to be NULL: callers fall back
-- to `name` whenever a translation hasn't been generated yet.
ALTER TABLE categories ADD COLUMN name_uk TEXT;
ALTER TABLE categories ADD COLUMN name_en TEXT;

-- +goose Down
ALTER TABLE categories DROP COLUMN name_en;
ALTER TABLE categories DROP COLUMN name_uk;
