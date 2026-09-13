// Package liqpay implements just enough of LiqPay's API (checkout
// requests, server-to-server calls, callback verification) for the Max
// plan's 7-day trial + monthly subscription. The request/signature
// scheme is cross-checked against LiqPay's own official Go and PHP SDKs
// (github.com/liqpay/sdk-go, github.com/liqpay/sdk-php) rather than
// scraped documentation, which gave conflicting (and wrong, for the
// callback signature) results.
package liqpay

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiBaseURL  = "https://www.liqpay.ua/api"
	CheckoutURL = apiBaseURL + "/3/checkout"
	dateTimeFmt = "2006-01-02 15:04:05" // LiqPay's native subscribe_date_start format.
	Version     = 3
	ActionSub   = "subscribe"
	ActionUnsub = "unsubscribe"
	Periodicity = "month"
)

// Callback status values, decoded from a webhook's data field.
const (
	StatusSubscribed   = "subscribed"   // Card tokenized, subscription created — trial started.
	StatusSuccess      = "success"      // A charge (trial conversion or renewal) succeeded.
	StatusFailure      = "failure"      // A charge failed.
	StatusError        = "error"        // Malformed request.
	StatusUnsubscribed = "unsubscribed" // Subscription canceled (by us or LiqPay).
)

type Client struct {
	publicKey  string
	privateKey string
	sandbox    bool
	httpClient *http.Client
}

func New(publicKey, privateKey string, sandbox bool) *Client {
	return &Client{
		publicKey:  publicKey,
		privateKey: privateKey,
		sandbox:    sandbox,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Sign matches LiqPay's str_to_sign exactly: base64(sha1_raw(private_key
// + data + private_key)) — raw binary SHA1 output, not hex, before
// base64 encoding. Confirmed against both official SDKs.
func (c *Client) Sign(data string) string {
	h := sha1.New()
	h.Write([]byte(c.privateKey))
	h.Write([]byte(data))
	h.Write([]byte(c.privateKey))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// encode base64-JSON-encodes params, injecting public_key/version/sandbox
// so every call site doesn't have to remember to.
func (c *Client) encode(params map[string]any) (string, error) {
	params["public_key"] = c.publicKey
	params["version"] = Version
	if c.sandbox {
		params["sandbox"] = 1
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal liqpay params: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// CheckoutFields returns the data+signature pair the frontend embeds in a
// hidden form POSTing to CheckoutURL — this is a browser redirect flow
// (LiqPay's own hosted card page), so the private key never touches the
// browser, only these two opaque strings do.
func (c *Client) CheckoutFields(params map[string]any) (data, signature string, err error) {
	data, err = c.encode(params)
	if err != nil {
		return "", "", err
	}
	return data, c.Sign(data), nil
}

// Callback is the decoded contents of a webhook's `data` field — only
// the fields billing.Service actually reads.
type Callback struct {
	Action         string  `json:"action"`
	Status         string  `json:"status"`
	OrderID        string  `json:"order_id"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	ErrDescription string  `json:"err_description"`
}

// VerifyAndDecode checks the callback's signature (rejecting anything
// that doesn't match — a forged webhook could otherwise grant a free
// subscription) and decodes the payload.
func (c *Client) VerifyAndDecode(data, signature string) (Callback, error) {
	if c.Sign(data) != signature {
		return Callback{}, fmt.Errorf("invalid liqpay callback signature")
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return Callback{}, fmt.Errorf("decode callback data: %w", err)
	}
	var cb Callback
	if err := json.Unmarshal(raw, &cb); err != nil {
		return Callback{}, fmt.Errorf("unmarshal callback data: %w", err)
	}
	return cb, nil
}

// Request makes a server-to-server API call (e.g. action=unsubscribe) —
// as opposed to CheckoutFields, which builds a browser-redirect form.
func (c *Client) Request(ctx context.Context, params map[string]any) (map[string]any, error) {
	data, err := c.encode(params)
	if err != nil {
		return nil, err
	}
	signature := c.Sign(data)

	form := url.Values{"data": {data}, "signature": {signature}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/request", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build liqpay request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call liqpay: %w", err)
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode liqpay response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("liqpay returned status %d: %v", resp.StatusCode, out["err_description"])
	}
	return out, nil
}

// FormatSubscribeDate renders t in LiqPay's native subscribe_date_start
// format ("YYYY-MM-DD HH:mm:ss").
func FormatSubscribeDate(t time.Time) string {
	return t.Format(dateTimeFmt)
}
