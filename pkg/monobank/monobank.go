// Package monobank is a minimal client for the parts of Monobank's Personal
// API (https://api.monobank.ua/docs/) that this app needs: validating a
// personal token and registering a webhook for push transaction updates.
package monobank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://api.monobank.ua"

type Client struct {
	httpClient *http.Client
}

func New() *Client {
	return &Client{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

type Account struct {
	ID           string   `json:"id"`
	CurrencyCode int      `json:"currencyCode"`
	MaskedPan    []string `json:"maskedPan"`
	Type         string   `json:"type"`
}

type ClientInfo struct {
	ClientID   string    `json:"clientId"`
	Name       string    `json:"name"`
	WebHookURL string    `json:"webHookUrl"`
	Accounts   []Account `json:"accounts"`
}

// ClientInfo validates the personal token and returns the account list.
// Monobank returns 403 for an invalid/revoked token.
func (c *Client) ClientInfo(ctx context.Context, personalToken string) (*ClientInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/personal/client-info", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Token", personalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call monobank: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("monobank returned status %d: %s", resp.StatusCode, string(body))
	}

	var info ClientInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode client-info: %w", err)
	}
	return &info, nil
}

// Statement fetches an account's transaction history directly (as
// opposed to waiting for a webhook push) — the fallback for catching up
// on whatever a webhook missed while our server was unreachable (e.g.
// the dev machine was asleep). Monobank caps the range at 31 days and
// rate-limits this endpoint the same way as ClientInfo (~1 request per
// token per 60 seconds) — callers syncing several accounts must space
// the calls out themselves.
func (c *Client) Statement(ctx context.Context, personalToken, accountID string, from, to time.Time) ([]StatementItem, error) {
	url := fmt.Sprintf("%s/personal/statement/%s/%d/%d", baseURL, accountID, from.Unix(), to.Unix())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Token", personalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call monobank: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("monobank returned status %d: %s", resp.StatusCode, string(body))
	}

	var items []StatementItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decode statement: %w", err)
	}
	return items, nil
}

// SetWebHook registers the URL Monobank will POST new transactions to.
func (c *Client) SetWebHook(ctx context.Context, personalToken, webhookURL string) error {
	payload, err := json.Marshal(map[string]string{"webHookUrl": webhookURL})
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/personal/webhook", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Token", personalToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call monobank: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("monobank returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
