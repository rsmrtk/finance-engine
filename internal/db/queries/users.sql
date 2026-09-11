-- name: UserGetByAppleSub :one
SELECT id, apple_sub, email, base_currency, created_at
FROM users
WHERE apple_sub = $1;

-- name: UserCreate :one
INSERT INTO users (apple_sub, email, base_currency)
VALUES ($1, $2, $3)
RETURNING id, apple_sub, email, base_currency, created_at;

-- name: UserGetByID :one
SELECT id, apple_sub, email, base_currency, created_at
FROM users
WHERE id = $1;
