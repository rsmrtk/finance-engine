package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rsmrtk/finance-engine/internal/config"
	grpcserver "github.com/rsmrtk/finance-engine/internal/grpc"
	"github.com/rsmrtk/finance-engine/internal/ratesync"
	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/internal/rest"
	"github.com/rsmrtk/finance-engine/internal/retention"
	adminsvc "github.com/rsmrtk/finance-engine/internal/service/admin"
	advisorsvc "github.com/rsmrtk/finance-engine/internal/service/advisor"
	authsvc "github.com/rsmrtk/finance-engine/internal/service/auth"
	billingsvc "github.com/rsmrtk/finance-engine/internal/service/billing"
	categorysvc "github.com/rsmrtk/finance-engine/internal/service/category"
	monobanksvc "github.com/rsmrtk/finance-engine/internal/service/monobank"
	ratesvc "github.com/rsmrtk/finance-engine/internal/service/rate"
	sessionsvc "github.com/rsmrtk/finance-engine/internal/service/session"
	transactionsvc "github.com/rsmrtk/finance-engine/internal/service/transaction"
	"github.com/rsmrtk/finance-engine/internal/webhook"
	"github.com/rsmrtk/finance-engine/pkg/appleauth"
	"github.com/rsmrtk/finance-engine/pkg/cryptobox"
	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/googleauth"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
	"github.com/rsmrtk/finance-engine/pkg/liqpay"
	"github.com/rsmrtk/finance-engine/pkg/logger"
	"github.com/rsmrtk/finance-engine/pkg/monobank"
	"github.com/rsmrtk/finance-engine/pkg/ollama"
)

const (
	rateSyncInterval      = 6 * time.Hour
	retentionSyncInterval = 24 * time.Hour
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.NewConsoleLogger()
	log.Info("starting finance-engine", logger.H{"grpc_address": cfg.GRPCAddress, "dev_mode": cfg.DevMode})
	if cfg.DevMode {
		log.Info("DEV_MODE is on: AuthService.DevSignIn bypasses Sign in with Apple. Never enable this on a real deployment.", nil)
	}
	if cfg.PublicBaseURL == "" {
		log.Info("PUBLIC_BASE_URL is not set: Monobank connect will be unavailable until this points at a public HTTPS URL.", nil)
	}
	if cfg.GoogleClientID == "" {
		log.Info("GOOGLE_CLIENT_ID is not set: /api/auth/google will reject every request until it's configured.", nil)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	queries := dbq.New(pool)

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	defer redisClient.Close()

	userRepo := repository.NewUserRepository(queries)
	categoryRepo := repository.NewCategoryRepository(queries)
	transactionRepo := repository.NewTransactionRepository(queries)
	rateRepo := repository.NewRateRepository(redisClient)
	monobankRepo := repository.NewMonobankRepository(queries)
	paymentRepo := repository.NewPaymentRepository(queries)
	sessionRepo := repository.NewSessionRepository(queries)

	// gRPC/iOS keeps its single long-lived JWT (no refresh, matches how the
	// app has always worked). The web gets a short-lived access JWT plus a
	// separate, revocable DB-backed refresh token (internal/service/session)
	// — both instances share the same secret, so either can verify tokens
	// the other minted; only the embedded expiry (via Generate) differs.
	jwtManager := jwt.New(cfg.JWTSecret, cfg.JWTDuration)
	webAccessJWT := jwt.New(cfg.JWTSecret, cfg.JWTWebAccessDuration)
	appleVerifier := appleauth.NewVerifier(cfg.AppleBundleID)
	googleVerifier := googleauth.NewVerifier(cfg.GoogleClientID)
	sessionService := sessionsvc.New(sessionRepo, webAccessJWT, cfg.SessionRefreshDuration)

	tokenBox, err := cryptobox.New(cfg.MonobankTokenKey)
	if err != nil {
		return fmt.Errorf("init token encryption: %w", err)
	}
	monobankClient := monobank.New()
	monobankService := monobanksvc.New(monobankRepo, monobankClient, tokenBox, cfg.PublicBaseURL)

	ollamaClient := ollama.New(cfg.OllamaURL, cfg.OllamaModel)
	advisorService := advisorsvc.New(transactionRepo, categoryRepo, rateRepo, ollamaClient, redisClient)

	liqpayClient := liqpay.New(cfg.LiqPayPublicKey, cfg.LiqPayPrivateKey, cfg.LiqPaySandbox)
	billingService := billingsvc.New(userRepo, paymentRepo, liqpayClient, cfg.CORSAllowedOrigin, cfg.PublicBaseURL, log)
	adminService := adminsvc.New(userRepo, transactionRepo, billingService)

	services := grpcserver.Services{
		Auth:        authsvc.New(userRepo, appleVerifier, googleVerifier, jwtManager, cfg.DevMode),
		Category:    categorysvc.New(categoryRepo),
		Transaction: transactionsvc.New(transactionRepo, categoryRepo),
		Rate:        ratesvc.New(rateRepo),
		Monobank:    monobankService,
	}

	// Keeps exchange_rates fresh from the National Bank of Ukraine without
	// needing a separate deployed service.
	syncer := ratesync.New(cfg.NBUExchangeURL, rateRepo, log)
	go syncer.Run(ctx, rateSyncInterval)

	retentionJob := retention.New(transactionRepo, log)
	go retentionJob.Run(ctx, retentionSyncInterval)

	server := grpcserver.NewServer(grpcserver.ServerOptions{
		Address:  cfg.GRPCAddress,
		JWT:      jwtManager,
		Logger:   log,
		Services: services,
	})

	monobankWebhook := webhook.NewMonobankHandler(monobankRepo, transactionRepo, categoryRepo, log)
	liqpayWebhook := webhook.NewLiqPayHandler(billingService, log)
	webhookMux := http.NewServeMux()
	webhookMux.Handle("/webhooks/monobank/", monobankWebhook)
	webhookMux.Handle("/webhooks/liqpay", liqpayWebhook)
	// The web frontend's JSON API rides on this same plain-HTTP server —
	// it's already the one port a browser (or Render/kind) can reach
	// without speaking gRPC, so there's no need for a second listener.
	webhookMux.Handle("/api/", rest.NewMux(rest.Options{
		Services:      services,
		Sessions:      sessionService,
		Advisor:       advisorService,
		Billing:       billingService,
		Admin:         adminService,
		JWT:           webAccessJWT,
		CORSOrigin:    cfg.CORSAllowedOrigin,
		AdminOrigin:   cfg.AdminDashboardOrigin,
		AdminPassword: cfg.AdminPassword,
		JWTSecret:     cfg.JWTSecret,
		AccessMaxAge:  cfg.JWTWebAccessDuration,
		RefreshMaxAge: cfg.SessionRefreshDuration,
		Log:           log,
	}))
	webhookServer := &http.Server{Addr: cfg.WebhookAddress, Handler: webhookMux}

	errCh := make(chan error, 2)
	go func() {
		if serveErr := server.Serve(); serveErr != nil {
			errCh <- fmt.Errorf("grpc server: %w", serveErr)
		}
	}()
	go func() {
		log.Info("webhook server listening", logger.H{"address": cfg.WebhookAddress})
		if serveErr := webhookServer.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("webhook server: %w", serveErr)
		}
	}()

	select {
	case serveErr := <-errCh:
		return serveErr
	case <-ctx.Done():
		log.Info("shutting down", nil)
	}

	server.Shutdown(context.Background())
	_ = webhookServer.Shutdown(context.Background())
	log.Info("stopped gracefully", nil)
	return nil
}
