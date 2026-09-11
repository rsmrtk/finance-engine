// Package webhook handles inbound HTTP callbacks from third parties
// (currently just Monobank), separate from the gRPC API surface since
// Monobank speaks plain HTTP/JSON, not gRPC.
package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/logger"
	"github.com/rsmrtk/finance-engine/pkg/mcc"
	"github.com/rsmrtk/finance-engine/pkg/monobank"
)

type MonobankHandler struct {
	connections  *repository.MonobankRepository
	transactions *repository.TransactionRepository
	categories   *repository.CategoryRepository
	log          logger.Logger
}

func NewMonobankHandler(
	connections *repository.MonobankRepository,
	transactions *repository.TransactionRepository,
	categories *repository.CategoryRepository,
	log logger.Logger,
) *MonobankHandler {
	return &MonobankHandler{connections: connections, transactions: transactions, categories: categories, log: log}
}

// ServeHTTP handles POST /webhooks/monobank/{secret}. The secret is an
// unguessable 256-bit token generated when the user connected their
// account (see internal/service/monobank) — Monobank's personal API
// doesn't sign webhook payloads, so this is the auth mechanism.
func (h *MonobankHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	secret := strings.TrimPrefix(r.URL.Path, "/webhooks/monobank/")
	if secret == "" {
		http.Error(w, "missing webhook secret", http.StatusNotFound)
		return
	}

	var payload monobank.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Monobank also sends a "verify" ping with no statement item when a
	// webhook URL is first registered; nothing to do but 200 it.
	if payload.Type != "StatementItem" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.process(r.Context(), secret, payload); err != nil {
		h.log.Error("failed to process monobank webhook", logger.H{"error": err.Error()})
		// Still 200: Monobank retries on non-2xx, and most failures here
		// (unknown secret, bad payload) won't be fixed by a retry.
	}

	w.WriteHeader(http.StatusOK)
}

func (h *MonobankHandler) process(ctx context.Context, secret string, payload monobank.WebhookPayload) error {
	conn, err := h.connections.GetByWebhookSecret(ctx, secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("unknown webhook secret")
		}
		return fmt.Errorf("lookup connection: %w", err)
	}

	item := payload.Data.StatementItem
	if payload.Data.Account != conn.AccountID {
		return nil // A different account on the same token; not the one we track.
	}
	if item.Amount == 0 {
		return nil
	}

	transactionType := "income"
	amount := item.Amount
	if amount < 0 {
		transactionType = "expense"
		amount = -amount
	}

	categoryID := uuid.Nil
	if name := mcc.CategoryName(item.MCC); name != "" {
		if category, ok := h.findCategory(ctx, conn.UserID, name, transactionType); ok {
			categoryID = category
		}
	}

	_, err = h.transactions.Create(ctx, repository.CreateTransactionParams{
		UserID:     conn.UserID,
		CategoryID: categoryID,
		Amount:     fmt.Sprintf("%d.%02d", amount/100, amount%100),
		Currency:   monobank.CurrencyCode(item.CurrencyCode),
		Type:       transactionType,
		Date:       time.Unix(item.Time, 0),
		Note:       item.Description,
	})
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	return h.connections.TouchSync(ctx, conn.UserID)
}

func (h *MonobankHandler) findCategory(ctx context.Context, userID uuid.UUID, name, transactionType string) (uuid.UUID, bool) {
	categories, err := h.categories.ListForUser(ctx, userID)
	if err != nil {
		return uuid.Nil, false
	}
	for _, category := range categories {
		if category.Name == name && category.Type == transactionType {
			return category.ID, true
		}
	}
	return uuid.Nil, false
}
