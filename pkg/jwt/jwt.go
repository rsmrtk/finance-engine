package jwt

import (
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	gojwt.RegisteredClaims
	UserID string `json:"user_id"`
}

type JWT interface {
	Generate(userID uuid.UUID) (string, error)
	Verify(token string) (uuid.UUID, error)
}

type jwtManager struct {
	secret   string
	duration time.Duration
}

func New(secret string, duration time.Duration) JWT {
	return &jwtManager{secret: secret, duration: duration}
}

func (m *jwtManager) Generate(userID uuid.UUID) (string, error) {
	claims := Claims{
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(m.duration)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
		UserID: userID.String(),
	}
	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secret))
}

func (m *jwtManager) Verify(token string) (uuid.UUID, error) {
	tkn, err := gojwt.ParseWithClaims(token, &Claims{}, func(token *gojwt.Token) (any, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secret), nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := tkn.Claims.(*Claims)
	if !ok || !tkn.Valid {
		return uuid.Nil, fmt.Errorf("invalid token claims")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user id in token: %w", err)
	}
	return userID, nil
}
