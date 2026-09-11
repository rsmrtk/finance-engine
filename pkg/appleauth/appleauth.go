// Package appleauth verifies "Sign in with Apple" identity tokens against
// Apple's published JWKS, without any third-party JWKS dependency.
package appleauth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwksURL = "https://appleid.apple.com/auth/keys"
	issuer  = "https://appleid.apple.com"
)

type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// Verifier validates Apple identity tokens for one specific audience
// (your app's bundle id / Services ID).
type Verifier struct {
	audience string

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func NewVerifier(audience string) *Verifier {
	return &Verifier{audience: audience, keys: map[string]*rsa.PublicKey{}}
}

// Verify parses and validates the identity token and returns Apple's
// stable, per-user subject id and the email claim (if present).
func (v *Verifier) Verify(identityToken string) (sub string, email string, err error) {
	token, err := jwt.ParseWithClaims(identityToken, &Claims{}, v.keyFunc,
		jwt.WithIssuer(issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", "", fmt.Errorf("parse identity token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", "", fmt.Errorf("invalid identity token")
	}
	if claims.Subject == "" {
		return "", "", fmt.Errorf("identity token has no subject")
	}
	return claims.Subject, claims.Email, nil
}

func (v *Verifier) keyFunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, fmt.Errorf("identity token has no kid header")
	}
	return v.publicKey(kid)
}

func (v *Verifier) publicKey(kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if key, ok := v.keys[kid]; ok && time.Since(v.fetchedAt) < time.Hour {
		return key, nil
	}
	if err := v.refreshKeysLocked(); err != nil {
		return nil, err
	}
	key, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("no matching Apple public key for kid %q", kid)
	}
	return key, nil
}

type jwk struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

func (v *Verifier) refreshKeysLocked() error {
	resp, err := http.Get(jwksURL)
	if err != nil {
		return fmt.Errorf("fetch Apple JWKS: %w", err)
	}
	defer resp.Body.Close()

	var set jwkSet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("decode Apple JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	v.keys = keys
	v.fetchedAt = time.Now()
	return nil
}

func parseRSAPublicKey(nEnc, eEnc string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nEnc)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eEnc)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
