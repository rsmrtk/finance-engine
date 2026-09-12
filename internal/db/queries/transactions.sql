-- name: TransactionListForUser :many
SELECT id, user_id, category_id, amount, currency, type, date, note, created_at
FROM transactions
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR date >= $2)
  AND ($3::timestamptz IS NULL OR date <= $3)
ORDER BY date DESC;

-- name: TransactionCreate :one
INSERT INTO transactions (user_id, category_id, amount, currency, type, date, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, category_id, amount, currency, type, date, note, created_at;

-- name: TransactionUpdateForUser :one
UPDATE transactions
SET category_id = $3, amount = $4, currency = $5, type = $6, date = $7, note = $8
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, category_id, amount, currency, type, date, note, created_at;

-- name: TransactionDeleteForUser :execrows
DELETE FROM transactions
WHERE id = $1 AND user_id = $2;
