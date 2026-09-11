package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

type MonobankConnection struct {
	UserID         uuid.UUID
	EncryptedToken []byte
	WebhookSecret  string
	MaskedPan      string
	AccountID      string
	ConnectedAt    time.Time
	LastSyncedAt   time.Time // Zero value if never synced.
}

type MonobankRepository struct {
	q *dbq.Queries
}

func NewMonobankRepository(q *dbq.Queries) *MonobankRepository {
	return &MonobankRepository{q: q}
}

type UpsertMonobankConnectionParams struct {
	UserID         uuid.UUID
	EncryptedToken []byte
	WebhookSecret  string
	MaskedPan      string
	AccountID      string
}

func (r *MonobankRepository) Upsert(ctx context.Context, p UpsertMonobankConnectionParams) (MonobankConnection, error) {
	row, err := r.q.MonobankConnectionUpsert(ctx, dbq.MonobankConnectionUpsertParams{
		UserID:         pgutil.UUIDFromGoogle(p.UserID),
		EncryptedToken: p.EncryptedToken,
		WebhookSecret:  p.WebhookSecret,
		MaskedPan:      p.MaskedPan,
		AccountID:      p.AccountID,
	})
	if err != nil {
		return MonobankConnection{}, err
	}
	return connectionFromRow(row), nil
}

func (r *MonobankRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (MonobankConnection, error) {
	row, err := r.q.MonobankConnectionGetByUserID(ctx, pgutil.UUIDFromGoogle(userID))
	if err != nil {
		return MonobankConnection{}, err
	}
	return connectionFromRow(row), nil
}

func (r *MonobankRepository) GetByWebhookSecret(ctx context.Context, secret string) (MonobankConnection, error) {
	row, err := r.q.MonobankConnectionGetByWebhookSecret(ctx, secret)
	if err != nil {
		return MonobankConnection{}, err
	}
	return connectionFromRow(row), nil
}

func (r *MonobankRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	return r.q.MonobankConnectionDelete(ctx, pgutil.UUIDFromGoogle(userID))
}

func (r *MonobankRepository) TouchSync(ctx context.Context, userID uuid.UUID) error {
	return r.q.MonobankConnectionTouchSync(ctx, pgutil.UUIDFromGoogle(userID))
}

func connectionFromRow(row dbq.MonobankConnection) MonobankConnection {
	lastSynced := time.Time{}
	if row.LastSyncedAt.Valid {
		lastSynced = row.LastSyncedAt.Time
	}
	return MonobankConnection{
		UserID:         pgutil.UUIDToGoogle(row.UserID),
		EncryptedToken: row.EncryptedToken,
		WebhookSecret:  row.WebhookSecret,
		MaskedPan:      row.MaskedPan,
		AccountID:      row.AccountID,
		ConnectedAt:    row.ConnectedAt.Time,
		LastSyncedAt:   lastSynced,
	}
}
