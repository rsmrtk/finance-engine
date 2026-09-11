// Package ratesync periodically fetches official exchange rates from the
// National Bank of Ukraine and upserts them into the exchange_rates table.
package ratesync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/logger"
)

// Currencies the app cares about, besides UAH (which is always 1).
var supportedCurrencies = map[string]bool{
	"USD": true,
	"EUR": true,
	"GBP": true,
	"PLN": true,
}

// nbuRate matches the actual shape of https://bank.gov.ua/NBU_Exchange/exchange?json,
// e.g. {"CurrencyCode":"840","CurrencyCodeL":"USD","Units":1,"Amount":44.5483}.
type nbuRate struct {
	CurrencyCodeL string  `json:"CurrencyCodeL"`
	Units         float64 `json:"Units"`
	Amount        float64 `json:"Amount"`
}

type Syncer struct {
	url   string
	rates *repository.RateRepository
	log   logger.Logger
}

func New(url string, rates *repository.RateRepository, log logger.Logger) *Syncer {
	return &Syncer{url: url, rates: rates, log: log}
}

// Run fetches rates immediately, then again every interval, until ctx is
// canceled. Intended to run in its own goroutine for the lifetime of the
// server process.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	s.syncOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncOnce(ctx)
		}
	}
}

func (s *Syncer) syncOnce(ctx context.Context) {
	if err := s.fetchAndStore(ctx); err != nil {
		s.log.Error("failed to sync exchange rates", logger.H{"error": err.Error()})
		return
	}
	s.log.Info("exchange rates synced", nil)
}

func (s *Syncer) fetchAndStore(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch NBU rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("NBU API returned status %d", resp.StatusCode)
	}

	var rates []nbuRate
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return fmt.Errorf("decode NBU response: %w", err)
	}

	if err := s.rates.Upsert(ctx, "UAH", 1); err != nil {
		return fmt.Errorf("upsert UAH rate: %w", err)
	}

	for _, r := range rates {
		if !supportedCurrencies[r.CurrencyCodeL] || r.Units == 0 {
			continue
		}
		if err := s.rates.Upsert(ctx, r.CurrencyCodeL, r.Amount/r.Units); err != nil {
			return fmt.Errorf("upsert %s rate: %w", r.CurrencyCodeL, err)
		}
	}
	return nil
}
