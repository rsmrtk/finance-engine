// Package translate calls Google's free, keyless "gtx" translation
// endpoint — the same undocumented one translate.google.com's own web
// client uses. No API key, no billing, no account needed; the trade-off
// is that it's unofficial and could change or rate-limit without notice.
// Every caller treats a failure as "no translation available" and falls
// back to the original text, so an outage here degrades gracefully
// instead of breaking anything.
package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const endpoint = "https://translate.googleapis.com/translate_a/single"

// httpClient has an explicit timeout — the shared cluster egress IP is
// currently blocked by Google, and a hung TCP connection to a black hole
// would otherwise stall every caller (category listing, in particular)
// far longer than a clean, fast failure does.
var httpClient = &http.Client{Timeout: 3 * time.Second}

// Circuit breaker: once the endpoint fails once, every category list
// request would otherwise retry it up to 2x per category (uk+en) — with
// 13 default categories that's 26 outbound calls per request. While the
// endpoint is known-down, skip the network call entirely and fail fast
// for cooldown, re-probing only after it expires.
const cooldown = 10 * time.Minute

var (
	mu            sync.Mutex
	cooldownUntil time.Time
)

func circuitOpen() bool {
	mu.Lock()
	defer mu.Unlock()
	return time.Now().Before(cooldownUntil)
}

func tripCircuit() {
	mu.Lock()
	cooldownUntil = time.Now().Add(cooldown)
	mu.Unlock()
}

func resetCircuit() {
	mu.Lock()
	cooldownUntil = time.Time{}
	mu.Unlock()
}

// Translate auto-detects the source language and translates text into
// targetLang (e.g. "en", "uk"). Empty input returns empty output.
func Translate(ctx context.Context, text, targetLang string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if circuitOpen() {
		return "", fmt.Errorf("translate endpoint in cooldown after a recent failure")
	}

	params := url.Values{
		"client": {"gtx"},
		"sl":     {"auto"},
		"tl":     {targetLang},
		"dt":     {"t"},
		"q":      {text},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("build translate request: %w", err)
	}
	// Without a browser-like User-Agent this endpoint answers with either
	// a 400 or a 429 almost immediately — it's gating on more than just
	// rate, apparently also on looking like a real client.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		tripCircuit()
		return "", fmt.Errorf("call translate endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tripCircuit()
		return "", fmt.Errorf("translate endpoint returned status %d", resp.StatusCode)
	}
	resetCircuit()

	// Response shape: [[["translated chunk","source chunk",null,null,...], ...], null, "detected_lang", ...]
	// Long input can be split into multiple sentence chunks — join them.
	var raw []any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", fmt.Errorf("decode translate response: %w", err)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("empty translate response")
	}
	segments, ok := raw[0].([]any)
	if !ok {
		return "", fmt.Errorf("unexpected translate response shape")
	}

	var sb strings.Builder
	for _, seg := range segments {
		segArr, ok := seg.([]any)
		if !ok || len(segArr) == 0 {
			continue
		}
		if s, ok := segArr[0].(string); ok {
			sb.WriteString(s)
		}
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("translate returned no text")
	}
	return sb.String(), nil
}
