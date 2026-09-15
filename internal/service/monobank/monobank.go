package monobank

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rsmrtk/finance-engine/internal/monobankimport"
	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/cryptobox"
	"github.com/rsmrtk/finance-engine/pkg/monobank"
)

type Service struct {
	connections    *repository.MonobankRepository
	client         *monobank.Client
	box            *cryptobox.Box
	importer       *monobankimport.Importer
	webhookBaseURL string // e.g. "https://finance-engine-api.onrender.com"; must be public HTTPS.
}

func New(connections *repository.MonobankRepository, client *monobank.Client, box *cryptobox.Box, importer *monobankimport.Importer, webhookBaseURL string) *Service {
	return &Service{connections: connections, client: client, box: box, importer: importer, webhookBaseURL: webhookBaseURL}
}

type Status struct {
	IsConnected  bool
	MaskedPans   []string
	ConnectedAt  time.Time
	LastSyncedAt time.Time // Zero value if never synced.
}

// AccountOption is one card/jar under a personal token, shown to the user
// so they can pick which one to track — Monobank's API returns every
// account (different currencies, cards, and jars all get their own
// entry), and there's no reliable way to guess which one the user
// actually wants without asking them.
type AccountOption struct {
	ID        string
	MaskedPan string
	Currency  string
	Type      string // Monobank's raw type: "black", "white", "platinum", "iron", "fop", "yellow", "jar", etc.
	Selected  bool   // Only meaningful from ListMyAccounts — true if already tracked.
}

// ListAccounts validates the personal token against Monobank and returns
// every account under it, so the frontend can show a picker instead of
// us guessing which one to track.
func (s *Service) ListAccounts(ctx context.Context, personalToken string) ([]AccountOption, error) {
	if personalToken == "" {
		return nil, fmt.Errorf("personal_token is required")
	}
	info, err := s.client.ClientInfo(ctx, personalToken)
	if err != nil {
		return nil, fmt.Errorf("validate token with monobank: %w", err)
	}
	options := make([]AccountOption, 0, len(info.Accounts))
	for _, a := range info.Accounts {
		pan := ""
		if len(a.MaskedPan) > 0 {
			pan = a.MaskedPan[0]
		}
		options = append(options, AccountOption{
			ID:        a.ID,
			MaskedPan: pan,
			Currency:  monobank.CurrencyCode(a.CurrencyCode),
			Type:      a.Type,
		})
	}
	return options, nil
}

// Connect registers our webhook so new transactions push to us
// automatically, and stores the token encrypted (it's effectively a
// bearer credential for the account). accountIDs/maskedPans (parallel,
// same order) come from a prior ListAccounts call the caller already
// made — Connect deliberately does NOT call ClientInfo again to
// re-verify them: Monobank rate-limits /personal/client-info to roughly
// one call per token per 60 seconds, and a user picking cards takes less
// time than that, so a second call here reliably 429s and silently
// breaks every connection attempt. SetWebHook (a different, separately
// -limited endpoint) still fails loudly on a bad or revoked token, so
// token validity isn't lost by skipping the recheck.
func (s *Service) Connect(ctx context.Context, userID uuid.UUID, personalToken string, accountIDs, maskedPans, accountTypes, accountCurrencies []string) (Status, error) {
	if personalToken == "" {
		return Status{}, fmt.Errorf("personal_token is required")
	}
	if len(accountIDs) == 0 {
		return Status{}, fmt.Errorf("at least one account_id is required")
	}
	if s.webhookBaseURL == "" {
		return Status{}, fmt.Errorf("PUBLIC_BASE_URL is not configured on the server; Monobank needs a public HTTPS URL to send transactions to")
	}

	secret, err := randomSecret()
	if err != nil {
		return Status{}, fmt.Errorf("generate webhook secret: %w", err)
	}

	encrypted, err := s.box.Encrypt(personalToken)
	if err != nil {
		return Status{}, fmt.Errorf("encrypt token: %w", err)
	}

	webhookURL := s.webhookBaseURL + "/webhooks/monobank/" + secret
	if err := s.client.SetWebHook(ctx, personalToken, webhookURL); err != nil {
		return Status{}, fmt.Errorf("register webhook: %w", err)
	}

	conn, err := s.connections.Upsert(ctx, repository.UpsertMonobankConnectionParams{
		UserID:            userID,
		EncryptedToken:    encrypted,
		WebhookSecret:     secret,
		MaskedPans:        maskedPans,
		AccountIDs:        accountIDs,
		AccountTypes:      accountTypes,
		AccountCurrencies: accountCurrencies,
	})
	if err != nil {
		return Status{}, fmt.Errorf("save connection: %w", err)
	}

	return statusFromConnection(conn), nil
}

// ListMyAccounts is ListAccounts for an already-connected user editing
// which cards to track — it decrypts the token we already stored instead
// of asking for it again, and flags which accounts are currently tracked
// so the frontend can pre-check them in the same picker UI used at
// connect time.
func (s *Service) ListMyAccounts(ctx context.Context, userID uuid.UUID) ([]AccountOption, error) {
	conn, err := s.connections.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("monobank is not connected")
		}
		return nil, fmt.Errorf("lookup connection: %w", err)
	}
	token, err := s.box.Decrypt(conn.EncryptedToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}
	options, err := s.ListAccounts(ctx, token)
	if err != nil {
		return nil, err
	}
	tracked := make(map[string]bool, len(conn.AccountIDs))
	for _, id := range conn.AccountIDs {
		tracked[id] = true
	}
	for i := range options {
		options[i].Selected = tracked[options[i].ID]
	}
	return options, nil
}

// UpdateAccounts changes which accounts an already-connected user tracks
// — no Monobank call needed (the webhook is already registered for the
// whole token, not per-account), just which account ids our own webhook
// filter accepts.
func (s *Service) UpdateAccounts(ctx context.Context, userID uuid.UUID, accountIDs, maskedPans, accountTypes, accountCurrencies []string) (Status, error) {
	if len(accountIDs) == 0 {
		return Status{}, fmt.Errorf("at least one account_id is required")
	}
	conn, err := s.connections.UpdateAccounts(ctx, userID, accountIDs, maskedPans, accountTypes, accountCurrencies)
	if err != nil {
		return Status{}, fmt.Errorf("update tracked accounts: %w", err)
	}
	return statusFromConnection(conn), nil
}

// SyncNow pulls the tracked account's last 31 days of statement directly
// from Monobank and imports anything the real-time webhook missed — the
// dev machine being asleep/offline when a transaction fired is the
// common case. Idempotent via external_id (internal/monobankimport), so
// clicking it repeatedly never creates duplicates.
//
// Monobank rate-limits /personal/statement the same way as
// /personal/client-info (~1 request per token per 60 seconds) — with
// more than one tracked account under the same token, only the first
// gets synced per call; syncing the rest just means clicking again a
// minute later.
func (s *Service) SyncNow(ctx context.Context, userID uuid.UUID) (imported int, err error) {
	conn, err := s.connections.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("monobank is not connected")
		}
		return 0, fmt.Errorf("lookup connection: %w", err)
	}
	if len(conn.AccountIDs) == 0 {
		return 0, fmt.Errorf("no accounts are being tracked")
	}
	token, err := s.box.Decrypt(conn.EncryptedToken)
	if err != nil {
		return 0, fmt.Errorf("decrypt token: %w", err)
	}

	to := time.Now()
	from := to.AddDate(0, 0, -31) // Monobank's own cap on this endpoint's range.

	items, err := s.client.Statement(ctx, token, conn.AccountIDs[0], from, to)
	if err != nil {
		return 0, fmt.Errorf("fetch statement: %w", err)
	}

	// SyncNow only ever fetches conn.AccountIDs[0] (see the rate-limit
	// comment above), so the account-level currency to cross-check
	// against is always at the same index.
	var accountCurrency string
	if len(conn.AccountCurrencies) > 0 {
		accountCurrency = conn.AccountCurrencies[0]
	}
	for _, item := range items {
		created, itemErr := s.importer.Item(ctx, userID, item, accountCurrency)
		if itemErr != nil {
			return imported, fmt.Errorf("import transaction: %w", itemErr)
		}
		if created {
			imported++
		}
	}

	if err := s.connections.TouchSync(ctx, userID); err != nil {
		return imported, fmt.Errorf("touch sync: %w", err)
	}
	return imported, nil
}

func (s *Service) Status(ctx context.Context, userID uuid.UUID) (Status, error) {
	conn, err := s.connections.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Status{IsConnected: false}, nil
		}
		return Status{}, fmt.Errorf("lookup connection: %w", err)
	}
	return statusFromConnection(conn), nil
}

func (s *Service) Disconnect(ctx context.Context, userID uuid.UUID) error {
	return s.connections.Delete(ctx, userID)
}

func statusFromConnection(conn repository.MonobankConnection) Status {
	return Status{
		IsConnected:  true,
		MaskedPans:   conn.MaskedPans,
		ConnectedAt:  conn.ConnectedAt,
		LastSyncedAt: conn.LastSyncedAt,
	}
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
