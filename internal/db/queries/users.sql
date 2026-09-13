-- name: UserGetByAppleSub :one
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar
FROM users
WHERE apple_sub = $1;

-- name: UserGetByEmail :one
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar
FROM users
WHERE email = $1;

-- name: UserGetByGoogleSub :one
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar
FROM users
WHERE google_sub = $1;

-- name: UserGetByID :one
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar
FROM users
WHERE id = $1;

-- name: UserGetByLiqPayOrderID :one
-- Looks up the user a webhook callback belongs to — the order_id column
-- always holds the *current* subscription's order_id, so a stale/replayed
-- callback from a superseded order_id simply won't match anyone.
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar
FROM users
WHERE liqpay_order_id = $1;

-- name: UserCreate :one
INSERT INTO users (apple_sub, email, base_currency)
VALUES ($1, $2, $3)
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserCreateWithEmail :one
INSERT INTO users (email, password_hash, base_currency)
VALUES ($1, $2, $3)
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserCreateWithGoogle :one
INSERT INTO users (google_sub, email, base_currency)
VALUES ($1, $2, $3)
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserUpdatePreferences :one
UPDATE users SET theme = $2, gradient_color = $3
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserUpdateProfile :one
UPDATE users SET name = $2, avatar = $3
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserUpdateGoals :one
UPDATE users SET goals = $2
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserUpdateBaseCurrency :one
UPDATE users SET base_currency = $2
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserStartTrial :one
-- Records the order_id and the already-computed trial end date before the
-- checkout redirect even happens — the webhook that later confirms
-- "subscribed" just flips plan/status, it doesn't need to (re)compute
-- the trial end date itself.
UPDATE users SET liqpay_order_id = $2, subscription_status = 'pending', trial_ends_at = $3
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserUpdateSubscription :one
UPDATE users SET plan = $2, subscription_status = $3, trial_ends_at = $4
WHERE id = $1
RETURNING id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar;

-- name: UserListForAdmin :many
-- Powers the admin dashboard's user table: optional case-insensitive
-- email search, optional exact plan filter, newest first. total_count is
-- a window function so pagination doesn't need a second round-trip.
SELECT id, apple_sub, email, base_currency, password_hash, google_sub, theme, gradient_color, plan, goals, subscription_status, trial_ends_at, liqpay_order_id, created_at, name, avatar,
  count(*) OVER() AS total_count
FROM users
WHERE ($3::text = '' OR email ILIKE '%' || $3 || '%')
  AND ($4::text = '' OR plan = $4)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UserCountByPlan :many
SELECT plan, subscription_status, count(*) AS user_count
FROM users
GROUP BY plan, subscription_status;

-- name: UserSignupsByDay :many
-- Real signup counts per day for the last N days, zero-filled by
-- generate_series so the admin dashboard's trend chart doesn't have gaps
-- on days with no signups.
SELECT d.day::date AS day, count(u.id) AS signups
FROM generate_series(now() - ($1::int || ' days')::interval, now(), '1 day') AS d(day)
LEFT JOIN users u ON u.created_at::date = d.day::date
GROUP BY d.day
ORDER BY d.day;
