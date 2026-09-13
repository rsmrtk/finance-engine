package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

type User struct {
	ID                 uuid.UUID
	AppleSub           string
	Email              string
	BaseCurrency       string
	PasswordHash       string
	GoogleSub          string
	Theme              string
	GradientColor      string
	Plan               string
	Goals              string
	SubscriptionStatus string
	TrialEndsAt        time.Time // Zero value means no trial/not set.
	LiqPayOrderID      string
	CreatedAt          time.Time
	Name               string
	Avatar             string // A small data: URI, or "" if never set.
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
	return userFromGetByAppleSubRow(row), nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.UserGetByEmail(ctx, pgutil.TextFromString(email))
	if err != nil {
		return User{}, err
	}
	return userFromGetByEmailRow(row), nil
}

func (r *UserRepository) GetByGoogleSub(ctx context.Context, googleSub string) (User, error) {
	row, err := r.q.UserGetByGoogleSub(ctx, pgutil.TextFromString(googleSub))
	if err != nil {
		return User{}, err
	}
	return userFromGetByGoogleSubRow(row), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.UserGetByID(ctx, pgutil.UUIDFromGoogle(id))
	if err != nil {
		return User{}, err
	}
	return userFromGetByIDRow(row), nil
}

// GetByLiqPayOrderID looks up the user a billing webhook callback
// belongs to (internal/service/billing) — see the query's own comment
// for why matching on the *current* order_id is what makes a replayed or
// superseded callback harmless.
func (r *UserRepository) GetByLiqPayOrderID(ctx context.Context, orderID string) (User, error) {
	row, err := r.q.UserGetByLiqPayOrderID(ctx, orderID)
	if err != nil {
		return User{}, err
	}
	return userFromGetByLiqPayOrderIDRow(row), nil
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
	return userFromCreateRow(row), nil
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
	return userFromCreateWithEmailRow(row), nil
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
	return userFromCreateWithGoogleRow(row), nil
}

// UpdateGoals saves the account's free-form financial goals/plans text.
func (r *UserRepository) UpdateGoals(ctx context.Context, id uuid.UUID, goals string) (User, error) {
	row, err := r.q.UserUpdateGoals(ctx, dbq.UserUpdateGoalsParams{ID: pgutil.UUIDFromGoogle(id), Goals: goals})
	if err != nil {
		return User{}, err
	}
	return userFromUpdateGoalsRow(row), nil
}

// UpdateBaseCurrency changes which currency amounts are converted into for
// display (dashboard totals, analytics) — the transactions themselves keep
// whatever currency they were recorded in.
func (r *UserRepository) UpdateBaseCurrency(ctx context.Context, id uuid.UUID, currency string) (User, error) {
	row, err := r.q.UserUpdateBaseCurrency(ctx, dbq.UserUpdateBaseCurrencyParams{ID: pgutil.UUIDFromGoogle(id), BaseCurrency: currency})
	if err != nil {
		return User{}, err
	}
	return userFromUpdateBaseCurrencyRow(row), nil
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
	return userFromUpdatePreferencesRow(row), nil
}

// UpdateProfile saves the display name + avatar shown on the profile
// page in place of the raw email/initial-circle.
func (r *UserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, name, avatar string) (User, error) {
	row, err := r.q.UserUpdateProfile(ctx, dbq.UserUpdateProfileParams{ID: pgutil.UUIDFromGoogle(id), Name: name, Avatar: avatar})
	if err != nil {
		return User{}, err
	}
	return userFromUpdateProfileRow(row), nil
}

// BeginCheckout records the order_id about to be sent to LiqPay's checkout
// (and, for a trial, the already-computed trial end date), before the
// user even leaves the app — the webhook that later confirms the
// subscription looks the user back up by this same order_id. Used for
// both StartTrial and a direct, no-trial Subscribe (internal/service/billing).
func (r *UserRepository) BeginCheckout(ctx context.Context, id uuid.UUID, orderID string, trialEndsAt time.Time) (User, error) {
	row, err := r.q.UserStartTrial(ctx, dbq.UserStartTrialParams{
		ID: pgutil.UUIDFromGoogle(id), LiqpayOrderID: orderID, TrialEndsAt: pgutil.NullTimeFromGo(trialEndsAt),
	})
	if err != nil {
		return User{}, err
	}
	return userFromStartTrialRow(row), nil
}

// UpdateSubscription applies a billing webhook's outcome: the new plan,
// subscription_status, and (for a trial) when it converts to a real charge.
func (r *UserRepository) UpdateSubscription(ctx context.Context, id uuid.UUID, planName, status string, trialEndsAt time.Time) (User, error) {
	row, err := r.q.UserUpdateSubscription(ctx, dbq.UserUpdateSubscriptionParams{
		ID: pgutil.UUIDFromGoogle(id), Plan: planName, SubscriptionStatus: status,
		TrialEndsAt: pgutil.NullTimeFromGo(trialEndsAt),
	})
	if err != nil {
		return User{}, err
	}
	return userFromUpdateSubscriptionRow(row), nil
}

// ListForAdmin powers the admin dashboard's user table — optional email
// search + plan filter, newest first, with the total match count
// alongside each row (a window function) so the caller doesn't need a
// second query just for pagination.
func (r *UserRepository) ListForAdmin(ctx context.Context, search, planFilter string, limit, offset int) ([]User, int64, error) {
	rows, err := r.q.UserListForAdmin(ctx, dbq.UserListForAdminParams{
		Limit: int32(limit), Offset: int32(offset), Column3: search, Column4: planFilter,
	})
	if err != nil {
		return nil, 0, err
	}
	users := make([]User, len(rows))
	var total int64
	for i, row := range rows {
		users[i] = User{
			ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
			BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
			Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
			SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
			CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
		}
		total = row.TotalCount
	}
	return users, total, nil
}

// PlanCount is one row of the admin dashboard's plan-distribution stats.
type PlanCount struct {
	Plan               string
	SubscriptionStatus string
	Count              int64
}

func (r *UserRepository) CountByPlan(ctx context.Context) ([]PlanCount, error) {
	rows, err := r.q.UserCountByPlan(ctx)
	if err != nil {
		return nil, err
	}
	counts := make([]PlanCount, len(rows))
	for i, row := range rows {
		counts[i] = PlanCount{Plan: row.Plan, SubscriptionStatus: row.SubscriptionStatus, Count: row.UserCount}
	}
	return counts, nil
}

// DaySignups is one point on the admin dashboard's signup trend chart.
type DaySignups struct {
	Day     time.Time
	Signups int64
}

// SignupsByDay returns real, zero-filled daily signup counts for the
// last `days` days — no fabricated data, just an actual GROUP BY.
func (r *UserRepository) SignupsByDay(ctx context.Context, days int) ([]DaySignups, error) {
	rows, err := r.q.UserSignupsByDay(ctx, int32(days))
	if err != nil {
		return nil, err
	}
	out := make([]DaySignups, len(rows))
	for i, row := range rows {
		out[i] = DaySignups{Day: row.Day.Time, Signups: row.Signups}
	}
	return out, nil
}

func userFromGetByAppleSubRow(row dbq.UserGetByAppleSubRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromGetByEmailRow(row dbq.UserGetByEmailRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromGetByGoogleSubRow(row dbq.UserGetByGoogleSubRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromGetByIDRow(row dbq.UserGetByIDRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromGetByLiqPayOrderIDRow(row dbq.UserGetByLiqPayOrderIDRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromCreateRow(row dbq.UserCreateRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromCreateWithEmailRow(row dbq.UserCreateWithEmailRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromCreateWithGoogleRow(row dbq.UserCreateWithGoogleRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromUpdateProfileRow(row dbq.UserUpdateProfileRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromUpdateGoalsRow(row dbq.UserUpdateGoalsRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromUpdateBaseCurrencyRow(row dbq.UserUpdateBaseCurrencyRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromUpdatePreferencesRow(row dbq.UserUpdatePreferencesRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromStartTrialRow(row dbq.UserStartTrialRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}

func userFromUpdateSubscriptionRow(row dbq.UserUpdateSubscriptionRow) User {
	return User{
		ID: pgutil.UUIDToGoogle(row.ID), AppleSub: row.AppleSub.String, Email: row.Email.String,
		BaseCurrency: row.BaseCurrency, PasswordHash: row.PasswordHash.String, GoogleSub: row.GoogleSub.String,
		Theme: row.Theme, GradientColor: row.GradientColor, Plan: row.Plan, Goals: row.Goals,
		SubscriptionStatus: row.SubscriptionStatus, TrialEndsAt: row.TrialEndsAt.Time, LiqPayOrderID: row.LiqpayOrderID,
		CreatedAt: row.CreatedAt.Time, Name: row.Name, Avatar: row.Avatar,
	}
}
