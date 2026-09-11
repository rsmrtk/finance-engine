package main

import (
	"context"
	"fmt"
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
	ratesvc "github.com/rsmrtk/finance-engine/internal/service/rate"
	transactionsvc "github.com/rsmrtk/finance-engine/internal/service/transaction"
	"github.com/rsmrtk/finance-engine/pkg/appleauth"
	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
	"github.com/rsmrtk/finance-engine/pkg/logger"
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
	log.Info("starting finance-engine", logger.H{"grpc_address": cfg.GRPCAddress})

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

	jwtManager := jwt.New(cfg.JWTSecret, cfg.JWTDuration)
	appleVerifier := appleauth.NewVerifier(cfg.AppleBundleID)

	services := grpcserver.Services{
		Auth:        authsvc.New(userRepo, appleVerifier, jwtManager),
		Category:    categorysvc.New(categoryRepo),
		Transaction: transactionsvc.New(transactionRepo, categoryRepo),
		Rate:        ratesvc.New(rateRepo),
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

	errCh := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(); serveErr != nil {
			errCh <- serveErr
		}
	}()

	select {
	case serveErr := <-errCh:
		return fmt.Errorf("grpc server: %w", serveErr)
	case <-ctx.Done():
		log.Info("shutting down", nil)
	}

	server.Shutdown(context.Background())
	log.Info("stopped gracefully", nil)
	return nil
}
