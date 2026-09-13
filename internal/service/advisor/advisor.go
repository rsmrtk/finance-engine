// Package advisor is the "AI financial advisor" feature: a chat and a set
// of Dashboard insights, both grounded in the user's own transaction
// history and answered by a locally-hosted LLM (pkg/ollama) — free, and
// the user's income/expense data never leaves their machine.
package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/ollama"
)

type Service struct {
	transactions *repository.TransactionRepository
	categories   *repository.CategoryRepository
	rates        *repository.RateRepository
	llm          *ollama.Client
	rdb          *redis.Client
}

func New(transactions *repository.TransactionRepository, categories *repository.CategoryRepository, rates *repository.RateRepository, llm *ollama.Client, rdb *redis.Client) *Service {
	return &Service{transactions: transactions, categories: categories, rates: rates, llm: llm, rdb: rdb}
}

const systemPreamble = `Тебе звати Felix — AI-помічник застосунку Vaultly. Ти не формальний консультант, а дружній асистент користувача: розмовляй природно, як реальна людина-помічник, можеш мати легкий характер, але завжди корисний і чесний. Якщо запитання стосується фінансів — спирайся ЛИШЕ на дані користувача нижче, ніколи не вигадуй цифри. Якщо питання не про фінанси — просто підтримай розмову, ти не обмежений тільки цією темою.

Категорії користувача (список нижче) вже налаштовані в застосунку — НІКОЛИ не радь "створити категорії" чи "завести бюджет", вони вже є. Замість generic-порад на кшталт "складіть бюджет" чи "ведіть облік витрат" — давай конкретні спостереження з реальними цифрами й назвами категорій із даних нижче. Відсотки в даних нижче вже пораховані — використовуй їх як є, НЕ рахуй проценти чи співвідношення самостійно (арифметика в тебе ненадійна). Якщо даних замало для висновку — так і скажи, замість типової поради.

Відповідай тією ж мовою, якою пише користувач. Без вступних фраз на кшталт "звісно" чи "як AI-модель" — одразу по суті, природно. Коротко, якщо не просять розгорнуто.`

// Chat answers one user message, grounded in their recent financial
// summary. history is prior turns from the same conversation (kept
// client-side — this service is stateless, no chat is persisted).
func (s *Service) Chat(ctx context.Context, userID uuid.UUID, baseCurrency string, history []ollama.Message, userMessage string) (string, error) {
	summary, err := s.buildSummary(ctx, userID, baseCurrency)
	if err != nil {
		return "", fmt.Errorf("build financial summary: %w", err)
	}

	messages := make([]ollama.Message, 0, len(history)+2)
	messages = append(messages, ollama.Message{Role: "system", Content: systemPreamble + "\n\n" + summary})
	messages = append(messages, history...)
	messages = append(messages, ollama.Message{Role: "user", Content: userMessage})

	return s.llm.Chat(ctx, messages)
}

const insightsCacheTTL = time.Hour

// Insights returns 2-3 short, standalone observations about the user's
// recent spending (e.g. a category up sharply vs last month) — the model
// is asked for plain lines instead of JSON, since a small local model
// follows "one per line" far more reliably than strict JSON formatting.
// Cached in Redis for an hour: this runs on every Dashboard load, and an
// LLM call is far slower (and pointless to repeat) than the DB queries
// around it.
func (s *Service) Insights(ctx context.Context, userID uuid.UUID, baseCurrency string) ([]string, error) {
	cacheKey := fmt.Sprintf("advisor:insights:%s:%s", userID, baseCurrency)
	if cached, err := s.rdb.Get(ctx, cacheKey).Result(); err == nil {
		var insights []string
		if json.Unmarshal([]byte(cached), &insights) == nil {
			return insights, nil
		}
	}

	summary, err := s.buildSummary(ctx, userID, baseCurrency)
	if err != nil {
		return nil, fmt.Errorf("build financial summary: %w", err)
	}

	// A chat request with only a system message (no user turn) sometimes
	// makes the model echo template artifacts like a stray "assistant"
	// line before the real content — giving it an actual user turn to
	// respond to avoids that.
	reply, err := s.llm.Chat(ctx, []ollama.Message{
		{Role: "system", Content: systemPreamble + "\n\n" + summary},
		{Role: "user", Content: "Дай 2-3 коротких спостереження або поради на основі цих даних (кожне — окремий рядок, без нумерації, без зірочок, без вступу)."},
	})
	if err != nil {
		return nil, err
	}

	var insights []string
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-•*0123456789. "))
		if line == "" || strings.EqualFold(line, "assistant") {
			continue
		}
		insights = append(insights, line)
		if len(insights) == 3 {
			break
		}
	}

	if encoded, err := json.Marshal(insights); err == nil {
		s.rdb.Set(ctx, cacheKey, encoded, insightsCacheTTL)
	}
	return insights, nil
}

type categoryTotal struct {
	name   string
	amount float64
}

// FinancialScoreBreakdown is a 0-100 "financial literacy" gauge computed
// from a user's own last-30-days transactions — three equally-visible
// factors, not a black box: how much they keep vs spend, whether spending
// is spread across categories or concentrated in one, and how regularly
// they're actually logging transactions.
type FinancialScoreBreakdown struct {
	Total            int     // Weighted 0-100.
	SavingsRate      float64 // (income-expense)/income over the window, e.g. 0.15 = 15%. 0 if no income.
	SavingsScore     int     // 0-100.
	TopCategoryShare float64 // Largest expense category's share of total expenses, 0-1.
	BalanceScore     int     // 0-100.
	ActiveDays       int     // Distinct days with at least one transaction, out of the window.
	ConsistencyScore int     // 0-100.
}

const scoreWindow = 30 * 24 * time.Hour

// FinancialScore computes FinancialScoreBreakdown. Weights: 40% savings
// rate (20%+ saved = full marks, 0% or negative = zero), 30% category
// balance (top category under 25% of spend = full marks, over 75% =
// zero), 30% consistency (20+ active days out of the last 30 = full
// marks). These thresholds are a starting point, not a formula anyone
// should treat as precisely calibrated — tune them from real usage.
func (s *Service) FinancialScore(ctx context.Context, userID uuid.UUID, baseCurrency string) (FinancialScoreBreakdown, error) {
	now := time.Now()
	start := now.Add(-scoreWindow)

	txs, err := s.transactions.ListForUser(ctx, userID, start, now)
	if err != nil {
		return FinancialScoreBreakdown{}, err
	}
	rates, err := s.rates.List(ctx)
	if err != nil {
		return FinancialScoreBreakdown{}, err
	}
	rateToUAH := make(map[string]float64, len(rates)+1)
	rateToUAH["UAH"] = 1
	for _, r := range rates {
		rateToUAH[r.Currency] = r.RateToUAH
	}

	var income, expense float64
	expenseByCategory := map[uuid.UUID]float64{}
	activeDays := map[string]bool{}
	for _, tx := range txs {
		if tx.IsInternalTransfer {
			continue // Money moving between the user's own accounts — not real income or expense.
		}
		amount, _ := parseAmount(tx.Amount)
		converted := convert(amount, tx.Currency, baseCurrency, rateToUAH)
		activeDays[tx.Date.Format("2006-01-02")] = true
		if tx.Type == "income" {
			income += converted
		} else {
			expense += converted
			expenseByCategory[tx.CategoryID] += converted
		}
	}

	savingsRate := 0.0
	if income > 0 {
		savingsRate = (income - expense) / income
	}
	savingsScore := clampScore(savingsRate / 0.20 * 100)

	topCategoryShare := 0.0
	if expense > 0 {
		for _, amount := range expenseByCategory {
			if share := amount / expense; share > topCategoryShare {
				topCategoryShare = share
			}
		}
	}
	balanceScore := clampScore((0.75 - topCategoryShare) / 0.5 * 100)

	consistencyScore := clampScore(float64(len(activeDays)) / 20 * 100)

	total := int(math.Round(0.4*float64(savingsScore) + 0.3*float64(balanceScore) + 0.3*float64(consistencyScore)))

	return FinancialScoreBreakdown{
		Total:            total,
		SavingsRate:      savingsRate,
		SavingsScore:     savingsScore,
		TopCategoryShare: topCategoryShare,
		BalanceScore:     balanceScore,
		ActiveDays:       len(activeDays),
		ConsistencyScore: consistencyScore,
	}, nil
}

func clampScore(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return int(math.Round(v))
}

// buildSummary computes this-month and last-month income/expense (in
// baseCurrency) plus this month's top expense categories, formatted as a
// compact text block for the LLM's context — not the raw transaction
// list, which would blow past a small model's useful context fast and
// bury the numbers that actually matter.
func (s *Service) buildSummary(ctx context.Context, userID uuid.UUID, baseCurrency string) (string, error) {
	now := time.Now()
	startOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	startOfLastMonth := startOfThisMonth.AddDate(0, -1, 0)

	txs, err := s.transactions.ListForUser(ctx, userID, startOfLastMonth, now)
	if err != nil {
		return "", err
	}
	categories, err := s.categories.ListForUser(ctx, userID)
	if err != nil {
		return "", err
	}
	rates, err := s.rates.List(ctx)
	if err != nil {
		return "", err
	}

	categoryNames := make(map[uuid.UUID]string, len(categories))
	for _, c := range categories {
		categoryNames[c.ID] = c.Name
	}
	rateToUAH := make(map[string]float64, len(rates)+1)
	rateToUAH["UAH"] = 1
	for _, r := range rates {
		rateToUAH[r.Currency] = r.RateToUAH
	}

	var thisIncome, thisExpense, lastIncome, lastExpense float64
	expenseByCategory := map[string]float64{}
	incomeByCategory := map[string]float64{}

	for _, tx := range txs {
		if tx.IsInternalTransfer {
			continue // Money moving between the user's own accounts — not real income or expense.
		}
		amount, _ := parseAmount(tx.Amount)
		converted := convert(amount, tx.Currency, baseCurrency, rateToUAH)
		isThisMonth := !tx.Date.Before(startOfThisMonth)

		switch {
		case tx.Type == "income" && isThisMonth:
			thisIncome += converted
			name := categoryNames[tx.CategoryID]
			if name == "" {
				name = "Без категорії"
			}
			incomeByCategory[name] += converted
		case tx.Type == "expense" && isThisMonth:
			thisExpense += converted
			name := categoryNames[tx.CategoryID]
			if name == "" {
				name = "Без категорії"
			}
			expenseByCategory[name] += converted
		case tx.Type == "income":
			lastIncome += converted
		case tx.Type == "expense":
			lastExpense += converted
		}
	}

	// Every category with spending this month, not just the top few — Felix
	// reasons better about "which category is disproportionate" with the
	// full picture than with an arbitrarily truncated one.
	top := make([]categoryTotal, 0, len(expenseByCategory))
	for name, amount := range expenseByCategory {
		top = append(top, categoryTotal{name: name, amount: amount})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].amount > top[j].amount })

	var expenseCategories, incomeCategories []string
	for _, c := range categories {
		if c.Type == "expense" {
			expenseCategories = append(expenseCategories, c.Name)
		} else {
			incomeCategories = append(incomeCategories, c.Name)
		}
	}

	topIncome := make([]categoryTotal, 0, len(incomeByCategory))
	for name, amount := range incomeByCategory {
		topIncome = append(topIncome, categoryTotal{name: name, amount: amount})
	}
	sort.Slice(topIncome, func(i, j int) bool { return topIncome[i].amount > topIncome[j].amount })

	// Percentages are computed here, not left for the model to work out —
	// an 8B model does surprisingly unreliable arithmetic (e.g. calling
	// 8000/30000 "more than half"), so every ratio Felix might want to
	// quote is handed to it pre-computed instead.
	expensePctOfIncome := 0.0
	savingsRatePct := 0.0
	if thisIncome > 0 {
		expensePctOfIncome = thisExpense / thisIncome * 100
		savingsRatePct = (thisIncome - thisExpense) / thisIncome * 100
	}
	expenseChangePct := 0.0
	if lastExpense > 0 {
		expenseChangePct = (thisExpense - lastExpense) / lastExpense * 100
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Базова валюта користувача: %s\n", baseCurrency)
	fmt.Fprintf(&b, "Категорії витрат, які вже є у користувача: %s\n", strings.Join(expenseCategories, ", "))
	fmt.Fprintf(&b, "Категорії доходів, які вже є у користувача: %s\n", strings.Join(incomeCategories, ", "))
	fmt.Fprintf(&b, "Цей місяць: дохід %.0f, витрати %.0f (це %.0f%% від доходу, збережено %.0f%% доходу)\n", thisIncome, thisExpense, expensePctOfIncome, savingsRatePct)
	fmt.Fprintf(&b, "Минулий місяць: дохід %.0f, витрати %.0f\n", lastIncome, lastExpense)
	if lastExpense > 0 {
		fmt.Fprintf(&b, "Зміна витрат порівняно з минулим місяцем: %+.0f%%\n", expenseChangePct)
	}
	if len(top) > 0 {
		b.WriteString("Розподіл витрат цього місяця по категоріях (сума і % від загальних витрат цього місяця):\n")
		for _, ct := range top {
			pctOfExpense := 0.0
			if thisExpense > 0 {
				pctOfExpense = ct.amount / thisExpense * 100
			}
			fmt.Fprintf(&b, "- %s: %.0f (%.0f%%)\n", ct.name, ct.amount, pctOfExpense)
		}
	} else {
		b.WriteString("Цього місяця ще немає жодної витрати.\n")
	}
	if len(topIncome) > 0 {
		b.WriteString("Розподіл доходів цього місяця по категоріях (сума і % від загального доходу цього місяця):\n")
		for _, ct := range topIncome {
			pctOfIncome := 0.0
			if thisIncome > 0 {
				pctOfIncome = ct.amount / thisIncome * 100
			}
			fmt.Fprintf(&b, "- %s: %.0f (%.0f%%)\n", ct.name, ct.amount, pctOfIncome)
		}
	}

	if score, err := s.FinancialScore(ctx, userID, baseCurrency); err == nil {
		fmt.Fprintf(&b, "\nШкала фінансової грамотності користувача (0-100, за останні 30 днів): %d/100.\n", score.Total)
		fmt.Fprintf(&b, "Складові цієї шкали: заощадження %d/100 (%.0f%% доходу відкладено), баланс категорій %d/100 (найбільша категорія витрат займає %.0f%% усіх витрат), регулярність внесення транзакцій %d/100 (%d активних днів із 30).\n", score.SavingsScore, score.SavingsRate*100, score.BalanceScore, score.TopCategoryShare*100, score.ConsistencyScore, score.ActiveDays)
		b.WriteString("Якщо користувач питає про цю шкалу — поясни її саме через ці три складові, а не абстрактно.\n")
	}

	return b.String(), nil
}

func convert(amount float64, from, to string, rateToUAH map[string]float64) float64 {
	if from == to {
		return amount
	}
	fromRate, toRate := rateToUAH[from], rateToUAH[to]
	if fromRate == 0 || toRate == 0 {
		return amount // Unknown currency pair — better to show it unconverted than silently drop it.
	}
	return amount * fromRate / toRate
}

func parseAmount(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
