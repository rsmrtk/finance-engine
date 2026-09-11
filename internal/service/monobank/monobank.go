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

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/cryptobox"
	"github.com/rsmrtk/finance-engine/pkg/monobank"
)

type Service struct {
	connections    *repository.MonobankRepository
	client         *monobank.Client
	box            *cryptobox.Box
	webhookBaseURL string // e.g. "https://finance-engine-api.onrender.com"; must be public HTTPS.
}

func New(connections *repository.MonobankRepository, client *monobank.Client, box *cryptobox.Box, webhookBaseURL string) *Service {
	return &Service{connections: connections, client: client, box: box, webhookBaseURL: webhookBaseURL}
}

type Status struct {
	IsConnected  bool
	MaskedPan    string
	ConnectedAt  time.Time
	LastSyncedAt time.Time // Zero value if never synced.
}

// Connect validates the personal token against Monobank, registers our
// webhook so new transactions push to us automatically, and stores the
// token encrypted (it's effectively a bearer credential for the account).
func (s *Service) Connect(ctx context.Context, userID uuid.UUID, personalToken string) (Status, error) {
	if personalToken == "" {
		return Status{}, fmt.Errorf("personal_token is required")
	}
	if s.webhookBaseURL == "" {
		return Status{}, fmt.Errorf("PUBLIC_BASE_URL is not configured on the server; Monobank needs a public HTTPS URL to send transactions to")
	}

	info, err := s.client.ClientInfo(ctx, personalToken)
	if err != nil {
		return Status{}, fmt.Errorf("validate token with monobank: %w", err)
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
		UserID:         userID,
		EncryptedToken: encrypted,
		WebhookSecret:  secret,
		MaskedPan:      info.FirstMaskedPan(),
		AccountID:      info.PrimaryAccountID(),
	})
	if err != nil {
		return Status{}, fmt.Errorf("save connection: %w", err)
	}

	return statusFromConnection(conn), nil
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
		MaskedPan:    conn.MaskedPan,
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
