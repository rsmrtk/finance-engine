package advisor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// --- Subscription detection ------------------------------------------------

// Subscription is a recurring-charge pattern detected from transaction
// notes — Monobank imports always carry the merchant name in Note, so
// this works well for bank-synced data; manually entered transactions
// without a note simply aren't grouped (nothing reliable to key on).
type Subscription struct {
	Description         string
	AverageAmount       float64
	Currency            string
	Occurrences         int
	LastDate            time.Time
	AverageIntervalDays float64
}

const (
	subscriptionWindow      = 180 * 24 * time.Hour
	subscriptionMinCount    = 3
	subscriptionMinInterval = 20.0 // days
	subscriptionMaxInterval = 40.0 // days — catches monthly-ish billing, not weekly/annual.
	subscriptionMaxVariance = 0.20 // amount can drift up to 20% (currency swings, small price changes) and still count.
)

// DetectSubscriptions groups same-note expenses over the last 180 days
// and flags groups that recur roughly monthly with a roughly stable
// amount — the signature of a subscription rather than coincidentally
// similar one-off purchases. Amounts are reported in their original
// currency (never converted) since a real subscription is always billed
// in one consistent currency.
func (s *Service) DetectSubscriptions(ctx context.Context, userID uuid.UUID) ([]Subscription, error) {
	since := time.Now().Add(-subscriptionWindow)
	txs, err := s.transactions.ListForUser(ctx, userID, since, time.Now())
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	type entry struct {
		date     time.Time
		amount   float64
		currency string
	}
	groups := map[string][]entry{}
	for _, tx := range txs {
		if tx.Type != "expense" || tx.IsInternalTransfer {
			continue
		}
		note := strings.ToLower(strings.TrimSpace(tx.Note))
		if note == "" {
			continue
		}
		amount, err := parseAmount(tx.Amount)
		if err != nil {
			continue
		}
		groups[note] = append(groups[note], entry{date: tx.Date, amount: amount, currency: tx.Currency})
	}

	var out []Subscription
	for note, entries := range groups {
		if len(entries) < subscriptionMinCount {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].date.Before(entries[j].date) })

		var intervalSum float64
		for i := 1; i < len(entries); i++ {
			intervalSum += entries[i].date.Sub(entries[i-1].date).Hours() / 24
		}
		avgInterval := intervalSum / float64(len(entries)-1)
		if avgInterval < subscriptionMinInterval || avgInterval > subscriptionMaxInterval {
			continue
		}

		var amountSum float64
		for _, e := range entries {
			amountSum += e.amount
		}
		avgAmount := amountSum / float64(len(entries))
		if avgAmount == 0 {
			continue
		}
		maxDeviation := 0.0
		for _, e := range entries {
			dev := (e.amount - avgAmount) / avgAmount
			if dev < 0 {
				dev = -dev
			}
			if dev > maxDeviation {
				maxDeviation = dev
			}
		}
		if maxDeviation > subscriptionMaxVariance {
			continue
		}

		last := entries[len(entries)-1]
		out = append(out, Subscription{
			Description:         displayNote(note),
			AverageAmount:       avgAmount,
			Currency:            last.currency,
			Occurrences:         len(entries),
			LastDate:            last.date,
			AverageIntervalDays: avgInterval,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].AverageAmount > out[j].AverageAmount })
	return out, nil
}

// displayNote restores simple title-casing for a lowercased group key —
// cosmetic only, so "netflix.com" reads as "Netflix.com" in the UI.
func displayNote(note string) string {
	if note == "" {
		return note
	}
	return strings.ToUpper(note[:1]) + note[1:]
}

// --- Payday runway forecast --------------------------------------------

// RunwayForecast answers "will I make it to my next payday, or run out
// first" — the two numbers (spend pace and expected next income) most
// budget apps show separately, cross-checked against each other instead.
type RunwayForecast struct {
	CurrentBalance     float64
	DailyBurnRate      float64
	ProjectedZeroDate  time.Time // Zero value if burn rate is <= 0 (never runs out).
	NextPaydayEstimate time.Time // Zero value if no recurring income pattern was found.
	WillMakeIt         bool
}

const runwayBurnWindow = 30 * 24 * time.Hour

// Runway computes the forecast from the user's full transaction history
// (for the running balance) and recent history (for burn rate + payday
// pattern detection).
func (s *Service) Runway(ctx context.Context, userID uuid.UUID, baseCurrency string) (RunwayForecast, error) {
	rates, err := s.rates.List(ctx)
	if err != nil {
		return RunwayForecast{}, fmt.Errorf("list rates: %w", err)
	}
	rateToUAH := make(map[string]float64, len(rates)+1)
	rateToUAH["UAH"] = 1
	for _, r := range rates {
		rateToUAH[r.Currency] = r.RateToUAH
	}

	allTxs, err := s.transactions.ListForUser(ctx, userID, time.Time{}, time.Now())
	if err != nil {
		return RunwayForecast{}, fmt.Errorf("list transactions: %w", err)
	}

	var balance float64
	// Salary-like income: grouped by category, the category with the
	// most total income over the window is assumed to be the paycheck —
	// its typical day-of-month is the payday estimate.
	incomeByCategory := map[uuid.UUID]float64{}
	var incomeDaysByCategory = map[uuid.UUID][]int{}
	since30 := time.Now().Add(-runwayBurnWindow)
	var recentExpense float64

	for _, tx := range allTxs {
		if tx.IsInternalTransfer {
			continue // Money moving between the user's own accounts — doesn't belong in salary detection or burn rate.
		}
		amount, err := parseAmount(tx.Amount)
		if err != nil {
			continue
		}
		converted := convert(amount, tx.Currency, baseCurrency, rateToUAH)
		if tx.Type == "income" {
			balance += converted
			incomeByCategory[tx.CategoryID] += converted
			incomeDaysByCategory[tx.CategoryID] = append(incomeDaysByCategory[tx.CategoryID], tx.Date.Day())
		} else {
			balance -= converted
			if !tx.Date.Before(since30) {
				recentExpense += converted
			}
		}
	}

	dailyBurn := recentExpense / 30

	var salaryCategoryID uuid.UUID
	maxIncome := 0.0
	for catID, total := range incomeByCategory {
		if total > maxIncome {
			maxIncome = total
			salaryCategoryID = catID
		}
	}

	var nextPayday time.Time
	if days := incomeDaysByCategory[salaryCategoryID]; len(days) >= 2 {
		sum := 0
		for _, d := range days {
			sum += d
		}
		typicalDay := sum / len(days)
		now := time.Now()
		candidate := time.Date(now.Year(), now.Month(), typicalDay, 0, 0, 0, 0, now.Location())
		if !candidate.After(now) {
			candidate = candidate.AddDate(0, 1, 0)
		}
		nextPayday = candidate
	}

	forecast := RunwayForecast{
		CurrentBalance:     balance,
		DailyBurnRate:      dailyBurn,
		NextPaydayEstimate: nextPayday,
		WillMakeIt:         true,
	}
	if dailyBurn > 0 {
		daysLeft := balance / dailyBurn
		forecast.ProjectedZeroDate = time.Now().AddDate(0, 0, int(daysLeft))
		if !nextPayday.IsZero() && forecast.ProjectedZeroDate.Before(nextPayday) {
			forecast.WillMakeIt = false
		}
	}
	return forecast, nil
}

// --- Budget pace tracker -------------------------------------------------

// CategoryPace compares this month's spend-so-far against what's typical
// for the same category, adjusted for how much of the month has already
// elapsed — "80% of a typical month's budget spent, but only 60% of the
// month has passed" is a much earlier warning than a plain running total.
type CategoryPace struct {
	CategoryID     uuid.UUID
	CategoryName   string
	TypicalMonthly float64 // Average of the last 3 full months.
	SpentSoFar     float64 // This month, to date.
	PaceRatio      float64 // (SpentSoFar / TypicalMonthly) / (day-of-month / days-in-month). 1.0 = right on pace.
}

const paceHistoryMonths = 3

// BudgetPace computes CategoryPace for every expense category with
// spending history, sorted by most over-pace first.
func (s *Service) BudgetPace(ctx context.Context, userID uuid.UUID, baseCurrency string) ([]CategoryPace, error) {
	now := time.Now()
	startOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	historyStart := startOfThisMonth.AddDate(0, -paceHistoryMonths, 0)

	txs, err := s.transactions.ListForUser(ctx, userID, historyStart, now)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	categories, err := s.categories.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	rates, err := s.rates.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rates: %w", err)
	}
	rateToUAH := make(map[string]float64, len(rates)+1)
	rateToUAH["UAH"] = 1
	for _, r := range rates {
		rateToUAH[r.Currency] = r.RateToUAH
	}
	names := make(map[uuid.UUID]string, len(categories))
	for _, c := range categories {
		names[c.ID] = c.Name
	}

	historyTotals := map[uuid.UUID]float64{}
	thisMonthTotals := map[uuid.UUID]float64{}
	for _, tx := range txs {
		if tx.Type != "expense" || tx.IsInternalTransfer {
			continue
		}
		amount, err := parseAmount(tx.Amount)
		if err != nil {
			continue
		}
		converted := convert(amount, tx.Currency, baseCurrency, rateToUAH)
		if tx.Date.Before(startOfThisMonth) {
			historyTotals[tx.CategoryID] += converted
		} else {
			thisMonthTotals[tx.CategoryID] += converted
		}
	}

	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	monthProgress := float64(now.Day()) / float64(daysInMonth)

	var out []CategoryPace
	for catID, historyTotal := range historyTotals {
		typical := historyTotal / paceHistoryMonths
		if typical == 0 {
			continue
		}
		spent := thisMonthTotals[catID]
		spendProgress := spent / typical
		pace := spendProgress / monthProgress
		out = append(out, CategoryPace{
			CategoryID:     catID,
			CategoryName:   names[catID],
			TypicalMonthly: typical,
			SpentSoFar:     spent,
			PaceRatio:      pace,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PaceRatio > out[j].PaceRatio })
	return out, nil
}
