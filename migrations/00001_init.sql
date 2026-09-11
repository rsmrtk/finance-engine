-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    apple_sub      TEXT NOT NULL UNIQUE,
    email          TEXT,
    base_currency  TEXT NOT NULL DEFAULT 'UAH',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users (id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    icon_name   TEXT NOT NULL,
    color_hex   TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('expense', 'income')),
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_categories_user_id ON categories (user_id);

CREATE TABLE transactions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category_id  UUID REFERENCES categories (id) ON DELETE SET NULL,
    amount       NUMERIC(14, 2) NOT NULL,
    currency     TEXT NOT NULL,
    type         TEXT NOT NULL CHECK (type IN ('expense', 'income')),
    date         TIMESTAMPTZ NOT NULL,
    note         TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_user_id_date ON transactions (user_id, date DESC);

CREATE TABLE exchange_rates (
    currency     TEXT PRIMARY KEY,
    rate_to_uah  NUMERIC(14, 6) NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO categories (name, icon_name, color_hex, type, is_default) VALUES
    ('Продукти', 'cart.fill', 'FF9500', 'expense', true),
    ('Транспорт', 'car.fill', '007AFF', 'expense', true),
    ('Житло', 'house.fill', '5856D6', 'expense', true),
    ('Розваги', 'gamecontroller.fill', 'FF2D55', 'expense', true),
    ('Здоров''я', 'cross.case.fill', '34C759', 'expense', true),
    ('Одяг', 'tshirt.fill', 'AF52DE', 'expense', true),
    ('Освіта', 'book.fill', '5AC8FA', 'expense', true),
    ('Інше', 'ellipsis.circle.fill', '8E8E93', 'expense', true),
    ('Зарплата', 'banknote.fill', '34C759', 'income', true),
    ('Фріланс', 'laptopcomputer', '007AFF', 'income', true),
    ('Подарунки', 'gift.fill', 'FF2D55', 'income', true),
    ('Інвестиції', 'chart.line.uptrend.xyaxis', 'FF9500', 'income', true),
    ('Інше', 'ellipsis.circle.fill', '8E8E93', 'income', true);

-- +goose Down
DROP TABLE exchange_rates;
DROP TABLE transactions;
DROP TABLE categories;
DROP TABLE users;
