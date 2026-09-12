-- name: UserGetByAppleSub :one
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at
FROM users
WHERE apple_sub = $1;

-- name: UserGetByEmail :one
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at
FROM users
WHERE email = $1;

-- name: UserGetByGoogleSub :one
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at
FROM users
WHERE google_sub = $1;

-- name: UserGetByID :one
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at
FROM users
WHERE id = $1;

-- name: UserCreate :one
INSERT INTO users (apple_sub, email, base_currency)
VALUES ($1, $2, $3)
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at;

-- name: UserCreateWithEmail :one
INSERT INTO users (email, password_hash, base_currency)
VALUES ($1, $2, $3)
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at;

-- name: UserCreateWithGoogle :one
INSERT INTO users (google_sub, email, base_currency)
VALUES ($1, $2, $3)
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at;

-- name: UserUpdatePreferences :one
UPDATE users SET theme = $2, gradient_color = $3
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at;

-- name: UserUpdateGoals :one
UPDATE users SET goals = $2
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at;

-- name: UserUpdateBaseCurrency :one
UPDATE users SET base_currency = $2
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, created_at;
