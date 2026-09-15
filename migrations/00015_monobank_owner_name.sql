-- +goose Up
-- Monobank's ClientInfo.name is the account holder's real name — stored so
-- the importer can recognize "Від: <name>" / "Переказ від <name>" style
-- descriptions as a self-transfer (the sender IS the account owner, e.g.
-- money moved in from a different bank or the same person's FOP account)
-- instead of only catching card-color-named transfers between Monobank's
-- own cards. Never re-fetched aggressively: ClientInfo is rate-limited to
-- ~1 request/60s per token, so this is only refreshed opportunistically
-- when a call already happening for another reason (Connect, the "Edit
-- accounts" flow) has the name in hand anyway.
ALTER TABLE monobank_connections
    ADD COLUMN owner_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE monobank_connections
    DROP COLUMN owner_name;
