// Package ollama calls a self-hosted Ollama instance's chat API — this is
// the free, private path for the financial advisor: no API key, no
// request limits, and the user's income/expense data never leaves their
// own machine. In kind, the backend pod reaches it via
// host.docker.internal, since Ollama runs natively on the host (for
// Metal GPU acceleration on macOS) rather than as a container.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Message struct {
	Role    string `json:"role"` // "system", "user", or "assistant".
	Content string `json:"content"`
}

type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func New(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		// Generation on CPU/Metal for an 8B model can take tens of seconds
		// for a longer reply — far past a typical API timeout, but this is
		// always an explicit, waited-for user action (chat send, or a
		// Redis-cached insights fetch), never a blocking hot path.
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type chatResponse struct {
	Message Message `json:"message"`
}

// Chat sends the full message history (system + prior turns + the new
// user message) and returns the assistant's reply text.
func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: c.model, Messages: messages, Stream: false})
	if err != nil {
		return "", fmt.Errorf("encode ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d — is `ollama serve` running with OLLAMA_HOST=0.0.0.0?", resp.StatusCode)
	}

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	return out.Message.Content, nil
}
