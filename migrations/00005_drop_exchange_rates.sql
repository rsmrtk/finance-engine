-- +goose Up

-- Exchange rates are pure reference/statistics data (global, refreshed
-- from NBU on a timer, never written by a user) — not "user data" in the
-- sense everything else in this schema is. Moving it to Redis
-- (internal/repository/rate.go) keeps Postgres scoped to actual user data
-- (accounts, categories, transactions, sessions) and gives rates a natural
-- TTL/cache shape instead of a relational table nobody joins against.
DROP TABLE exchange_rates;

-- +goose Down
CREATE TABLE exchange_rates (
    currency     TEXT PRIMARY KEY,
    rate_to_uah  NUMERIC(14, 6) NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
