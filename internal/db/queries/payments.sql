-- name: PaymentEventCreate :one
INSERT INTO payment_events (user_id, order_id, plan, action, status, amount, currency, error_description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, user_id, order_id, plan, action, status, amount, currency, error_description, created_at;

-- name: PaymentEventListForUser :many
SELECT id, user_id, order_id, plan, action, status, amount, currency, error_description, created_at
FROM payment_events
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;
