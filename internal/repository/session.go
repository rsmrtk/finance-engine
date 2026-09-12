package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

// Session is a web refresh-token session. The access token itself stays a
// stateless JWT (pkg/jwt, shared with iOS) — this is only the refresh side,
// which is what makes a web session revocable server-side, unlike the
// long-lived Apple JWT that iOS uses today.
type Session struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	RefreshHash string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   time.Time // zero value means "not revoked".
	UserAgent   string
}

type SessionRepository struct {
	q *dbq.Queries
}

func NewSessionRepository(q *dbq.Queries) *SessionRepository {
	return &SessionRepository{q: q}
}

// Create stores a new session keyed by refreshHash (never the raw refresh
// token — see pkg/sessiontoken).
func (r *SessionRepository) Create(ctx context.Context, userID uuid.UUID, refreshHash string, expiresAt time.Time, userAgent string) (Session, error) {
	row, err := r.q.SessionCreate(ctx, dbq.SessionCreateParams{
		UserID:      pgutil.UUIDFromGoogle(userID),
		RefreshHash: refreshHash,
		ExpiresAt:   pgutil.TimeFromGo(expiresAt),
		UserAgent:   userAgent,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromRow(row), nil
}

// ListActiveForUser returns the user's non-revoked, non-expired sessions —
// what the Privacy page's "active sessions" list shows, newest first.
func (r *SessionRepository) ListActiveForUser(ctx context.Context, userID uuid.UUID) ([]Session, error) {
	rows, err := r.q.SessionListActiveForUser(ctx, pgutil.UUIDFromGoogle(userID))
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, len(rows))
	for i, row := range rows {
		sessions[i] = sessionFromRow(row)
	}
	return sessions, nil
}

// RevokeForUser revokes a session by ID, but only if it belongs to userID —
// without this guard a user could revoke anyone's session by guessing IDs.
// Returns false (no error) if nothing matched.
func (r *SessionRepository) RevokeForUser(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	rows, err := r.q.SessionRevokeForUser(ctx, dbq.SessionRevokeForUserParams{
		ID:     pgutil.UUIDFromGoogle(id),
		UserID: pgutil.UUIDFromGoogle(userID),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *SessionRepository) GetByRefreshHash(ctx context.Context, refreshHash string) (Session, error) {
	row, err := r.q.SessionGetByRefreshHash(ctx, refreshHash)
	if err != nil {
		return Session{}, err
	}
	return sessionFromRow(row), nil
}

func (r *SessionRepository) RevokeByID(ctx context.Context, id uuid.UUID) error {
	return r.q.SessionRevokeByID(ctx, pgutil.UUIDFromGoogle(id))
}

func (r *SessionRepository) RevokeByRefreshHash(ctx context.Context, refreshHash string) error {
	return r.q.SessionRevokeByRefreshHash(ctx, refreshHash)
}

func sessionFromRow(row dbq.Session) Session {
	return Session{
		ID:          pgutil.UUIDToGoogle(row.ID),
		UserID:      pgutil.UUIDToGoogle(row.UserID),
		RefreshHash: row.RefreshHash,
		CreatedAt:   row.CreatedAt.Time,
		ExpiresAt:   row.ExpiresAt.Time,
		RevokedAt:   row.RevokedAt.Time,
		UserAgent:   row.UserAgent,
	}
}
