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
}

type SessionRepository struct {
	q *dbq.Queries
}

func NewSessionRepository(q *dbq.Queries) *SessionRepository {
	return &SessionRepository{q: q}
}

// Create stores a new session keyed by refreshHash (never the raw refresh
// token — see pkg/sessiontoken).
func (r *SessionRepository) Create(ctx context.Context, userID uuid.UUID, refreshHash string, expiresAt time.Time) (Session, error) {
	row, err := r.q.SessionCreate(ctx, dbq.SessionCreateParams{
		UserID:      pgutil.UUIDFromGoogle(userID),
		RefreshHash: refreshHash,
		ExpiresAt:   pgutil.TimeFromGo(expiresAt),
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromRow(row), nil
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
	}
}
