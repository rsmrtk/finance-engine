package rate

import (
	"context"

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
