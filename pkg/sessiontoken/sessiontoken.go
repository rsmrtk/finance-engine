// Package sessiontoken generates opaque web refresh tokens and hashes them
// for storage. The raw token is high-entropy (32 random bytes) and only
// ever handed to the browser (as a cookie) — the database only ever sees
// Hash(raw), so a leaked database dump can't be replayed as a live session.
// A fast hash (SHA-256) is appropriate here, unlike passwords (pkg/passwordhash):
// the token is already unguessable, so we're hashing for at-rest storage,
// not for resistance against brute-forcing a low-entropy secret.
package sessiontoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// New returns a fresh raw token (to hand to the client) and its hash (to
// store in the database).
func New() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate random token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, Hash(raw), nil
}

// Hash deterministically hashes a raw token for lookup/storage.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
