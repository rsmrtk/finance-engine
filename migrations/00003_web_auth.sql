-- +goose Up

-- Web login methods (email+password, Google) alongside the existing
-- Sign in with Apple. Apple was the only method until now, hence NOT NULL;
-- web signups won't have one.
ALTER TABLE users ALTER COLUMN apple_sub DROP NOT NULL;
ALTER TABLE users ADD COLUMN password_hash TEXT;
ALTER TABLE users ADD COLUMN google_sub TEXT;

-- Partial unique indexes (not plain UNIQUE columns) so multiple NULLs are
-- allowed — most users will have exactly one of apple_sub/google_sub/
-- password_hash, never all three.
CREATE UNIQUE INDEX users_google_sub_key ON users (google_sub) WHERE google_sub IS NOT NULL;
CREATE UNIQUE INDEX users_email_key ON users (email) WHERE email IS NOT NULL AND email <> '';

-- Web sessions: the access token stays a short-lived stateless JWT (same
-- pkg/jwt as iOS), but the web additionally gets a real, revocable
-- refresh token stored here (hashed, never the raw value) so a browser
-- session can be refreshed without re-entering credentials, and can be
-- killed server-side (unlike the iOS/Apple JWT, which has no revocation
-- path at all today).
CREATE TABLE sessions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    refresh_hash TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);

-- +goose Down
DROP TABLE sessions;
DROP INDEX users_email_key;
DROP INDEX users_google_sub_key;
ALTER TABLE users DROP COLUMN google_sub;
ALTER TABLE users DROP COLUMN password_hash;
ALTER TABLE users ALTER COLUMN apple_sub SET NOT NULL;
