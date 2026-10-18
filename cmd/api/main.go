package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	httpAdapter "github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/bootstrap"
	"github.com/luismarquitti/intellifinance-go/internal/config"
)

func main() {
	// 1. Structured logging
	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// 2. Load configuration
	cfg := config.Load()
	logger.Info("starting IntelliFinance API Engine...",
		"env", cfg.Environment,
		"bind_addr", cfg.BindAddr,
		"port", cfg.Port,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. PostgreSQL connection pool
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("invalid DATABASE_URL configuration", "err", err)
		os.Exit(1)
	}
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 2
	poolConfig.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Error("failed to create database connection pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Error("failed to ping database", "err", err, "url", cfg.DatabaseURL)
		os.Exit(1)
	}
	logger.Info("database connection established successfully")

	// 4. Auto-migration
	if err := bootstrap.AutoMigrate(ctx, pool, logger); err != nil {
		logger.Error("database auto-migration failed", "err", err)
		os.Exit(1)
	}

	// 5. Dev admin seeding
	if cfg.IsDevelopment() {
		if err := bootstrap.SeedDevAdmin(ctx, pool, cfg, logger); err != nil {
			logger.Error("dev admin seeding failed", "err", err)
			os.Exit(1)
		}
	}

	// 6. Repositories
	accountRepo := postgres.NewAccountRepository(pool)
	categoryRepo := postgres.NewCategoryRepository(pool)
	transactionRepo := postgres.NewTransactionRepository(pool)

	// 7. Parsers
	csvParser := parsers.NewCSVParser()

	// 8. Application Services
	accountService := app.NewAccountService(accountRepo)
	categoryService := app.NewCategoryService(categoryRepo)
	transactionService := app.NewTransactionService(transactionRepo, accountRepo)
	csvIngestionService := app.NewCSVIngestionService(csvParser, transactionRepo, accountRepo)

	// 9. HTTP Router
	corsOpts := cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Tenant-ID", "X-User-ID", "X-Request-ID"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}

	router := httpAdapter.NewRouter(httpAdapter.RouterConfig{
		AccountService:      accountService,
		CategoryService:     categoryService,
		TransactionService:  transactionService,
		CSVIngestionService: csvIngestionService,
		AppConfig:           cfg,
		Logger:              logger,
		CORSOptions:         &corsOpts,
	})

	// 10. HTTP Server
	serverAddr := fmt.Sprintf("%s:%s", cfg.BindAddr, cfg.Port)
	srv := &http.Server{
		Addr:              serverAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 11. Start server asynchronously
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", "address", serverAddr, "url", fmt.Sprintf("http://%s", serverAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- fmt.Errorf("http server failed: %w", err)
		}
	}()

	// 12. Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("server fatal error", "err", err)
		os.Exit(1)
	case sig := <-shutdown:
		logger.Info("shutdown signal received, commencing graceful termination", "signal", sig.String())

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful server shutdown failed, forcing close", "err", err)
			_ = srv.Close()
		}
		logger.Info("server exited cleanly")
	}
}
