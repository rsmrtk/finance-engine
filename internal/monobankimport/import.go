// Package monobankimport turns one Monobank StatementItem into a
// transaction — shared by the real-time webhook (internal/webhook) and
// the manual "sync now" backfill (internal/service/monobank), so a
// transaction imported either way gets identical categorization and the
// same dedup guarantee (external_id) rather than two logics drifting
// apart over time.
package monobankimport

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/logger"
	"github.com/rsmrtk/finance-engine/pkg/mcc"
	"github.com/rsmrtk/finance-engine/pkg/monobank"
)

type Importer struct {
	transactions *repository.TransactionRepository
	categories   *repository.CategoryRepository
	log          logger.Logger
}

func New(transactions *repository.TransactionRepository, categories *repository.CategoryRepository, log logger.Logger) *Importer {
	return &Importer{transactions: transactions, categories: categories, log: log}
}

// Item imports one StatementItem for userID. Every import lands as a
// real income/expense (IsInternalTransfer always false) — this app used
// to guess which transactions were money moving between the user's own
// cards/jars from Monobank's description text and from same-amount
// pairing, but that guessing kept getting confidently confirmed and then
// un-confirmed against real data across this account (card-color
// mentions for untracked cards, "for transfer to card" FOP legs that
// turned out to be real debits, matched pairs with mismatched currency
// labels...). The account owner now marks a transaction as a transfer
// themselves via the edit modal's "Не відображати" toggle
// (repository.UpdateTransactionParams.IsInternalTransfer) — a manual
// decision beats another automatic guess. created is false (with a nil
// error) when this exact item (by external_id) was already imported —
// callers should treat that as "nothing to do", not a failure.
//
// accountCurrency is what the user's own Monobank integration says this
// specific account's currency actually is (from the account picker,
// stored on the connection — see MonobankConnection.AccountCurrencies).
// "" when unknown (never selected/backfilled yet, or the iOS gRPC path
// which doesn't send this metadata at all) skips the check entirely.
func (im *Importer) Item(ctx context.Context, userID uuid.UUID, item monobank.StatementItem, accountCurrency string) (created bool, err error) {
	if item.Amount == 0 {
		return false, nil
	}

	transactionType := "income"
	amount := item.Amount
	if amount < 0 {
		transactionType = "expense"
		amount = -amount
	}
	amountStr := fmt.Sprintf("%d.%02d", amount/100, amount%100)
	currency := monobank.CurrencyCode(item.CurrencyCode)

	// Monobank has been confirmed (via operationAmount cross-checking —
	// e.g. an "amount" of 20799.75 tagged currencyCode=EUR paired with an
	// operationAmount of -40000, a ~52x ratio matching the real UAH/EUR
	// rate) to sometimes report a foreign merchant's own currency in
	// currencyCode for international transactions (TRANSAVIA, Carrefour,
	// a European airport lounge, ...) even though `amount` itself is
	// already correctly expressed in the TRACKED ACCOUNT's real currency,
	// not the merchant's. Per-transaction currencyCode is proven
	// unreliable for this account; what the user themselves confirmed
	// this account to be (accountCurrency) is not.
	mismatch := accountCurrency != "" && currency != accountCurrency
	if mismatch {
		currency = accountCurrency
	}

	// Logged here (not just in the webhook handler) so the manual "sync
	// now" path — which calls Item() directly and never goes through the
	// webhook — gets the same diagnostic trail.
	if im.log != nil {
		im.log.Info("importing monobank transaction", logger.H{
			"externalId":                    item.ID,
			"mcc":                           item.MCC,
			"currencyCode":                  item.CurrencyCode,
			"currency":                      currency,
			"amount":                        amountStr,
			"operationAmount":               item.OperationAmount,
			"operationCurrencyCode":         item.OperationCurrencyCode,
			"type":                          transactionType,
			"description":                   item.Description,
			"currencyOverriddenFromAccount": mismatch,
		})
	}

	name := mcc.CategoryName(item.MCC)
	if name == "" && transactionType == "expense" {
		// No MCC match (common for transfers/top-ups, which carry no MCC
		// at all) — fall back to the catch-all category instead of
		// leaving it blank, so there's at least something to filter/sort
		// by without editing every single import.
		name = "Інше"
	}
	categoryID := uuid.Nil
	if name != "" {
		if category, ok := im.findCategory(ctx, userID, name, transactionType); ok {
			categoryID = category
		}
	}

	date := time.Unix(item.Time, 0)
	_, created, err = im.transactions.CreateWithExternalID(ctx, repository.CreateTransactionWithExternalIDParams{
		UserID:                userID,
		CategoryID:            categoryID,
		Amount:                amountStr,
		Currency:              currency,
		Type:                  transactionType,
		Date:                  date,
		Note:                  item.Description,
		ExternalID:            item.ID,
		OperationAmount:       item.OperationAmount,
		OperationCurrencyCode: item.OperationCurrencyCode,
	})
	if err != nil {
		return false, fmt.Errorf("create transaction: %w", err)
	}
	return created, nil
}

func (im *Importer) findCategory(ctx context.Context, userID uuid.UUID, name, transactionType string) (uuid.UUID, bool) {
	categories, err := im.categories.ListForUser(ctx, userID)
	if err != nil {
		return uuid.Nil, false
	}
	for _, category := range categories {
		if strings.EqualFold(strings.TrimSpace(category.Name), name) && category.Type == transactionType {
			return category.ID, true
		}
	}
	return uuid.Nil, false
}
