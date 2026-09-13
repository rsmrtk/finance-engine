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

// transferMatchWindow is how close in time the two sides of a real
// transfer between the user's own accounts are expected to post — both
// legs of a jar top-up or a white<->platinum<->FOP move land within
// seconds of each other in practice, so this stays tight on purpose: a
// wider window would risk pairing up two unrelated same-amount
// transactions (e.g. a purchase and an unrelated refund).
const transferMatchWindow = 3 * time.Minute

// Item imports one StatementItem for userID. created is false (with a
// nil error) when this exact item (by external_id) was already
// imported — callers should treat that as "nothing to do", not a failure.
func (im *Importer) Item(ctx context.Context, userID uuid.UUID, item monobank.StatementItem) (created bool, err error) {
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

	// Logged here (not just in the webhook handler) so the manual "sync
	// now" path — which calls Item() directly and never goes through the
	// webhook — gets the same diagnostic trail. This is what would have
	// caught the currency-mismatch bug immediately instead of requiring a
	// live DB query to prove it after the fact.
	if im.log != nil {
		im.log.Info("importing monobank transaction", logger.H{
			"externalId":   item.ID,
			"mcc":          item.MCC,
			"currencyCode": item.CurrencyCode,
			"currency":     currency,
			"amount":       amountStr,
			"type":         transactionType,
			"description":  item.Description,
		})
	}

	// Money moving between the user's own Monobank cards/jars (white <->
	// platinum <-> FOP <-> a jar) isn't real income or a real expense —
	// same money, different pocket. A jar top-up/withdrawal gives itself
	// away in the description (Monobank's own UI puts the jar name in
	// «guillemets» after "Поповнення"); other own-account transfers are
	// caught below by matching against the opposite leg once it exists.
	isTransfer := looksLikeInternalTransfer(item.Description)

	categoryID := uuid.Nil
	if !isTransfer {
		name := mcc.CategoryName(item.MCC)
		if name == "" && transactionType == "expense" {
			// No MCC match (common for transfers/top-ups, which carry no
			// MCC at all) — fall back to the catch-all category instead
			// of leaving it blank, so there's at least something to
			// filter/sort by without editing every single import.
			name = "Інше"
		}
		if name != "" {
			if category, ok := im.findCategory(ctx, userID, name, transactionType); ok {
				categoryID = category
			}
		}
	}

	date := time.Unix(item.Time, 0)
	tx, created, err := im.transactions.CreateWithExternalID(ctx, repository.CreateTransactionWithExternalIDParams{
		UserID:             userID,
		CategoryID:         categoryID,
		Amount:             amountStr,
		Currency:           currency,
		Type:               transactionType,
		Date:               date,
		Note:               item.Description,
		ExternalID:         item.ID,
		IsInternalTransfer: isTransfer,
	})
	if err != nil {
		return false, fmt.Errorf("create transaction: %w", err)
	}
	if !created {
		return false, nil // Already imported — nothing left to do.
	}

	im.matchTransfer(ctx, userID, tx, transactionType, amountStr)
	return created, nil
}

// matchTransfer looks for the opposite leg of a transfer between the
// user's own tracked accounts — same amount, opposite type, posted
// within transferMatchWindow — and flags both sides if found.
// Best-effort: a failed lookup just leaves the transaction as a normal
// income/expense rather than blocking the import. Deliberately not
// matched on currency too: Monobank has been observed reporting the two
// legs of the same real transfer with mismatched currency labels (see
// TransactionFindTransferMatch).
func (im *Importer) matchTransfer(ctx context.Context, userID uuid.UUID, tx repository.Transaction, transactionType, amountStr string) {
	im.matchTransferPair(ctx, userID, tx, transactionType, amountStr)
}

// matchTransferPair is matchTransfer, additionally reporting the matched
// transaction's id — used by ReclassifyExisting to avoid reprocessing
// the second leg of a pair later in the same backfill pass.
func (im *Importer) matchTransferPair(ctx context.Context, userID uuid.UUID, tx repository.Transaction, transactionType, amountStr string) (uuid.UUID, bool) {
	oppositeType := "income"
	if transactionType == "income" {
		oppositeType = "expense"
	}
	match, ok, err := im.transactions.FindTransferMatch(
		ctx, userID, oppositeType, amountStr,
		tx.Date.Add(-transferMatchWindow), tx.Date.Add(transferMatchWindow),
	)
	if err != nil || !ok || match.ID == tx.ID {
		return uuid.Nil, false
	}
	_ = im.transactions.MarkInternalTransfer(ctx, tx.ID)
	_ = im.transactions.MarkInternalTransfer(ctx, match.ID)
	return match.ID, true
}

// looksLikeInternalTransfer recognizes Monobank's own descriptions for
// money moving between the user's own cards/jars, confirmed against real
// synced data: jar top-ups/withdrawals ("Поповнення «...»",
// "Часткове зняття «...»") and named-card transfers ("З Білої картки",
// "На платинову картку", ...) — Monobank's card-color names (White,
// Platinum, EUR, USD) only ever refer to the user's OWN other cards, so
// these are unambiguous. Deliberately NOT matched here: a generic
// "Переказ на картку"/"Від: <name>" with no card-color qualifier — those
// also appear on genuine incoming/outgoing payments to other people (a
// friend paying back a split bill would say "Від: <their name>" too), so
// hiding every one of those as a transfer would wrongly erase real
// income/expense. Those cases are instead caught by pair-matching
// against the opposite leg (see matchTransferPair) once both sides of an
// actual self-transfer exist.
func looksLikeInternalTransfer(description string) bool {
	lower := strings.ToLower(description)
	if strings.ContainsAny(description, "«»") &&
		(strings.Contains(lower, "поповнення") || strings.Contains(lower, "зняття")) {
		return true
	}
	switch {
	case strings.HasPrefix(lower, "з білої картки"),
		strings.HasPrefix(lower, "з платинової картки"),
		strings.HasPrefix(lower, "з єврової картки"),
		strings.HasPrefix(lower, "з доларової картки"),
		strings.HasPrefix(lower, "на білу картку"),
		strings.HasPrefix(lower, "на платинову картку"),
		strings.HasPrefix(lower, "на єврову картку"),
		strings.HasPrefix(lower, "на доларову картку"):
		return true
	}
	return false
}

// ReclassifyExisting re-runs internal-transfer detection over every
// already-imported (external_id set) transaction — the one-time backfill
// for data imported before this detection existed. Cheap enough to run
// on every "sync now" click: personal transaction volumes are small, and
// MarkInternalTransfer/FindTransferMatch are no-ops once everything is
// already flagged correctly.
func (im *Importer) ReclassifyExisting(ctx context.Context, userID uuid.UUID) (updated int, err error) {
	all, err := im.transactions.ListForUser(ctx, userID, time.Time{}, time.Now())
	if err != nil {
		return 0, fmt.Errorf("list transactions: %w", err)
	}
	// matchTransfer flags both legs of a pair at once — track which ids
	// this pass already handled so the loop doesn't reprocess (and
	// double-count) the second leg once it comes up on its own turn,
	// since the in-memory copy from ListForUser predates that update.
	handled := make(map[uuid.UUID]bool)
	for _, tx := range all {
		if tx.ExternalID == "" || tx.IsInternalTransfer || handled[tx.ID] {
			continue
		}
		if looksLikeInternalTransfer(tx.Note) {
			if err := im.transactions.MarkInternalTransfer(ctx, tx.ID); err == nil {
				updated++
				handled[tx.ID] = true
			}
			continue
		}
		if match, ok := im.matchTransferPair(ctx, userID, tx, tx.Type, tx.Amount); ok {
			updated++
			handled[tx.ID] = true
			handled[match] = true
		}
	}
	return updated, nil
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
