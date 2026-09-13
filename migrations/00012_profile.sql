-- +goose Up
-- Display name + avatar for the profile page — replacing "your email and
-- an initial-letter circle" as the account's identity. Avatar is a small
-- data: URI (client resizes/compresses before upload), stored as plain
-- TEXT — no object storage needed for something this small, and it keeps
-- the whole stack free to run.
ALTER TABLE users
    ADD COLUMN name TEXT NOT NULL DEFAULT '',
    ADD COLUMN avatar TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users
    DROP COLUMN name,
    DROP COLUMN avatar;
