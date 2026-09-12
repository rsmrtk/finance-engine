package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
)

type Rate struct {
	Currency  string
	RateToUAH float64
	UpdatedAt time.Time
}

// redisKey is a single Redis hash: field = currency code, value = JSON
// {rate, updatedAt}. Exchange rates are global reference/statistics data
// (never user-specific, refreshed from NBU on a timer) — storing them in
// Redis instead of Postgres keeps the relational DB scoped to actual user
// data (accounts, categories, transactions, sessions).
const redisKey = "exchange_rates"

type rateValue struct {
	RateToUAH float64   `json:"rateToUah"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type RateRepository struct {
	rdb *redis.Client
}

func NewRateRepository(rdb *redis.Client) *RateRepository {
	return &RateRepository{rdb: rdb}
}

func (r *RateRepository) List(ctx context.Context) ([]Rate, error) {
	raw, err := r.rdb.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("read exchange rates from redis: %w", err)
	}

	rates := make([]Rate, 0, len(raw))
	for currency, blob := range raw {
		var v rateValue
		if err := json.Unmarshal([]byte(blob), &v); err != nil {
			continue // Corrupt/legacy entry — skip rather than fail the whole list.
		}
		rates = append(rates, Rate{Currency: currency, RateToUAH: v.RateToUAH, UpdatedAt: v.UpdatedAt})
	}
	sort.Slice(rates, func(i, j int) bool { return rates[i].Currency < rates[j].Currency })
	return rates, nil
}

func (r *RateRepository) Upsert(ctx context.Context, currency string, rateToUAH float64) error {
	blob, err := json.Marshal(rateValue{RateToUAH: rateToUAH, UpdatedAt: time.Now()})
	if err != nil {
		return fmt.Errorf("marshal rate: %w", err)
	}
	if err := r.rdb.HSet(ctx, redisKey, currency, blob).Err(); err != nil {
		return fmt.Errorf("write exchange rate to redis: %w", err)
	}
	return nil
}

var historicalSupportedCurrencies = map[string]bool{"USD": true, "EUR": true, "GBP": true, "PLN": true}

type nbuHistoricalRate struct {
	CC          string  `json:"cc"`
	RatePerUnit float64 `json:"rate_per_unit"`
}

// HistoricalRates returns the NBU's official rate-to-UAH for every
// supported currency as of `date` — used to convert old transactions at
// the rate that actually applied when they happened, not today's rate.
// A past date's rates never change, so the result is cached in Redis
// forever once fetched (one NBU call per calendar day, ever).
func (r *RateRepository) HistoricalRates(ctx context.Context, date time.Time) (map[string]float64, error) {
	dateKey := date.Format("20060102")
	cacheKey := "rate_history:" + dateKey

	if cached, err := r.rdb.Get(ctx, cacheKey).Result(); err == nil {
		var m map[string]float64
		if json.Unmarshal([]byte(cached), &m) == nil {
			return m, nil
		}
	}

	url := "https://bank.gov.ua/NBU_Exchange/exchange_site?date=" + dateKey + "&json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build NBU historical request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch NBU historical rates: %w", err)
	}
	defer resp.Body.Close()

	var raw []nbuHistoricalRate
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode NBU historical response: %w", err)
	}

	result := map[string]float64{"UAH": 1}
	for _, rate := range raw {
		if historicalSupportedCurrencies[rate.CC] {
			result[rate.CC] = rate.RatePerUnit
		}
	}
	// A date NBU has no data for (too old, or a non-banking day with no
	// separate publish) yields just {"UAH":1} — fall back to whatever's
	// currently cached rather than silently converting everything as if
	// 1 unit = 1 UAH.
	if len(result) == 1 {
		current, err := r.List(ctx)
		if err == nil {
			for _, c := range current {
				result[c.Currency] = c.RateToUAH
			}
		}
	}

	if blob, err := json.Marshal(result); err == nil {
		r.rdb.Set(ctx, cacheKey, blob, 0) // No TTL — history is immutable once published.
	}
	return result, nil
}
