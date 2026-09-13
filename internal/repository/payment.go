package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

// PaymentEvent is one LiqPay callback we received — a receipt/audit
// trail, never the card itself (LiqPay never sends us that).
type PaymentEvent struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	OrderID          string
	Plan             string
	Action           string
	Status           string
	Amount           float64
	Currency         string
	ErrorDescription string
	CreatedAt        time.Time
}

type PaymentRepository struct {
	q *dbq.Queries
}

func NewPaymentRepository(q *dbq.Queries) *PaymentRepository {
	return &PaymentRepository{q: q}
}

type CreatePaymentEventParams struct {
	UserID           uuid.UUID
	OrderID          string
	Plan             string
	Action           string
	Status           string
	Amount           float64
	Currency         string
	ErrorDescription string
}

func (r *PaymentRepository) Create(ctx context.Context, p CreatePaymentEventParams) (PaymentEvent, error) {
	amount, err := pgutil.NumericFromFloat64(p.Amount)
	if err != nil {
		return PaymentEvent{}, err
	}
	row, err := r.q.PaymentEventCreate(ctx, dbq.PaymentEventCreateParams{
		UserID:           pgutil.UUIDFromGoogle(p.UserID),
		OrderID:          p.OrderID,
		Plan:             p.Plan,
		Action:           p.Action,
		Status:           p.Status,
		Amount:           amount,
		Currency:         p.Currency,
		ErrorDescription: p.ErrorDescription,
	})
	if err != nil {
		return PaymentEvent{}, err
	}
	return paymentEventFromRow(row), nil
}

func (r *PaymentRepository) ListForUser(ctx context.Context, userID uuid.UUID, limit int32) ([]PaymentEvent, error) {
	rows, err := r.q.PaymentEventListForUser(ctx, dbq.PaymentEventListForUserParams{
		UserID: pgutil.UUIDFromGoogle(userID),
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	events := make([]PaymentEvent, len(rows))
	for i, row := range rows {
		events[i] = paymentEventFromRow(row)
	}
	return events, nil
}

func paymentEventFromRow(row dbq.PaymentEvent) PaymentEvent {
	return PaymentEvent{
		ID:               pgutil.UUIDToGoogle(row.ID),
		UserID:           pgutil.UUIDToGoogle(row.UserID),
		OrderID:          row.OrderID,
		Plan:             row.Plan,
		Action:           row.Action,
		Status:           row.Status,
		Amount:           pgutil.NumericToFloat64(row.Amount),
		Currency:         row.Currency,
		ErrorDescription: row.ErrorDescription,
		CreatedAt:        row.CreatedAt.Time,
	}
}
