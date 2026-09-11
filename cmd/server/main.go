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

	"github.com/rsmrtk/finance-engine/internal/config"
	grpcserver "github.com/rsmrtk/finance-engine/internal/grpc"
	"github.com/rsmrtk/finance-engine/internal/ratesync"
	"github.com/rsmrtk/finance-engine/internal/repository"
	authsvc "github.com/rsmrtk/finance-engine/internal/service/auth"
	categorysvc "github.com/rsmrtk/finance-engine/internal/service/category"
	monobanksvc "github.com/rsmrtk/finance-engine/internal/service/monobank"
	ratesvc "github.com/rsmrtk/finance-engine/internal/service/rate"
	transactionsvc "github.com/rsmrtk/finance-engine/internal/service/transaction"
	"github.com/rsmrtk/finance-engine/internal/webhook"
	"github.com/rsmrtk/finance-engine/pkg/appleauth"
	"github.com/rsmrtk/finance-engine/pkg/cryptobox"
	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
	"github.com/rsmrtk/finance-engine/pkg/logger"
	"github.com/rsmrtk/finance-engine/pkg/monobank"
)

const rateSyncInterval = 6 * time.Hour

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	queries := dbq.New(pool)

	userRepo := repository.NewUserRepository(queries)
	categoryRepo := repository.NewCategoryRepository(queries)
	transactionRepo := repository.NewTransactionRepository(queries)
	rateRepo := repository.NewRateRepository(queries)
	monobankRepo := repository.NewMonobankRepository(queries)

	jwtManager := jwt.New(cfg.JWTSecret, cfg.JWTDuration)
	appleVerifier := appleauth.NewVerifier(cfg.AppleBundleID)

	tokenBox, err := cryptobox.New(cfg.MonobankTokenKey)
	if err != nil {
		return fmt.Errorf("init token encryption: %w", err)
	}
	monobankClient := monobank.New()
	monobankService := monobanksvc.New(monobankRepo, monobankClient, tokenBox, cfg.PublicBaseURL)

	services := grpcserver.Services{
		Auth:        authsvc.New(userRepo, appleVerifier, jwtManager, cfg.DevMode),
		Category:    categorysvc.New(categoryRepo),
		Transaction: transactionsvc.New(transactionRepo, categoryRepo),
		Rate:        ratesvc.New(rateRepo),
		Monobank:    monobankService,
	}

	// Keeps exchange_rates fresh from the National Bank of Ukraine without
	// needing a separate deployed service.
	syncer := ratesync.New(cfg.NBUExchangeURL, rateRepo, log)
	go syncer.Run(ctx, rateSyncInterval)

	server := grpcserver.NewServer(grpcserver.ServerOptions{
		Address:  cfg.GRPCAddress,
		JWT:      jwtManager,
		Logger:   log,
		Services: services,
	})

	monobankWebhook := webhook.NewMonobankHandler(monobankRepo, transactionRepo, categoryRepo, log)
	webhookMux := http.NewServeMux()
	webhookMux.Handle("/webhooks/monobank/", monobankWebhook)
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
