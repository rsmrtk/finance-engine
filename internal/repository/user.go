package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

type User struct {
	ID            uuid.UUID
	AppleSub      string
	Email         string
	BaseCurrency  string
	PasswordHash  string
	GoogleSub     string
	Theme         string
	GradientColor string
	Plan          string
	Goals         string
	CreatedAt     time.Time
}

type UserRepository struct {
	q *dbq.Queries
}

func NewUserRepository(q *dbq.Queries) *UserRepository {
	return &UserRepository{q: q}
}

func (r *UserRepository) GetByAppleSub(ctx context.Context, appleSub string) (User, error) {
	row, err := r.q.UserGetByAppleSub(ctx, pgutil.TextFromString(appleSub))
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.UserGetByEmail(ctx, pgutil.TextFromString(email))
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *UserRepository) GetByGoogleSub(ctx context.Context, googleSub string) (User, error) {
	row, err := r.q.UserGetByGoogleSub(ctx, pgutil.TextFromString(googleSub))
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.UserGetByID(ctx, pgutil.UUIDFromGoogle(id))
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *UserRepository) Create(ctx context.Context, appleSub, email, baseCurrency string) (User, error) {
	row, err := r.q.UserCreate(ctx, dbq.UserCreateParams{
		AppleSub:     pgutil.TextFromString(appleSub),
		Email:        pgutil.TextFromString(email),
		BaseCurrency: baseCurrency,
	})
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

// CreateWithEmail registers a user via email+password. passwordHash is
// already-hashed (see pkg/passwordhash) — this layer never sees a plaintext
// password.
func (r *UserRepository) CreateWithEmail(ctx context.Context, email, passwordHash, baseCurrency string) (User, error) {
	row, err := r.q.UserCreateWithEmail(ctx, dbq.UserCreateWithEmailParams{
		Email:        pgutil.TextFromString(email),
		PasswordHash: pgutil.TextFromString(passwordHash),
		BaseCurrency: baseCurrency,
	})
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *UserRepository) CreateWithGoogle(ctx context.Context, googleSub, email, baseCurrency string) (User, error) {
	row, err := r.q.UserCreateWithGoogle(ctx, dbq.UserCreateWithGoogleParams{
		GoogleSub:    pgutil.TextFromString(googleSub),
		Email:        pgutil.TextFromString(email),
		BaseCurrency: baseCurrency,
	})
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

// UpdateGoals saves the account's free-form financial goals/plans text.
func (r *UserRepository) UpdateGoals(ctx context.Context, id uuid.UUID, goals string) (User, error) {
	row, err := r.q.UserUpdateGoals(ctx, dbq.UserUpdateGoalsParams{ID: pgutil.UUIDFromGoogle(id), Goals: goals})
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

// UpdateBaseCurrency changes which currency amounts are converted into for
// display (dashboard totals, analytics) — the transactions themselves keep
// whatever currency they were recorded in.
func (r *UserRepository) UpdateBaseCurrency(ctx context.Context, id uuid.UUID, currency string) (User, error) {
	row, err := r.q.UserUpdateBaseCurrency(ctx, dbq.UserUpdateBaseCurrencyParams{ID: pgutil.UUIDFromGoogle(id), BaseCurrency: currency})
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}

// UpdatePreferences saves the account-wide theme + accent (gradient) color
// so they follow the user across devices/browsers instead of living only
// in one browser's localStorage.
func (r *UserRepository) UpdatePreferences(ctx context.Context, id uuid.UUID, theme, gradientColor string) (User, error) {
	row, err := r.q.UserUpdatePreferences(ctx, dbq.UserUpdatePreferencesParams{
		ID: pgutil.UUIDFromGoogle(id), Theme: theme, GradientColor: gradientColor,
	})
	if err != nil {
		return User{}, err
	}
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals, CreatedAt: row.CreatedAt.Time,
	}, nil
}
