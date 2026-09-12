// Package session issues and validates the web's refresh-token sessions.
// This is deliberately separate from service/auth: auth.Service only cares
// about identity ("who is this"), while a browser session (short-lived
// access JWT + a revocable, DB-backed refresh token) is a web-transport
// concern that iOS/gRPC never needs — iOS keeps using its single
// long-lived JWT from service/auth, unchanged.
package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
	"github.com/rsmrtk/finance-engine/pkg/sessiontoken"
)

var ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")

type Service struct {
	sessions        *repository.SessionRepository
	accessJWT       jwt.JWT
	refreshDuration time.Duration
}

func New(sessions *repository.SessionRepository, accessJWT jwt.JWT, refreshDuration time.Duration) *Service {
	return &Service{sessions: sessions, accessJWT: accessJWT, refreshDuration: refreshDuration}
}

type Pair struct {
	AccessToken  string
	RefreshToken string
}

// Issue mints a fresh access+refresh pair for a just-authenticated user.
func (s *Service) Issue(ctx context.Context, userID uuid.UUID) (Pair, error) {
	access, err := s.accessJWT.Generate(userID)
	if err != nil {
		return Pair{}, fmt.Errorf("generate access token: %w", err)
	}

	raw, hash, err := sessiontoken.New()
	if err != nil {
		return Pair{}, fmt.Errorf("generate refresh token: %w", err)
	}
	if _, err := s.sessions.Create(ctx, userID, hash, time.Now().Add(s.refreshDuration)); err != nil {
		return Pair{}, fmt.Errorf("store session: %w", err)
	}

	return Pair{AccessToken: access, RefreshToken: raw}, nil
}

// Refresh rotates the refresh token (revokes the old one, issues a new
// pair) so a stolen-and-replayed old cookie stops working after the
// legitimate client refreshes once.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (Pair, error) {
	hash := sessiontoken.Hash(rawRefreshToken)
	sess, err := s.sessions.GetByRefreshHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Pair{}, ErrInvalidRefreshToken
		}
		return Pair{}, fmt.Errorf("lookup session: %w", err)
	}
	if !sess.RevokedAt.IsZero() || time.Now().After(sess.ExpiresAt) {
		return Pair{}, ErrInvalidRefreshToken
	}

	if err := s.sessions.RevokeByID(ctx, sess.ID); err != nil {
		return Pair{}, fmt.Errorf("revoke old session: %w", err)
	}
	return s.Issue(ctx, sess.UserID)
}

// Revoke logs a session out server-side (called on explicit logout).
// Unlike iOS's Apple JWT, this actually invalidates the session — the
// refresh token stops working immediately; the short-lived access token
// still expires naturally within JWTWebAccessDuration.
func (s *Service) Revoke(ctx context.Context, rawRefreshToken string) error {
	return s.sessions.RevokeByRefreshHash(ctx, sessiontoken.Hash(rawRefreshToken))
}
