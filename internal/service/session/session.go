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
// userAgent is stored purely for display on the Privacy page's active
// sessions list ("Chrome on macOS") — it plays no role in auth itself.
func (s *Service) Issue(ctx context.Context, userID uuid.UUID, userAgent string) (Pair, error) {
	access, err := s.accessJWT.Generate(userID)
	if err != nil {
		return Pair{}, fmt.Errorf("generate access token: %w", err)
	}

	raw, hash, err := sessiontoken.New()
	if err != nil {
		return Pair{}, fmt.Errorf("generate refresh token: %w", err)
	}
	if _, err := s.sessions.Create(ctx, userID, hash, time.Now().Add(s.refreshDuration), userAgent); err != nil {
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
	// Carries the same device label forward across rotation — the request
	// that triggers a silent refresh doesn't go through issueAndRespond,
	// so there's no fresh User-Agent header to read here.
	return s.Issue(ctx, sess.UserID, sess.UserAgent)
}

// Revoke logs a session out server-side (called on explicit logout).
// Unlike iOS's Apple JWT, this actually invalidates the session — the
// refresh token stops working immediately; the short-lived access token
// still expires naturally within JWTWebAccessDuration.
func (s *Service) Revoke(ctx context.Context, rawRefreshToken string) error {
	return s.sessions.RevokeByRefreshHash(ctx, sessiontoken.Hash(rawRefreshToken))
}

// Active is a session as shown on the Privacy page — never exposes the
// refresh hash itself, only what's needed to display and act on the list.
type Active struct {
	ID        uuid.UUID
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
	Current   bool
}

// ListActive returns the user's active sessions, flagging which one (if
// any) matches the caller's own current refresh token — rawRefreshToken is
// empty when called somewhere that doesn't have the cookie (there is none
// today, but keeps the signature honest about why matching can miss).
func (s *Service) ListActive(ctx context.Context, userID uuid.UUID, rawRefreshToken string) ([]Active, error) {
	sessions, err := s.sessions.ListActiveForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	currentHash := ""
	if rawRefreshToken != "" {
		currentHash = sessiontoken.Hash(rawRefreshToken)
	}
	active := make([]Active, len(sessions))
	for i, sess := range sessions {
		active[i] = Active{
			ID:        sess.ID,
			UserAgent: sess.UserAgent,
			CreatedAt: sess.CreatedAt,
			ExpiresAt: sess.ExpiresAt,
			Current:   currentHash != "" && sess.RefreshHash == currentHash,
		}
	}
	return active, nil
}

// RevokeForUser lets a user kill one of their own sessions from the
// Privacy page (e.g. a lost device) — ownership-checked in the repository
// so a session ID can't be used to revoke someone else's.
func (s *Service) RevokeForUser(ctx context.Context, userID, sessionID uuid.UUID) error {
	ok, err := s.sessions.RevokeForUser(ctx, sessionID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session not found")
	}
	return nil
}
