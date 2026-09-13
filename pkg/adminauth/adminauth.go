// Package adminauth issues and verifies tokens for finance-dashboard,
// the internal admin panel. There's no admin user model — just one
// shared password (config.AdminPassword) — so a token here only ever
// asserts "this request presented the admin password", never an
// identity. Deliberately a separate signing subject from pkg/jwt's user
// tokens so a leaked user JWT can never be replayed as an admin token.
package adminauth

import (
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

const subject = "admin"

type claims struct {
	gojwt.RegisteredClaims
}

func Generate(secret string, duration time.Duration) (string, error) {
	c := claims{RegisteredClaims: gojwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: gojwt.NewNumericDate(time.Now().Add(duration)),
		IssuedAt:  gojwt.NewNumericDate(time.Now()),
	}}
	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, c)
	return token.SignedString([]byte(secret))
}

func Verify(secret, token string) error {
	tkn, err := gojwt.ParseWithClaims(token, &claims{}, func(t *gojwt.Token) (any, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return fmt.Errorf("parse admin token: %w", err)
	}
	c, ok := tkn.Claims.(*claims)
	if !ok || !tkn.Valid || c.Subject != subject {
		return fmt.Errorf("invalid admin token")
	}
	return nil
}
