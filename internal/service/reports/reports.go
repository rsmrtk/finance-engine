// Package reports handles CSV export/import of a user's transactions —
// export for taking data out (a month, or any custom range), import for
// bringing in an existing Excel-based expense log when someone migrates
// to the app. Import expects the same column shape Export produces, so
// export → edit in a spreadsheet → re-import round-trips cleanly.
package reports

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	categorysvc "github.com/rsmrtk/finance-engine/internal/service/category"
	transactionsvc "github.com/rsmrtk/finance-engine/internal/service/transaction"
)

type Service struct {
	transactions *transactionsvc.Service
	categories   *categorysvc.Service
}

func New(transactions *transactionsvc.Service, categories *categorysvc.Service) *Service {
	return &Service{transactions: transactions, categories: categories}
}

var csvHeader = []string{"date", "type", "category", "amount", "currency", "note"}

// Export renders every transaction in [from, to] as CSV, newest first.
func (s *Service) Export(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]byte, error) {
	txs, err := s.transactions.List(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	cats, err := s.categories.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	names := make(map[uuid.UUID]string, len(cats))
	for _, c := range cats {
		names[c.ID] = c.Name
	}

	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write(csvHeader); err != nil {
		return nil, err
	}
	for _, tx := range txs {
		if err := w.Write([]string{
			tx.Date.Format("2006-01-02"),
			tx.Type,
			names[tx.CategoryID],
			tx.Amount,
			tx.Currency,
			tx.Note,
		}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

type ImportResult struct {
	Imported int
	Skipped  int
	Errors   []string // Human-readable, capped — a full row-by-row dump isn't useful past a certain size.
}

const maxImportErrors = 20

// Import parses CSV in the shape Export produces (date, type, category,
// amount, currency, note — header row optional). Unknown categories are
// created automatically, type-matched — an Excel migration should keep
// whatever categories the user already used, not dump everything into
// "Інше". Amount tolerates a comma decimal separator and thousands
// spaces (common in UA-locale spreadsheet exports); type and date accept
// a few common spellings/formats rather than requiring one exact shape.
func (s *Service) Import(ctx context.Context, userID uuid.UUID, defaultCurrency string, data []byte) (ImportResult, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	rows, err := r.ReadAll()
	if err != nil {
		return ImportResult{}, fmt.Errorf("parse csv: %w", err)
	}
	if len(rows) == 0 {
		return ImportResult{}, fmt.Errorf("file is empty")
	}

	start := 0
	if looksLikeHeader(rows[0]) {
		start = 1
	}

	existing, err := s.categories.List(ctx, userID)
	if err != nil {
		return ImportResult{}, fmt.Errorf("list categories: %w", err)
	}
	categoryCache := make(map[string]uuid.UUID, len(existing))
	for _, c := range existing {
		categoryCache[cacheKey(c.Type, c.Name)] = c.ID
	}

	var result ImportResult
	addError := func(rowNum int, format string, args ...any) {
		result.Skipped++
		if len(result.Errors) < maxImportErrors {
			result.Errors = append(result.Errors, fmt.Sprintf("рядок %d: %s", rowNum, fmt.Sprintf(format, args...)))
		}
	}

	for i := start; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue // Blank line.
		}

		txType, err := normalizeType(field(row, 1))
		if err != nil {
			addError(rowNum, "невідомий тип операції %q (очікується expense/income)", field(row, 1))
			continue
		}
		date, err := parseDate(field(row, 0))
		if err != nil {
			addError(rowNum, "невірна дата %q", field(row, 0))
			continue
		}
		amount, err := normalizeAmount(field(row, 3))
		if err != nil {
			addError(rowNum, "невірна сума %q", field(row, 3))
			continue
		}
		currency := strings.ToUpper(strings.TrimSpace(field(row, 4)))
		if currency == "" {
			currency = defaultCurrency
		}
		categoryName := strings.TrimSpace(field(row, 2))
		note := field(row, 5)

		categoryID := uuid.Nil
		if categoryName != "" {
			key := cacheKey(txType, categoryName)
			id, ok := categoryCache[key]
			if !ok {
				created, err := s.categories.Create(ctx, userID, categoryName, "circle", "8E8E93", txType)
				if err != nil {
					addError(rowNum, "не вдалось створити категорію %q", categoryName)
					continue
				}
				id = created.ID
				categoryCache[key] = id
			}
			categoryID = id
		}

		if _, err := s.transactions.Create(ctx, transactionsvc.CreateParams{
			UserID: userID, CategoryID: categoryID, Amount: amount, Currency: currency,
			Type: txType, Date: date, Note: note,
		}); err != nil {
			addError(rowNum, "%s", err.Error())
			continue
		}
		result.Imported++
	}

	return result, nil
}

func field(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return row[i]
}

func looksLikeHeader(row []string) bool {
	first := strings.ToLower(strings.TrimSpace(field(row, 0)))
	return first == "date" || first == "дата"
}

func cacheKey(transactionType, name string) string {
	return strings.ToLower(transactionType) + "|" + strings.ToLower(strings.TrimSpace(name))
}

func normalizeType(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "expense", "витрата", "витрати", "видаток", "видатки":
		return "expense", nil
	case "income", "дохід", "доход", "прибуток":
		return "income", nil
	default:
		return "", fmt.Errorf("unknown type %q", s)
	}
}

var dateLayouts = []string{
	"2006-01-02",
	"02.01.2006",
	"2006/01/02",
	"02/01/2006",
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}

// normalizeAmount tolerates spreadsheet export quirks: a comma decimal
// separator (common in UA-locale Excel), thousands-separator spaces, and
// a stray currency symbol/whitespace around the number.
func normalizeAmount(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "") // Non-breaking space, Excel's usual thousands separator.
	if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", ".")
	} else {
		s = strings.ReplaceAll(s, ",", "")
	}
	s = strings.TrimLeft(s, "+")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", err
	}
	if v < 0 {
		v = -v // Sign is carried by transaction type, not the amount.
	}
	return strconv.FormatFloat(v, 'f', 2, 64), nil
}
