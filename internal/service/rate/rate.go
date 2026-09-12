package rate

import (
	"context"
	"time"

	"github.com/rsmrtk/finance-engine/internal/repository"
)

type Service struct {
	rates *repository.RateRepository
}

func New(rates *repository.RateRepository) *Service {
	return &Service{rates: rates}
}

func (s *Service) List(ctx context.Context) ([]repository.Rate, error) {
	return s.rates.List(ctx)
}

// HistoryAt returns every supported currency's rate-to-UAH as it stood on
// `date` — so a transaction from a year ago converts at the rate that
// actually applied then, not today's.
func (s *Service) HistoryAt(ctx context.Context, date time.Time) (map[string]float64, error) {
	return s.rates.HistoricalRates(ctx, date)
}
