-- +goose Up
-- Persists Monobank's own operationAmount/operationCurrencyCode fields on
-- the transaction row itself, alongside amount/currency (see pkg/monobank
-- StatementItem). Previously these were only ever written to the pod's
-- stdout log — which this local kind cluster doesn't persist, so the
-- exact raw values behind an already-imported transaction became
-- unrecoverable the moment the pod that processed it got replaced by the
-- next redeploy (which is routine, not rare). Storing them durably means
-- any future "what did Monobank actually send for this one" question can
-- be answered by querying the transaction directly, no live token or
-- surviving pod required. Diagnostic only — 0 for manually-entered
-- transactions and never read by any business logic.
ALTER TABLE transactions
    ADD COLUMN operation_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN operation_currency_code INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE transactions
    DROP COLUMN operation_amount,
    DROP COLUMN operation_currency_code;
