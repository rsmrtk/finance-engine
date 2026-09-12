-- +goose Up

-- Theme + accent (gradient) color, tied to the account so they follow the
-- user across devices/browsers instead of living only in one browser's
-- localStorage.
ALTER TABLE users ADD COLUMN theme TEXT NOT NULL DEFAULT 'system';
ALTER TABLE users ADD COLUMN gradient_color TEXT NOT NULL DEFAULT '#34C759';

-- +goose Down
ALTER TABLE users DROP COLUMN gradient_color;
ALTER TABLE users DROP COLUMN theme;
