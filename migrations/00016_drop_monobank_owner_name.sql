-- +goose Up
-- Reverts migration 00015: matching "Від: <name>" against the account
-- owner's own name turned out to be the wrong call. This app only ever
-- tracks ONE Monobank card/token — money arriving from a FOP account or a
-- different bank under the same person's name is invisible on the other
-- side (no matching outflow anywhere in this app's data), so hiding it as
-- a "transfer" just made real, spendable money disappear from the
-- numbers instead of correctly counting it as income into the tracked
-- account. Internal-transfer detection stays for cases where BOTH legs
-- are genuinely visible here: jars/card-color transfers within the same
-- token (looksLikeInternalTransfer) and amount-matched pairs
-- (TransactionFindTransferMatch) — see internal/monobankimport.
ALTER TABLE monobank_connections
    DROP COLUMN owner_name;

-- +goose Down
ALTER TABLE monobank_connections
    ADD COLUMN owner_name TEXT NOT NULL DEFAULT '';
