package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Config struct {
	GRPCAddress string
	PostgresDSN string
	// RedisAddr backs the exchange-rates cache (internal/repository/rate.go)
	// — reference/statistics data, deliberately kept out of Postgres so
	// the relational DB stays scoped to actual user data.
	RedisAddr      string
	JWTSecret      string
	JWTDuration    time.Duration
	AppleBundleID  string
	NBUExchangeURL string
	// DevMode enables AuthService.DevSignIn, a Sign in with Apple bypass for
	// local testing without a paid Apple Developer account. Never set this
	// on a real deployment.
	DevMode bool

	// WebhookAddress is where the plain-HTTP webhook server listens (Monobank
	// speaks JSON/HTTP, not gRPC).
	WebhookAddress string
	// PublicBaseURL is this server's own public HTTPS URL, used to build the
	// webhook URL registered with Monobank. Must be reachable from the
	// internet, so it's meaningless for a local-only (kind) deployment.
	PublicBaseURL string
	// MonobankTokenKey encrypts personal tokens at rest (AES-256, derived via
	// SHA-256 from this passphrase — any non-empty string works).
	MonobankTokenKey string

	// GoogleClientID is the OAuth Client ID used as the expected audience
	// when verifying Google Sign-In ID tokens. Optional: Apple sign-in
	// keeps working without it, only /api/auth/google needs it.
	GoogleClientID string
	// CORSAllowedOrigin is the single web frontend origin allowed to call
	// the REST bridge with credentials (cookies) — must be an exact origin,
	// not a wildcard, since credentialed CORS forbids "*".
	CORSAllowedOrigin string
	// JWTWebAccessDuration is how long the web's access token lives before
	// the frontend must use its refresh token to get a new one. Kept short
	// and separate from JWTDuration (which stays iOS's long-lived, no-refresh
	// token) so a stolen web access token has a small blast radius.
	JWTWebAccessDuration time.Duration
	// SessionRefreshDuration is how long a web refresh token (stored
	// hashed in the sessions table) stays valid before the user must log
	// in again.
	SessionRefreshDuration time.Duration
}

func Load() (*Config, error) {
	jwtDuration, err := time.ParseDuration(getEnv("JWT_DURATION", "720h")) // 30 days.
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_DURATION: %w", err)
	}
	jwtWebAccessDuration, err := time.ParseDuration(getEnv("JWT_WEB_ACCESS_DURATION", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_WEB_ACCESS_DURATION: %w", err)
	}
	sessionRefreshDuration, err := time.ParseDuration(getEnv("SESSION_REFRESH_DURATION", "720h")) // 30 days.
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_REFRESH_DURATION: %w", err)
	}

	cfg := &Config{
		GRPCAddress:    getEnv("GRPC_ADDRESS", ":8443"),
		PostgresDSN:    getEnv("POSTGRES_DSN", "postgres://finance:finance@localhost:5432/finance_engine?sslmode=disable"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTDuration:    jwtDuration,
		AppleBundleID:  getEnv("APPLE_BUNDLE_ID", ""),
		NBUExchangeURL: getEnv("NBU_EXCHANGE_URL", "https://bank.gov.ua/NBU_Exchange/exchange?json"),
		DevMode:        getEnv("DEV_MODE", "false") == "true",

		WebhookAddress:   getEnv("WEBHOOK_ADDRESS", ":8080"),
		PublicBaseURL:    getEnv("PUBLIC_BASE_URL", ""),
		MonobankTokenKey: getEnv("MONOBANK_TOKEN_KEY", "dev-monobank-key-change-me"),

		GoogleClientID:         getEnv("GOOGLE_CLIENT_ID", ""),
		CORSAllowedOrigin:      getEnv("CORS_ALLOWED_ORIGIN", "http://localhost:5173"),
		JWTWebAccessDuration:   jwtWebAccessDuration,
		SessionRefreshDuration: sessionRefreshDuration,
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.PostgresDSN == "" {
		return errors.New("POSTGRES_DSN is required")
	}
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if c.AppleBundleID == "" {
		return errors.New("APPLE_BUNDLE_ID is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
