package repository

import (
	"context"
	"time"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

type Rate struct {
	Currency  string
	RateToUAH float64
	UpdatedAt time.Time
}

type RateRepository struct {
	q *dbq.Queries
}

func NewRateRepository(q *dbq.Queries) *RateRepository {
	return &RateRepository{q: q}
}

func (r *RateRepository) List(ctx context.Context) ([]Rate, error) {
	rows, err := r.q.RateList(ctx)
	if err != nil {
		return nil, err
	}
	rates := make([]Rate, len(rows))
	for i, row := range rows {
		rates[i] = Rate{
			Currency:  row.Currency,
			RateToUAH: pgutil.NumericToFloat64(row.RateToUah),
			UpdatedAt: row.UpdatedAt.Time,
		}
	}
	return rates, nil
}

func (r *RateRepository) Upsert(ctx context.Context, currency string, rateToUAH float64) error {
	numeric, err := pgutil.NumericFromFloat64(rateToUAH)
	if err != nil {
		return err
	}
	return r.q.RateUpsert(ctx, dbq.RateUpsertParams{
		Currency:  currency,
		RateToUah: numeric,
	})
}
