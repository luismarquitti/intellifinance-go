package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/openfinance/pluggy"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

func main() {
	itemIDFlag := flag.String("item", "", "Pluggy connection item ID (defaults to PLUGGY_ITEM_ID or 7e0ab668-2a83-4d50-bce0-a62aff22d104)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()

	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	clientID := os.Getenv("PLUGGY_CLIENT_ID")
	clientSecret := os.Getenv("PLUGGY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		logger.Error("PLUGGY_CLIENT_ID and PLUGGY_CLIENT_SECRET are required")
		os.Exit(1)
	}

	itemID := *itemIDFlag
	if itemID == "" {
		itemID = os.Getenv("PLUGGY_ITEM_ID")
	}
	if itemID == "" {
		itemID = "7e0ab668-2a83-4d50-bce0-a62aff22d104"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	tenantID, userID := resolveTenantAndUser(ctx, pool, cfg)
	if tenantID == uuid.Nil {
		logger.Error("no valid tenant found in database")
		os.Exit(1)
	}

	logger.Info("starting Open Finance sync...", "tenant_id", tenantID, "item_id", itemID)

	pluggyClient := pluggy.NewClient(pluggy.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Timeout:      30 * time.Second,
	})

	accountRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	invRepo := postgres.NewInvoiceRepository(pool)
	snapRepo := postgres.NewProviderBalanceSnapshotRepository(pool)
	pendingActionRepo := postgres.NewPendingActionRepository(pool)
	catRepo := postgres.NewCategoryRepository(pool)

	reconciler := app.NewTransferReconciliationService(txRepo, accountRepo, invRepo, pendingActionRepo)
	categoryMatcher := app.NewCategoryMatcher(txRepo, catRepo, pendingActionRepo)
	var syncService ports.OpenFinanceSyncService = app.NewOpenFinanceSyncService(
		pluggyClient,
		accountRepo,
		txRepo,
		invRepo,
		snapRepo,
		reconciler,
		categoryMatcher,
		logger,
	)

	res, err := syncService.SyncItem(ctx, ports.SyncItemRequest{
		TenantID: tenantID,
		UserID:   userID,
		ItemID:   itemID,
	})
	if err != nil {
		logger.Error("Open Finance sync failed", "err", err)
		os.Exit(1)
	}

	logger.Info("Open Finance sync completed successfully!",
		"accounts_synced", res.AccountsSynced,
		"snapshots_saved", res.SnapshotsSaved,
		"invoices_upserted", res.InvoicesUpserted,
		"invoices_settled", res.InvoicesSettled,
		"transactions_created", res.TransactionsCreated,
		"duplicates_reconciled", res.TransactionsDeduplicated,
		"transfer_actions_proposed", res.TransferActionsProposed,
		"category_actions_proposed", res.CategoryActionsProposed,
	)
}

func resolveTenantAndUser(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) (uuid.UUID, uuid.UUID) {
	tenantID := cfg.DevTenantID
	if envTenant := os.Getenv("INTELLIFINANCE_TENANT_ID"); envTenant != "" {
		if parsed, err := uuid.Parse(envTenant); err == nil {
			tenantID = parsed
		}
	}
	if tenantID == uuid.Nil {
		_ = pool.QueryRow(ctx, "SELECT id FROM tenants ORDER BY created_at ASC LIMIT 1").Scan(&tenantID)
	}

	var userID uuid.UUID
	if envUser := os.Getenv("INTELLIFINANCE_USER_ID"); envUser != "" {
		if parsed, err := uuid.Parse(envUser); err == nil {
			userID = parsed
		}
	}
	if userID == uuid.Nil && tenantID != uuid.Nil {
		_ = pool.QueryRow(ctx, "SELECT user_id FROM tenant_members WHERE tenant_id = $1 LIMIT 1", tenantID).Scan(&userID)
	}
	if userID == uuid.Nil {
		_ = pool.QueryRow(ctx, "SELECT id FROM users ORDER BY created_at ASC LIMIT 1").Scan(&userID)
	}
	if userID == uuid.Nil {
		userID = cfg.DevUserID
	}
	return tenantID, userID
}
