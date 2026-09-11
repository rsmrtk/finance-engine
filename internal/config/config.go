package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Config struct {
	GRPCAddress    string
	PostgresDSN    string
	JWTSecret      string
	JWTDuration    time.Duration
	AppleBundleID  string
	NBUExchangeURL string
	// DevMode enables AuthService.DevSignIn, a Sign in with Apple bypass for
	// local testing without a paid Apple Developer account. Never set this
	// on a real deployment.
	DevMode bool
}

func Load() (*Config, error) {
	jwtDuration, err := time.ParseDuration(getEnv("JWT_DURATION", "720h")) // 30 days.
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_DURATION: %w", err)
	}

	cfg := &Config{
		GRPCAddress:    getEnv("GRPC_ADDRESS", ":8443"),
		PostgresDSN:    getEnv("POSTGRES_DSN", "postgres://finance:finance@localhost:5432/finance_engine?sslmode=disable"),
		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTDuration:    jwtDuration,
		AppleBundleID:  getEnv("APPLE_BUNDLE_ID", ""),
		NBUExchangeURL: getEnv("NBU_EXCHANGE_URL", "https://bank.gov.ua/NBU_Exchange/exchange?json"),
		DevMode:        getEnv("DEV_MODE", "false") == "true",
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
