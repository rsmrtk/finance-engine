// Package billing runs the Max plan's 7-day-trial-then-subscription flow
// via LiqPay (pkg/liqpay) — the only Ukraine-friendly processor of the
// three considered, since Stripe doesn't support Ukrainian merchants.
package billing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/internal/plan"
	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/liqpay"
	"github.com/rsmrtk/finance-engine/pkg/logger"
)

const (
	trialDuration   = 7 * 24 * time.Hour
	proPlanPriceUSD = 15
	maxPlanPriceUSD = 99
)

// planPrice is the only place a plan's monthly USD price is defined for
// billing purposes — internal/service/admin's MRR math has its own copy
// for stats display, but this one is what actually gets charged.
func planPrice(planName string) (float64, bool) {
	switch planName {
	case plan.Pro:
		return proPlanPriceUSD, true
	case plan.Max:
		return maxPlanPriceUSD, true
	default:
		return 0, false
	}
}

type Service struct {
	users       *repository.UserRepository
	payments    *repository.PaymentRepository
	liqpay      *liqpay.Client
	frontendURL string // e.g. https://localhost:5173 — where result_url sends the browser back to.
	backendURL  string // e.g. the ngrok/public URL — where server_url (the webhook) lives.
	log         logger.Logger
}

func New(users *repository.UserRepository, payments *repository.PaymentRepository, lp *liqpay.Client, frontendURL, backendURL string, log logger.Logger) *Service {
	return &Service{users: users, payments: payments, liqpay: lp, frontendURL: frontendURL, backendURL: backendURL, log: log}
}

// Checkout is what the frontend needs to build the hidden form that
// redirects the browser to LiqPay's hosted card page.
type Checkout struct {
	URL       string
	Data      string
	Signature string
}

// orderID encodes everything HandleCallback needs to act on a webhook
// without a separate DB lookup for "what was this order for": the flow
// (trial vs a direct, no-delay purchase) and the target plan. "." never
// appears in a plan name, flow name, UUID, or unix timestamp, so it's a
// safe separator.
func orderID(flow, planName string, userID uuid.UUID) string {
	return fmt.Sprintf("sub.%s.%s.%s.%d", flow, planName, userID, time.Now().Unix())
}

func parseOrderID(id string) (flow, planName string, ok bool) {
	parts := strings.Split(id, ".")
	if len(parts) != 5 || parts[0] != "sub" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// StartTrial begins a one-time 7-day Max trial: the card is validated and
// tokenized now, but the first real charge (subscribe_date_start) doesn't
// happen until the trial ends. A user can only ever do this once — a
// non-empty subscription_status means they already have (or had) one.
func (s *Service) StartTrial(ctx context.Context, userID uuid.UUID) (Checkout, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return Checkout{}, fmt.Errorf("look up user: %w", err)
	}
	if user.SubscriptionStatus != "" {
		return Checkout{}, fmt.Errorf("a trial or subscription already exists for this account")
	}

	id := orderID("trial", plan.Max, userID)
	trialEndsAt := time.Now().Add(trialDuration)
	if _, err := s.users.BeginCheckout(ctx, userID, id, trialEndsAt); err != nil {
		return Checkout{}, fmt.Errorf("record trial start: %w", err)
	}

	data, signature, err := s.liqpay.CheckoutFields(map[string]any{
		"action":                liqpay.ActionSub,
		"amount":                maxPlanPriceUSD,
		"currency":              "USD",
		"description":           "Vaultly Max — monthly subscription",
		"order_id":              id,
		"subscribe":             1,
		"subscribe_date_start":  liqpay.FormatSubscribeDate(trialEndsAt),
		"subscribe_periodicity": liqpay.Periodicity,
		"server_url":            s.backendURL + "/webhooks/liqpay",
		"result_url":            s.frontendURL + "/app/profile/plan?checkout=done",
	})
	if err != nil {
		return Checkout{}, fmt.Errorf("build checkout: %w", err)
	}
	return Checkout{URL: liqpay.CheckoutURL, Data: data, Signature: signature}, nil
}

// Subscribe buys a plan directly — no trial delay, the first charge
// happens right after the card is validated. Used for Pro today; Max
// normally goes through StartTrial instead, but nothing stops reusing
// this for a Max purchase without a trial later.
func (s *Service) Subscribe(ctx context.Context, userID uuid.UUID, planName string) (Checkout, error) {
	price, ok := planPrice(planName)
	if !ok {
		return Checkout{}, fmt.Errorf("plan %q is not purchasable", planName)
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return Checkout{}, fmt.Errorf("look up user: %w", err)
	}
	if user.SubscriptionStatus != "" {
		return Checkout{}, fmt.Errorf("a trial or subscription already exists for this account")
	}

	id := orderID("direct", planName, userID)
	// A few minutes out, not literally "now" — LiqPay's recurring engine
	// schedules the first charge off this date, and a timestamp already
	// in the past by the time their servers process it has, in testing,
	// been rejected outright.
	startAt := time.Now().Add(5 * time.Minute)
	if _, err := s.users.BeginCheckout(ctx, userID, id, time.Time{}); err != nil {
		return Checkout{}, fmt.Errorf("record subscription start: %w", err)
	}

	data, signature, err := s.liqpay.CheckoutFields(map[string]any{
		"action":                liqpay.ActionSub,
		"amount":                price,
		"currency":              "USD",
		"description":           fmt.Sprintf("Vaultly %s — monthly subscription", planName),
		"order_id":              id,
		"subscribe":             1,
		"subscribe_date_start":  liqpay.FormatSubscribeDate(startAt),
		"subscribe_periodicity": liqpay.Periodicity,
		"server_url":            s.backendURL + "/webhooks/liqpay",
		"result_url":            s.frontendURL + "/app/profile/plan?checkout=done",
	})
	if err != nil {
		return Checkout{}, fmt.Errorf("build checkout: %w", err)
	}
	return Checkout{URL: liqpay.CheckoutURL, Data: data, Signature: signature}, nil
}

// HandleCallback processes one LiqPay webhook delivery. Every branch is
// keyed off cb.OrderID, which only ever matches the user whose *current*
// subscription it belongs to — a replayed or superseded callback just
// won't find anyone and is dropped.
func (s *Service) HandleCallback(ctx context.Context, rawData, rawSignature string) error {
	cb, err := s.liqpay.VerifyAndDecode(rawData, rawSignature)
	if err != nil {
		return fmt.Errorf("verify callback signature: %w", err)
	}
	s.log.Info("liqpay callback decoded", logger.H{
		"orderId":  cb.OrderID,
		"status":   cb.Status,
		"action":   cb.Action,
		"amount":   cb.Amount,
		"currency": cb.Currency,
	})

	user, err := s.users.GetByLiqPayOrderID(ctx, cb.OrderID)
	if err != nil {
		return fmt.Errorf("no user for liqpay order_id %q: %w", cb.OrderID, err)
	}
	flow, planName, ok := parseOrderID(cb.OrderID)
	if !ok {
		return fmt.Errorf("malformed order_id %q", cb.OrderID)
	}

	// Recorded regardless of outcome — a failed or errored charge is just
	// as much a receipt line as a successful one, and support/dispute
	// questions usually start with "what did LiqPay actually tell you".
	if _, recordErr := s.payments.Create(ctx, repository.CreatePaymentEventParams{
		UserID:           user.ID,
		OrderID:          cb.OrderID,
		Plan:             planName,
		Action:           cb.Action,
		Status:           cb.Status,
		Amount:           cb.Amount,
		Currency:         cb.Currency,
		ErrorDescription: cb.ErrDescription,
	}); recordErr != nil {
		return fmt.Errorf("record payment event: %w", recordErr)
	}

	switch cb.Status {
	case liqpay.StatusSubscribed:
		// Card tokenized. A trial flow isn't billed yet (status
		// "trialing", counting down to trial_ends_at set back in
		// StartTrial); a direct purchase starts getting real access
		// immediately even though the first charge event ("success")
		// technically lands moments later.
		status := "trialing"
		if flow == "direct" {
			status = "active"
		}
		_, err = s.users.UpdateSubscription(ctx, user.ID, planName, status, user.TrialEndsAt)
	case liqpay.StatusSuccess:
		// A charge succeeded — either the trial converting to paid, or a
		// later monthly renewal. Either way, active with no trial left.
		_, err = s.users.UpdateSubscription(ctx, user.ID, planName, "active", time.Time{})
	case liqpay.StatusFailure, liqpay.StatusUnsubscribed, liqpay.StatusError:
		_, err = s.users.UpdateSubscription(ctx, user.ID, plan.Free, "canceled", time.Time{})
	}
	return err
}

// Receipts returns a user's payment history (most recent first), for a
// billing/receipts page — never includes card data, LiqPay doesn't send
// us any.
func (s *Service) Receipts(ctx context.Context, userID uuid.UUID) ([]repository.PaymentEvent, error) {
	return s.payments.ListForUser(ctx, userID, 50)
}

// Cancel ends the subscription immediately (access drops to Free right
// away, not at the end of the current period — kept simple and honest
// about what "cancel" does rather than implying a grace period we don't
// track).
func (s *Service) Cancel(ctx context.Context, userID uuid.UUID) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("look up user: %w", err)
	}
	if user.LiqPayOrderID == "" {
		return fmt.Errorf("no active subscription to cancel")
	}

	if _, err := s.liqpay.Request(ctx, map[string]any{
		"action":   liqpay.ActionUnsub,
		"order_id": user.LiqPayOrderID,
	}); err != nil {
		return fmt.Errorf("unsubscribe via liqpay: %w", err)
	}

	_, err = s.users.UpdateSubscription(ctx, userID, plan.Free, "canceled", time.Time{})
	return err
}
