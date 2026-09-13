-- name: TransactionListForUser :many
SELECT id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer
FROM transactions
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR date >= $2)
  AND ($3::timestamptz IS NULL OR date <= $3)
ORDER BY date DESC;

-- name: TransactionCreate :one
INSERT INTO transactions (user_id, category_id, amount, currency, type, date, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer;

-- name: TransactionCreateWithExternalID :one
-- Used by Monobank imports (webhook + manual "sync now" backfill) —
-- ON CONFLICT DO NOTHING makes re-importing the same statement item a
-- harmless no-op instead of a duplicate row. No rows returned means it
-- already existed; the caller treats pgx.ErrNoRows as "skipped", not a
-- failure. is_internal_transfer is set at creation only when the
-- description itself gives it away (e.g. a jar top-up) — the other
-- detection path (matching against a same-amount opposite-type
-- transaction on a different tracked account) runs after creation, see
-- TransactionFindTransferMatch + TransactionMarkInternalTransfer.
INSERT INTO transactions (user_id, category_id, amount, currency, type, date, note, external_id, is_internal_transfer)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id, external_id) WHERE external_id <> '' DO NOTHING
RETURNING id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer;

-- name: TransactionUpdateForUser :one
UPDATE transactions
SET category_id = $3, amount = $4, currency = $5, type = $6, date = $7, note = $8
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer;

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

-- name: TransactionFindTransferMatch :one
-- Looks for the other side of an internal transfer: same user, opposite
-- type, exact same amount, another Monobank-imported transaction
-- (external_id set) not already flagged, within the time window the
-- caller passes in. Deliberately NOT filtered by currency: Monobank has
-- been observed reporting the two legs of the same real transfer with
-- mismatched currency labels (e.g. one side EUR, the other UAH, same
-- exact decimal amount — a real cross-currency conversion would never
-- produce an identical number on both sides at anything but a 1:1 rate),
-- so requiring a currency match was silently failing to catch exactly
-- the cases this exists for. A same-amount, opposite-type,
-- seconds-apart coincidence between two unrelated transactions is
-- vanishingly unlikely regardless.
SELECT id, user_id, category_id, amount, currency, type, date, note, created_at, external_id, is_internal_transfer
FROM transactions
WHERE user_id = $1
  AND type = $2
  AND amount = $3
  AND external_id <> ''
  AND is_internal_transfer = false
  AND date BETWEEN $4 AND $5
ORDER BY date DESC
LIMIT 1;

-- name: TransactionMarkInternalTransfer :exec
UPDATE transactions
SET is_internal_transfer = true
WHERE id = $1;
