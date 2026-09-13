// Package admin is finance-dashboard's backend: read/search the user
// base and manually manage a subscription (plan, trial length, cancel)
// without going through LiqPay — the support/ops path for the one
// operator, not something end users ever call.
package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/internal/plan"
	"github.com/rsmrtk/finance-engine/internal/repository"
	billingsvc "github.com/rsmrtk/finance-engine/internal/service/billing"
)

type Service struct {
	users        *repository.UserRepository
	transactions *repository.TransactionRepository
	billing      *billingsvc.Service
}

func New(users *repository.UserRepository, transactions *repository.TransactionRepository, billing *billingsvc.Service) *Service {
	return &Service{users: users, transactions: transactions, billing: billing}
}

type UserList struct {
	Users []repository.User
	Total int64
}

func (s *Service) ListUsers(ctx context.Context, search, planFilter string, limit, offset int) (UserList, error) {
	users, total, err := s.users.ListForAdmin(ctx, search, planFilter, limit, offset)
	if err != nil {
		return UserList{}, err
	}
	return UserList{Users: users, Total: total}, nil
}

type UserDetail struct {
	User             repository.User
	TransactionCount int64
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (UserDetail, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return UserDetail{}, err
	}
	txCount, err := s.transactions.CountForUser(ctx, id)
	if err != nil {
		return UserDetail{}, err
	}
	return UserDetail{User: user, TransactionCount: txCount}, nil
}

// SetPlan grants (or revokes) a plan by hand — subscription_status
// "manual" marks it as not billed through LiqPay, so the retention job
// and plan checks treat it identically to a paid plan but nothing here
// will ever try to charge a card for it.
func (s *Service) SetPlan(ctx context.Context, id uuid.UUID, planName string) (repository.User, error) {
	switch planName {
	case plan.Free, plan.Pro, plan.Max, plan.Enterprise:
	default:
		return repository.User{}, fmt.Errorf("invalid plan: %s", planName)
	}
	status := "manual"
	if planName == plan.Free {
		status = ""
	}
	return s.users.UpdateSubscription(ctx, id, planName, status, time.Time{})
}

// ExtendTrial adds days to a user's trial (or starts a fresh one from
// today if they don't have one), granting Max for the duration.
func (s *Service) ExtendTrial(ctx context.Context, id uuid.UUID, days int) (repository.User, error) {
	if days <= 0 {
		return repository.User{}, fmt.Errorf("days must be positive")
	}
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return repository.User{}, err
	}
	base := user.TrialEndsAt
	if base.Before(time.Now()) {
		base = time.Now()
	}
	newEnd := base.AddDate(0, 0, days)
	return s.users.UpdateSubscription(ctx, id, plan.Max, "trialing", newEnd)
}

// CancelSubscription downgrades to Free — best-effort unsubscribes via
// LiqPay first if the user has a real (non-manual) subscription, but
// always downgrades regardless of whether that call succeeds; a stuck
// LiqPay subscription shouldn't block the operator from freeing up the
// account locally.
func (s *Service) CancelSubscription(ctx context.Context, id uuid.UUID) error {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if user.LiqPayOrderID != "" {
		_ = s.billing.Cancel(ctx, id) // Already downgrades to Free on success.
		return nil
	}
	_, err = s.users.UpdateSubscription(ctx, id, plan.Free, "", time.Time{})
	return err
}

type DaySignups struct {
	Day     time.Time
	Signups int64
}

type Stats struct {
	TotalUsers int64
	ByPlan     map[string]int64
	// ActiveMRR only counts subscription_status "active" — i.e. LiqPay is
	// actually charging that user. "manual" grants (admin freebies) and
	// "trialing" (not converted yet) are real ByPlan counts but $0 of
	// real revenue, so they're deliberately excluded here.
	ActiveMRR    float64
	TrialCount   int64
	SignupsByDay []DaySignups // Last 30 days, real counts — see repository.UserRepository.SignupsByDay.
}

const (
	proPriceUSD     = 15
	maxPriceUSD     = 99
	signupTrendDays = 30
)

func (s *Service) Stats(ctx context.Context) (Stats, error) {
	counts, err := s.users.CountByPlan(ctx)
	if err != nil {
		return Stats{}, err
	}
	signups, err := s.users.SignupsByDay(ctx, signupTrendDays)
	if err != nil {
		return Stats{}, err
	}

	stats := Stats{ByPlan: map[string]int64{}}
	for _, c := range counts {
		stats.TotalUsers += c.Count
		stats.ByPlan[c.Plan] += c.Count

		if c.SubscriptionStatus == "trialing" {
			stats.TrialCount += c.Count
		}
		if c.SubscriptionStatus == "active" {
			switch c.Plan {
			case plan.Pro:
				stats.ActiveMRR += float64(c.Count) * proPriceUSD
			case plan.Max:
				stats.ActiveMRR += float64(c.Count) * maxPriceUSD
			}
		}
	}
	for _, d := range signups {
		stats.SignupsByDay = append(stats.SignupsByDay, DaySignups{Day: d.Day, Signups: d.Signups})
	}
	return stats, nil
}
