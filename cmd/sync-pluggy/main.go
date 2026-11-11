package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/openfinance/pluggy"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
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

	// Resolve tenant
	tenantID := cfg.DevTenantID
	if envTenant := os.Getenv("INTELLIFINANCE_TENANT_ID"); envTenant != "" {
		if parsed, pErr := uuid.Parse(envTenant); pErr == nil {
			tenantID = parsed
		}
	}
	if tenantID == uuid.Nil {
		_ = pool.QueryRow(ctx, "SELECT id FROM tenants ORDER BY created_at ASC LIMIT 1").Scan(&tenantID)
	}
	if tenantID == uuid.Nil {
		logger.Error("no valid tenant found in database")
		os.Exit(1)
	}

	// Resolve user
	userID := cfg.DevUserID
	if userID == uuid.Nil {
		_ = pool.QueryRow(ctx, "SELECT user_id FROM tenant_members WHERE tenant_id = $1 LIMIT 1", tenantID).Scan(&userID)
	}
	if userID == uuid.Nil {
		_ = pool.QueryRow(ctx, "SELECT id FROM users ORDER BY created_at ASC LIMIT 1").Scan(&userID)
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
	txService := app.NewTransactionService(txRepo)

	// 1. Fetch accounts from Pluggy
	extAccounts, err := pluggyClient.FetchAccounts(ctx, itemID)
	if err != nil {
		logger.Error("failed fetching external accounts", "err", err)
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("retrieved %d accounts from Pluggy", len(extAccounts)))

	// Load existing accounts for tenant
	existingAccounts, err := accountRepo.List(ctx, tenantID, ports.AccountFilter{})
	if err != nil {
		logger.Error("failed listing accounts from database", "err", err)
		os.Exit(1)
	}

	accountMap := make(map[string]*domain.Account)
	for _, acc := range existingAccounts {
		if acc.AccountNumberMasked != "" {
			accountMap[acc.AccountNumberMasked] = acc
		}
		accountMap[acc.Name] = acc
	}

	for _, extAcc := range extAccounts {
		accName := fmt.Sprintf("Pluggy %s", extAcc.Name)
		var internalAcc *domain.Account

		if existing, ok := accountMap[extAcc.Number]; ok {
			internalAcc = existing
		} else if existing, ok := accountMap[accName]; ok {
			internalAcc = existing
		} else {
			// Create account
			domainAccType := domain.AccountTypeChecking
			if extAcc.Type == ports.ExternalAccountCreditCard {
				domainAccType = domain.AccountTypeCreditCard
			}

			newAcc, createErr := domain.NewAccount(
				tenantID, userID, accName, domainAccType, extAcc.CurrencyCode, domain.VisibilityShared,
			)
			if createErr != nil {
				logger.Error("failed creating account entity", "err", createErr)
				continue
			}
			newAcc.AccountNumberMasked = extAcc.Number
			newAcc.InstitutionName = "Pluggy Bank Sandbox"
			newAcc.InstitutionID = extAcc.ID

			if err := accountRepo.Create(ctx, newAcc); err != nil {
				logger.Error("failed saving new account to database", "err", err, "name", accName)
				continue
			}
			internalAcc = newAcc
			accountMap[extAcc.Number] = internalAcc
			logger.Info("created financial account", "id", internalAcc.ID, "name", internalAcc.Name, "type", internalAcc.Type)
		}

		// 2. Save Provider Balance Snapshot (ADR-012)
		snap, snapErr := domain.NewProviderBalanceSnapshot(
			tenantID, internalAcc.ID, "PLUGGY", extAcc.Balance, extAcc.CurrencyCode, time.Now().UTC(),
		)
		if snapErr == nil {
			snap.Metadata["provider_account_id"] = extAcc.ID
			snap.Metadata["item_id"] = itemID
			if err := snapRepo.Save(ctx, snap); err != nil {
				logger.Warn("failed saving balance snapshot", "err", err)
			} else {
				logger.Info("saved provider balance snapshot", "account", internalAcc.Name, "balance", extAcc.Balance.StringFixed(2))
			}
		}

		// 3. If credit card, fetch bills
		var bills []ports.ExternalBill
		billIDMap := make(map[string]uuid.UUID)
		if extAcc.Type == ports.ExternalAccountCreditCard {
			var bErr error
			bills, bErr = pluggyClient.FetchBills(ctx, extAcc.ID)
			if bErr == nil && len(bills) > 0 {
				logger.Info(fmt.Sprintf("fetched %d bills for credit card %s", len(bills), internalAcc.Name))
				for _, b := range bills {
					invStatus := domain.InvoiceStatusOpen
					if strings.ToUpper(b.Status) == "CLOSED" {
						invStatus = domain.InvoiceStatusClosed
					} else if strings.ToUpper(b.Status) == "PAID" {
						invStatus = domain.InvoiceStatusPaid
					}

					inv, iErr := domain.NewCreditCardInvoice(
						tenantID, internalAcc.ID, b.DueDate, b.TotalAmount, invStatus,
					)
					if iErr == nil {
						inv.ExternalID = &b.ID
						inv.ClosingDate = b.ClosingDate
						if b.MinimumPaymentAmount != nil {
							inv.MinimumPayment = *b.MinimumPaymentAmount
						}
						inv.AllowsInstallments = b.AllowsInstallments
						if err := invRepo.Save(ctx, inv); err != nil {
							logger.Warn("failed saving credit card invoice", "err", err, "bill_id", b.ID)
						} else {
							logger.Info("saved credit card invoice", "bill_id", b.ID, "due_date", b.DueDate.Format("2006-01-02"), "total", b.TotalAmount.StringFixed(2))
						}

						savedInv, findErr := invRepo.FindByExternalID(ctx, tenantID, b.ID)
						if findErr == nil && savedInv != nil {
							billIDMap[b.ID] = savedInv.ID
						} else {
							billIDMap[b.ID] = inv.ID
						}
					}
				}
			}
		}

		// 4. Fetch and ingest transactions
		extTxs, tErr := pluggyClient.FetchTransactions(ctx, extAcc.ID, time.Time{}, time.Time{})
		if tErr != nil {
			logger.Error("failed fetching transactions for account", "err", tErr, "account", extAcc.Name)
			continue
		}

		logger.Info(fmt.Sprintf("retrieved %d transactions for %s", len(extTxs), internalAcc.Name))

		createdCount := 0
		duplicateCount := 0

		for _, tx := range extTxs {
			txType := domain.TransactionTypeExpense
			if tx.Amount.IsPositive() {
				txType = domain.TransactionTypeIncome
			}

			var internalBillID *uuid.UUID
			if tx.BillID != "" {
				if bUUID, ok := billIDMap[tx.BillID]; ok {
					internalBillID = &bUUID
				}
			} else if extAcc.Type == ports.ExternalAccountCreditCard && len(bills) > 0 {
				// Fallback to active open invoice for credit card purchases without explicit billId
				for _, b := range bills {
					if bUUID, ok := billIDMap[b.ID]; ok {
						internalBillID = &bUUID
						break
					}
				}
			}

			newTx, txErr := domain.NewTransaction(domain.NewTransactionParams{
				TenantID:        tenantID,
				AccountID:       internalAcc.ID,
				CreatedByUserID: &userID,
				TransactedAt:    tx.Date,
				Date:            tx.Date,
				Amount:          tx.Amount,
				Description:     tx.Description,
				Type:            txType,
				Status:          domain.TransactionStatusCompleted,
				Visibility:      domain.VisibilityShared,
				Source:          domain.SourceTypePluggy,
				ExternalID:      tx.ID,
				BillID:          internalBillID,
				Metadata: map[string]any{
					"provider":         "PLUGGY",
					"payment_method":   tx.PaymentMethod,
					"category_hint":    tx.CategoryHint,
					"installment_no":   tx.InstallmentNumber,
					"installments_tot": tx.TotalInstallments,
				},
			})
			if txErr != nil {
				continue
			}

			// Ingest transaction
			if err := txService.Create(ctx, newTx); err != nil {
				if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "uq_tenant_transaction_fingerprint") {
					duplicateCount++
					if internalBillID != nil && tx.ID != "" {
						_ = postgres.WithTenantTx(ctx, pool, tenantID, userID, func(pgTx pgx.Tx) error {
							_, execErr := pgTx.Exec(ctx, "UPDATE transactions SET bill_id = $1 WHERE tenant_id = $2 AND external_id = $3", *internalBillID, tenantID, tx.ID)
							return execErr
						})
					}
					continue
				}
				logger.Warn("failed creating transaction", "err", err, "desc", tx.Description)
				continue
			}
			if internalBillID != nil && tx.ID != "" {
				_ = postgres.WithTenantTx(ctx, pool, tenantID, userID, func(pgTx pgx.Tx) error {
					_, execErr := pgTx.Exec(ctx, "UPDATE transactions SET bill_id = $1 WHERE tenant_id = $2 AND external_id = $3", *internalBillID, tenantID, tx.ID)
					return execErr
				})
			}
			createdCount++
		}

		logger.Info("transactions sync summary",
			"account", internalAcc.Name,
			"created", createdCount,
			"duplicates_skipped", duplicateCount,
		)
	}

	logger.Info("Open Finance sync completed successfully!")
}
