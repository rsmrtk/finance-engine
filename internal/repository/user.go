package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

type User struct {
	ID           uuid.UUID
	AppleSub     string
	Email        string
	BaseCurrency string
	CreatedAt    time.Time
}

type UserRepository struct {
	q *dbq.Queries
}

func NewUserRepository(q *dbq.Queries) *UserRepository {
	return &UserRepository{q: q}
}

func (r *UserRepository) GetByAppleSub(ctx context.Context, appleSub string) (User, error) {
	row, err := r.q.UserGetByAppleSub(ctx, appleSub)
	if err != nil {
		return User{}, err
	}
	return userFromRow(row), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.UserGetByID(ctx, pgutil.UUIDFromGoogle(id))
	if err != nil {
		return User{}, err
	}
	return userFromRow(row), nil
}

func (r *UserRepository) Create(ctx context.Context, appleSub, email, baseCurrency string) (User, error) {
	row, err := r.q.UserCreate(ctx, dbq.UserCreateParams{
		AppleSub:     appleSub,
		Email:        pgutil.TextFromString(email),
		BaseCurrency: baseCurrency,
	})
	if err != nil {
		return User{}, err
	}
	return userFromRow(row), nil
}

func userFromRow(row dbq.User) User {
	return User{
		ID:           pgutil.UUIDToGoogle(row.ID),
		AppleSub:     row.AppleSub,
		Email:        row.Email.String,
		BaseCurrency: row.BaseCurrency,
		CreatedAt:    row.CreatedAt.Time,
	}
}
