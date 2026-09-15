-- name: TransactionListForUser :many
SELECT id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer, operation_amount, operation_currency_code, updated_at
FROM transactions
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR date >= $2)
  AND ($3::timestamptz IS NULL OR date <= $3)
ORDER BY date DESC;

-- name: TransactionCreate :one
INSERT INTO transactions (user_id, category_id, amount, currency, type, date, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer, operation_amount, operation_currency_code, updated_at;

-- name: TransactionCreateWithExternalID :one
-- Used by Monobank imports (webhook + manual "sync now" backfill) —
-- ON CONFLICT DO NOTHING makes re-importing the same statement item a
-- harmless no-op instead of a duplicate row. No rows returned means it
-- already existed; the caller treats pgx.ErrNoRows as "skipped", not a
-- failure. is_internal_transfer always starts false — the account owner
-- flags a transfer themselves afterward (see TransactionUpdateForUser),
-- automatic detection kept getting confidently wrong. operation_amount/
-- operation_currency_code are Monobank's own raw fields (see
-- pkg/monobank.StatementItem) — stored durably so a future "what did
-- Monobank actually send for this one" question can be answered by
-- querying the row directly, without needing a still-live pod's log
-- (which a routine redeploy erases) or the user's Monobank token.
INSERT INTO transactions (user_id, category_id, amount, currency, type, date, note, external_id, operation_amount, operation_currency_code)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (user_id, external_id) WHERE external_id <> '' DO NOTHING
RETURNING id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer, operation_amount, operation_currency_code, updated_at;

-- name: TransactionUpdateForUser :one
-- updated_at is set here only — never touched by import/create, and
-- never used for ORDER BY anywhere (TransactionListForUser sorts by
-- `date`, the transaction's own date) — so editing a transaction (e.g.
-- fixing its currency) records when the edit happened without ever
-- moving the row in the list. is_internal_transfer is user-editable here
-- — the account owner's own manual call on whether a transaction is a
-- transfer, not a guess this app makes for them.
UPDATE transactions
SET category_id = $3, amount = $4, currency = $5, type = $6, date = $7, note = $8, is_internal_transfer = $9, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer, operation_amount, operation_currency_code, updated_at;

-- name: TransactionDeleteForUser :execrows
DELETE FROM transactions
WHERE id = $1 AND user_id = $2;

-- name: TransactionCountForUser :one
SELECT count(*) FROM transactions WHERE user_id = $1;

-- name: TransactionDeleteExpiredForPlan :execrows
-- Data-retention cleanup: Free keeps 30 days, Pro keeps 365 — see
-- internal/plan and internal/retention. Max/Enterprise never call this.
DELETE FROM transactions
WHERE date < $1
  AND user_id IN (SELECT id FROM users WHERE plan = $2);
