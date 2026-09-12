// Package passwordhash hashes and verifies user passwords with bcrypt.
// Unlike pkg/cryptobox (reversible AES-GCM, used where we must read the
// value back — e.g. Monobank tokens), a password must never be recoverable,
// so this uses a one-way primitive instead.
package passwordhash

import "golang.org/x/crypto/bcrypt"

// Hash returns a bcrypt hash safe to store in the database.
func Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Verify reports whether password matches the given bcrypt hash.
func Verify(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
