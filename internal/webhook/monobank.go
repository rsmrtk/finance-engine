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
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/rsmrtk/finance-engine/internal/monobankimport"
	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/logger"
	"github.com/rsmrtk/finance-engine/pkg/monobank"
)

type MonobankHandler struct {
	connections *repository.MonobankRepository
	importer    *monobankimport.Importer
	log         logger.Logger
}

func NewMonobankHandler(
	connections *repository.MonobankRepository,
	transactions *repository.TransactionRepository,
	categories *repository.CategoryRepository,
	log logger.Logger,
) *MonobankHandler {
	return &MonobankHandler{connections: connections, importer: monobankimport.New(transactions, categories, log), log: log}
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
	accountIndex := slices.Index(conn.AccountIDs, payload.Data.Account)
	if accountIndex == -1 {
		return nil // An account on the same token the user didn't select to track.
	}

	var accountCurrency string
	if accountIndex < len(conn.AccountCurrencies) {
		accountCurrency = conn.AccountCurrencies[accountIndex]
	}
	if _, err := h.importer.Item(ctx, conn.UserID, item, accountCurrency); err != nil {
		return err
	}

	return h.connections.TouchSync(ctx, conn.UserID)
}
