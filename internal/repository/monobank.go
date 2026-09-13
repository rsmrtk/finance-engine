package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

type MonobankConnection struct {
	UserID         uuid.UUID
	EncryptedToken []byte
	WebhookSecret  string
	MaskedPans     []string
	AccountIDs     []string
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
	MaskedPans     []string
	AccountIDs     []string
}

func (r *MonobankRepository) Upsert(ctx context.Context, p UpsertMonobankConnectionParams) (MonobankConnection, error) {
	row, err := r.q.MonobankConnectionUpsert(ctx, dbq.MonobankConnectionUpsertParams{
		UserID:         pgutil.UUIDFromGoogle(p.UserID),
		EncryptedToken: p.EncryptedToken,
		WebhookSecret:  p.WebhookSecret,
		MaskedPans:     p.MaskedPans,
		AccountIds:     p.AccountIDs,
	})
	if err != nil {
		return MonobankConnection{}, err
	}
	return MonobankConnection{
		UserID:         pgutil.UUIDToGoogle(row.UserID),
		EncryptedToken: row.EncryptedToken,
		WebhookSecret:  row.WebhookSecret,
		MaskedPans:     row.MaskedPans,
		AccountIDs:     row.AccountIds,
		ConnectedAt:    row.ConnectedAt.Time,
		LastSyncedAt:   optionalTime(row.LastSyncedAt),
	}, nil
}

func (r *MonobankRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (MonobankConnection, error) {
	row, err := r.q.MonobankConnectionGetByUserID(ctx, pgutil.UUIDFromGoogle(userID))
	if err != nil {
		return MonobankConnection{}, err
	}
	return MonobankConnection{
		UserID:         pgutil.UUIDToGoogle(row.UserID),
		EncryptedToken: row.EncryptedToken,
		WebhookSecret:  row.WebhookSecret,
		MaskedPans:     row.MaskedPans,
		AccountIDs:     row.AccountIds,
		ConnectedAt:    row.ConnectedAt.Time,
		LastSyncedAt:   optionalTime(row.LastSyncedAt),
	}, nil
}

func (r *MonobankRepository) GetByWebhookSecret(ctx context.Context, secret string) (MonobankConnection, error) {
	row, err := r.q.MonobankConnectionGetByWebhookSecret(ctx, secret)
	if err != nil {
		return MonobankConnection{}, err
	}
	return MonobankConnection{
		UserID:         pgutil.UUIDToGoogle(row.UserID),
		EncryptedToken: row.EncryptedToken,
		WebhookSecret:  row.WebhookSecret,
		MaskedPans:     row.MaskedPans,
		AccountIDs:     row.AccountIds,
		ConnectedAt:    row.ConnectedAt.Time,
		LastSyncedAt:   optionalTime(row.LastSyncedAt),
	}, nil
}

func (r *MonobankRepository) UpdateAccounts(ctx context.Context, userID uuid.UUID, accountIDs, maskedPans []string) (MonobankConnection, error) {
	row, err := r.q.MonobankConnectionUpdateAccounts(ctx, dbq.MonobankConnectionUpdateAccountsParams{
		UserID:     pgutil.UUIDFromGoogle(userID),
		AccountIds: accountIDs,
		MaskedPans: maskedPans,
	})
	if err != nil {
		return MonobankConnection{}, err
	}
	return MonobankConnection{
		UserID:         pgutil.UUIDToGoogle(row.UserID),
		EncryptedToken: row.EncryptedToken,
		WebhookSecret:  row.WebhookSecret,
		MaskedPans:     row.MaskedPans,
		AccountIDs:     row.AccountIds,
		ConnectedAt:    row.ConnectedAt.Time,
		LastSyncedAt:   optionalTime(row.LastSyncedAt),
	}, nil
}

func (r *MonobankRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	return r.q.MonobankConnectionDelete(ctx, pgutil.UUIDFromGoogle(userID))
}

func (r *MonobankRepository) TouchSync(ctx context.Context, userID uuid.UUID) error {
	return r.q.MonobankConnectionTouchSync(ctx, pgutil.UUIDFromGoogle(userID))
}

func optionalTime(t pgtype.Timestamptz) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}
