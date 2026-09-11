-- name: RateList :many
SELECT currency, rate_to_uah, updated_at
FROM exchange_rates
ORDER BY currency;

-- name: RateUpsert :exec
INSERT INTO exchange_rates (currency, rate_to_uah, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (currency) DO UPDATE
SET rate_to_uah = EXCLUDED.rate_to_uah, updated_at = now();
