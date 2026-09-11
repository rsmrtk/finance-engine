// Package cryptobox provides AES-256-GCM encryption for small secrets (like
// a user's Monobank personal token) before they're stored at rest.
package cryptobox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// Box encrypts/decrypts with a key derived from a passphrase. Deriving via
// SHA-256 means any non-empty string works as MONOBANK_TOKEN_KEY, the same
// ergonomics as JWT_SECRET, while still yielding a valid 32-byte AES-256 key.
type Box struct {
	gcm cipher.AEAD
}

func New(passphrase string) (*Box, error) {
	if passphrase == "" {
		return nil, fmt.Errorf("passphrase must not be empty")
	}
	key := sha256.Sum256([]byte(passphrase))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return &Box{gcm: gcm}, nil
}

// Encrypt returns nonce||ciphertext, safe to store as a single blob.
func (b *Box) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return b.gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (b *Box) Decrypt(blob []byte) (string, error) {
	nonceSize := b.gcm.NonceSize()
	if len(blob) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	plaintext, err := b.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}
