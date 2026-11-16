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
	"github.com/shopspring/decimal"
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
	var userID uuid.UUID
	if envUser := os.Getenv("INTELLIFINANCE_USER_ID"); envUser != "" {
		if parsed, pErr := uuid.Parse(envUser); pErr == nil {
			userID = parsed
		}
	}
	if userID == uuid.Nil {
		_ = pool.QueryRow(ctx, "SELECT user_id FROM tenant_members WHERE tenant_id = $1 LIMIT 1", tenantID).Scan(&userID)
	}
	if userID == uuid.Nil {
		_ = pool.QueryRow(ctx, "SELECT id FROM users ORDER BY created_at ASC LIMIT 1").Scan(&userID)
	}
	if userID == uuid.Nil {
		userID = cfg.DevUserID
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
	catRepo := postgres.NewCategoryRepository(pool)
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

	var creditCardIDs []uuid.UUID
	cardPaymentsByAccount := make(map[uuid.UUID][]ports.ExternalTransaction)

	for _, extAcc := range extAccounts {
		accName := fmt.Sprintf("Pluggy %s", extAcc.Name)
		var internalAcc *domain.Account
		isNewAccount := false

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
			isNewAccount = true
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
		billByMonthMap := make(map[string]*domain.CreditCardInvoice)
		closingDay := 8
		dueDay := 15
		if extAcc.Type == ports.ExternalAccountCreditCard {
			// Preload existing invoices for this card so we preserve manual/reconciled PAID status & metadata
			existingInvs, _ := invRepo.ListByAccount(ctx, tenantID, internalAcc.ID)
			existingByExtID := make(map[string]*domain.CreditCardInvoice)
			for _, ei := range existingInvs {
				if ei.ExternalID != nil && *ei.ExternalID != "" {
					existingByExtID[*ei.ExternalID] = ei
				}
				billByMonthMap[ei.DueDate.Format("2006-01")] = ei
			}

			var bErr error
			bills, bErr = pluggyClient.FetchBills(ctx, extAcc.ID)
			if bErr == nil && len(bills) > 0 {
				logger.Info(fmt.Sprintf("fetched %d bills for credit card %s", len(bills), internalAcc.Name))
				if bills[0].ClosingDate != nil {
					closingDay = bills[0].ClosingDate.Day()
				}
				dueDay = bills[0].DueDate.Day()

				for _, b := range bills {
					invStatus := domain.InvoiceStatusOpen
					switch strings.ToUpper(b.Status) {
					case "CLOSED":
						invStatus = domain.InvoiceStatusClosed
					case "PAID":
						invStatus = domain.InvoiceStatusPaid
					case "OVERDUE":
						invStatus = domain.InvoiceStatusOverdue
					}
					if b.PaymentsCount > 0 {
						invStatus = domain.InvoiceStatusPaid
					}

					var existingInv *domain.CreditCardInvoice
					if ei, ok := existingByExtID[b.ID]; ok {
						existingInv = ei
					} else if ei, ok := billByMonthMap[b.DueDate.Format("2006-01")]; ok {
						existingInv = ei
					}

					inv, iErr := domain.NewCreditCardInvoice(
						tenantID, internalAcc.ID, b.DueDate, b.TotalAmount, invStatus,
					)
					if iErr == nil {
						if existingInv != nil {
							inv.ID = existingInv.ID
							inv.Metadata = existingInv.Metadata
							if existingInv.Status == domain.InvoiceStatusPaid {
								inv.Status = domain.InvoiceStatusPaid
							}
						}
						inv.ExternalID = &b.ID
						inv.ClosingDate = b.ClosingDate
						if b.MinimumPaymentAmount != nil {
							inv.MinimumPayment = *b.MinimumPaymentAmount
						}
						inv.AllowsInstallments = b.AllowsInstallments
						if err := invRepo.Save(ctx, inv); err != nil {
							logger.Warn("failed saving credit card invoice", "err", err, "bill_id", b.ID)
						} else {
							logger.Info("saved credit card invoice", "bill_id", b.ID, "due_date", b.DueDate.Format("2006-01-02"), "total", b.TotalAmount.StringFixed(2), "status", inv.Status)
						}

						savedInv, findErr := invRepo.FindByExternalID(ctx, tenantID, b.ID)
						if findErr == nil && savedInv != nil {
							billIDMap[b.ID] = savedInv.ID
							billByMonthMap[savedInv.DueDate.Format("2006-01")] = savedInv
						} else {
							billIDMap[b.ID] = inv.ID
							billByMonthMap[inv.DueDate.Format("2006-01")] = inv
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
		syntheticInvoiceTotals := make(map[uuid.UUID]decimal.Decimal)
		var cardPayments []ports.ExternalTransaction
		nowDateStr := time.Now().In(domain.BrazilLocation).Format("2006-01-02")

		for _, tx := range extTxs {
			txAmount := tx.Amount
			txType := domain.TransactionTypeExpense
			isBillPayment := false

			if extAcc.Type == ports.ExternalAccountCreditCard {
				// Pluggy credit card convention: purchases are positive, bill payments/refunds are negative
				if strings.EqualFold(strings.TrimSpace(tx.Description), "Pagamento recebido") ||
					strings.EqualFold(strings.TrimSpace(tx.CategoryHint), "Credit card payment") {
					txType = domain.TransactionTypeTransfer
					txAmount = tx.Amount.Abs()
					isBillPayment = true
					cardPayments = append(cardPayments, tx)
				} else if tx.Amount.IsPositive() {
					txType = domain.TransactionTypeExpense
					txAmount = tx.Amount.Neg()
				} else {
					txType = domain.TransactionTypeIncome
					txAmount = tx.Amount.Abs()
				}
			} else if tx.Amount.IsPositive() {
				txType = domain.TransactionTypeIncome
			}

			var internalBillID *uuid.UUID
			if extAcc.Type == ports.ExternalAccountCreditCard && !isBillPayment {
				if tx.BillID != "" {
					if bUUID, ok := billIDMap[tx.BillID]; ok {
						internalBillID = &bUUID
					}
				}
				if internalBillID == nil {
					// Determine target monthly billing cycle based on closing day
					txDateBr := tx.Date.In(domain.BrazilLocation)
					targetMonth := time.Date(txDateBr.Year(), txDateBr.Month(), 1, 0, 0, 0, 0, time.UTC)
					if txDateBr.Day() >= closingDay {
						targetMonth = targetMonth.AddDate(0, 1, 0)
					}
					ymKey := targetMonth.Format("2006-01")
					if existingCycleInv, ok := billByMonthMap[ymKey]; ok {
						internalBillID = &existingCycleInv.ID
						if existingCycleInv.ExternalID == nil {
							syntheticInvoiceTotals[existingCycleInv.ID] = syntheticInvoiceTotals[existingCycleInv.ID].Add(txAmount.Neg())
						}
					} else {
						lastDayOfMonth := targetMonth.AddDate(0, 1, -1).Day()
						dDay := dueDay
						if dDay > lastDayOfMonth {
							dDay = lastDayOfMonth
						}
						cDay := closingDay
						if cDay > lastDayOfMonth {
							cDay = lastDayOfMonth
						}
						targetDue := time.Date(targetMonth.Year(), targetMonth.Month(), dDay, 0, 0, 0, 0, time.UTC)
						targetClosing := time.Date(targetMonth.Year(), targetMonth.Month(), cDay, 0, 0, 0, 0, time.UTC)
						newInv, invErr := domain.NewCreditCardInvoice(tenantID, internalAcc.ID, targetDue, decimal.Zero, domain.InvoiceStatusOpen)
						if invErr == nil {
							newInv.ClosingDate = &targetClosing
							if saveErr := invRepo.Save(ctx, newInv); saveErr == nil {
								billByMonthMap[ymKey] = newInv
								internalBillID = &newInv.ID
								syntheticInvoiceTotals[newInv.ID] = syntheticInvoiceTotals[newInv.ID].Add(txAmount.Neg())
							}
						}
					}
				}
			}

			txMeta := map[string]any{
				"provider":         "PLUGGY",
				"payment_method":   tx.PaymentMethod,
				"category_hint":    tx.CategoryHint,
				"installment_no":   tx.InstallmentNumber,
				"installments_tot": tx.TotalInstallments,
			}
			if isBillPayment {
				txMeta["is_bill_payment"] = true
			}

			txStatus := domain.TransactionStatusCompleted
			if extAcc.Type != ports.ExternalAccountCreditCard && domain.StandardizeDateToBrazil(tx.Date) > nowDateStr {
				txStatus = domain.TransactionStatusPending
				txMeta["is_future"] = true
			}

			newTx, txErr := domain.NewTransaction(domain.NewTransactionParams{
				TenantID:        tenantID,
				AccountID:       internalAcc.ID,
				CreatedByUserID: &userID,
				TransactedAt:    tx.Date,
				Date:            tx.Date,
				Amount:          txAmount,
				Description:     tx.Description,
				Type:            txType,
				Status:          txStatus,
				Visibility:      domain.VisibilityShared,
				Source:          domain.SourceTypePluggy,
				ExternalID:      tx.ID,
				BillID:          internalBillID,
				Metadata:        txMeta,
			})
			if txErr != nil {
				continue
			}

			// Ingest transaction
			syncExisting := func() {
				if internalBillID != nil && tx.ID != "" {
					_ = postgres.WithTenantTx(ctx, pool, tenantID, userID, func(pgTx pgx.Tx) error {
						_, execErr := pgTx.Exec(ctx, "UPDATE transactions SET bill_id = $1 WHERE tenant_id = $2 AND external_id = $3", *internalBillID, tenantID, tx.ID)
						return execErr
					})
				}
				if tx.ID != "" && extAcc.Type != ports.ExternalAccountCreditCard {
					reconcileDuplicateTransactionStatus(ctx, pool, tenantID, userID, tx.ID, txStatus)
				}
			}

			if err := txService.Create(ctx, newTx); err != nil {
				if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "uq_tenant_transaction_fingerprint") {
					duplicateCount++
					syncExisting()
					continue
				}
				logger.Warn("failed creating transaction", "err", err, "desc", tx.Description)
				continue
			}
			syncExisting()
			createdCount++
		}

		// Update totals on synthetic future invoices without external_id
		for synID, totalAmt := range syntheticInvoiceTotals {
			if synInv, fErr := invRepo.FindByID(ctx, tenantID, synID); fErr == nil && synInv != nil && synInv.ExternalID == nil {
				synInv.TotalAmount = totalAmt.Round(2)
				_ = invRepo.Save(ctx, synInv)
			}
		}

		// Reconcile paid status after all accounts & transactions are synced
		if extAcc.Type == ports.ExternalAccountCreditCard {
			creditCardIDs = append(creditCardIDs, internalAcc.ID)
			cardPaymentsByAccount[internalAcc.ID] = cardPayments
		} else if isNewAccount {
			// Anchor initial liquid account balance to Pluggy's reported balance (adjusting for post-today scheduled transactions)
			futureDelta := decimal.Zero
			for _, tx := range extTxs {
				if domain.StandardizeDateToBrazil(tx.Date) > nowDateStr {
					futureDelta = futureDelta.Add(tx.Amount)
				}
			}
			targetBal := extAcc.Balance.Add(futureDelta).Round(2)
			if currentAcc, gErr := accountRepo.GetByID(ctx, tenantID, internalAcc.ID); gErr == nil && currentAcc != nil {
				if !currentAcc.Balance.Equal(targetBal) {
					delta := targetBal.Sub(currentAcc.Balance)
					if uErr := accountRepo.UpdateBalance(ctx, tenantID, internalAcc.ID, delta); uErr != nil {
						logger.Warn("failed anchoring liquid account balance", "account", internalAcc.Name, "err", uErr)
					} else {
						logger.Info("anchored liquid account balance", "account", internalAcc.Name, "balance", targetBal.StringFixed(2))
					}
				}
			}
		}

		logger.Info("transactions sync summary",
			"account", internalAcc.Name,
			"created", createdCount,
			"duplicates_skipped", duplicateCount,
		)
	}

	// 4b. Reconcile paid status for all synced credit card accounts against checking debits and card payments
	for _, cardID := range creditCardIDs {
		reconcileCardInvoicesPaid(ctx, tenantID, cardID, cardPaymentsByAccount[cardID], accountRepo, txRepo, invRepo, logger)
	}

	// 5. Automatic Transfer & Bill Payment Reconciliation Scan
	logger.Info("running automatic transfer reconciliation scan...")
	pendingActionRepo := postgres.NewPendingActionRepository(pool)
	reconciler := app.NewTransferReconciliationService(txRepo, accountRepo, invRepo, pendingActionRepo)
	scanResult, scanErr := reconciler.ScanAndPropose(ctx, tenantID)
	if scanErr != nil {
		logger.Warn("failed running transfer reconciliation scan", "err", scanErr)
	} else {
		logger.Info("transfer reconciliation scan complete",
			"scanned_txs", scanResult.ScannedTransactionsCount,
			"proposed_actions", scanResult.ProposedActionsCount,
		)
		for _, act := range scanResult.ProposedActions {
			logger.Info("proposed pending reconciliation action",
				"id", act.ID,
				"type", act.Payload["reconciliation_type"],
				"desc", act.Payload["description"],
				"amount", act.Payload["amount"],
			)
		}
	}

	// 6. Automatic Category Matching & Proposal Scan
	logger.Info("running automatic category matching scan...")
	categoryMatcher := app.NewCategoryMatcher(txRepo, catRepo, pendingActionRepo)
	catScanResult, catErr := categoryMatcher.ScanAndPropose(ctx, tenantID)
	if catErr != nil {
		logger.Warn("failed running category matching scan", "err", catErr)
	} else {
		logger.Info("category matching scan complete",
			"scanned_txs", catScanResult.ScannedTransactionsCount,
			"already_categorized", catScanResult.AlreadyCategorizedCount,
			"proposed_actions", catScanResult.ProposedActionsCount,
		)
	}

	logger.Info("Open Finance sync completed successfully!")
}

func reconcileCardInvoicesPaid(
	ctx context.Context,
	tenantID, cardAccountID uuid.UUID,
	cardPayments []ports.ExternalTransaction,
	accountRepo ports.AccountRepository,
	txRepo ports.TransactionRepository,
	invRepo ports.InvoiceRepository,
	logger *slog.Logger,
) {
	allAccounts, err := accountRepo.List(ctx, tenantID, ports.AccountFilter{})
	if err != nil {
		return
	}

	var checkingAccounts []*domain.Account
	checkingTxs := make(map[uuid.UUID][]*domain.Transaction)
	for _, acc := range allAccounts {
		if acc.Type == domain.AccountTypeChecking && acc.IsActive && !acc.IsDeleted() {
			checkingAccounts = append(checkingAccounts, acc)
			txs, _, listErr := txRepo.List(ctx, tenantID, ports.TransactionFilter{AccountID: &acc.ID, Limit: 1000})
			if listErr == nil {
				checkingTxs[acc.ID] = txs
			}
		}
	}

	invoices, err := invRepo.ListByAccount(ctx, tenantID, cardAccountID)
	if err != nil {
		return
	}

	unpaidSum := decimal.Zero
	for _, inv := range invoices {
		if inv.Status != domain.InvoiceStatusPaid {
			var matchedCheckingAcc *domain.Account
			var matchedCheckingTx *domain.Transaction

			// 1. Match exact checking debit near due date
			for _, chk := range checkingAccounts {
				for _, tx := range checkingTxs[chk.ID] {
					if tx.Status == domain.TransactionStatusVoid || !tx.Amount.IsNegative() {
						continue
					}
					diffDays := tx.Date.Sub(inv.DueDate).Hours() / 24.0
					if diffDays < -7 || diffDays > 15 {
						continue
					}
					if tx.Amount.Abs().Equal(inv.TotalAmount) {
						matchedCheckingAcc = chk
						matchedCheckingTx = tx
						upper := strings.ToUpper(tx.Description)
						if strings.Contains(upper, "FATURA") || strings.Contains(upper, "ITAU BLACK") || strings.Contains(upper, "UNICLASS") {
							break
						}
					}
				}
				if matchedCheckingTx != nil {
					break
				}
			}

			// 2. Fallback match against Pluggy card "Pagamento recebido" entries
			hasCardPayment := false
			var cardPayDate time.Time
			for _, cp := range cardPayments {
				if inv.ExternalID != nil && cp.BillID == *inv.ExternalID {
					hasCardPayment = true
					cardPayDate = cp.Date
					break
				}
				diffDays := cp.Date.Sub(inv.DueDate).Hours() / 24.0
				if diffDays >= -7 && diffDays <= 15 && cp.Amount.Abs().Equal(inv.TotalAmount) {
					hasCardPayment = true
					cardPayDate = cp.Date
					break
				}
			}

			if matchedCheckingTx != nil || hasCardPayment {
				if inv.Metadata == nil {
					inv.Metadata = make(map[string]any)
				}
				if matchedCheckingTx != nil && matchedCheckingAcc != nil {
					inv.Metadata["payment_transaction_id"] = matchedCheckingTx.ID.String()
					inv.Metadata["payment_account_id"] = matchedCheckingAcc.ID.String()
					inv.Metadata["payment_account_name"] = matchedCheckingAcc.Name
					inv.Metadata["paid_at"] = matchedCheckingTx.Date.Format("2006-01-02")
				} else if !cardPayDate.IsZero() {
					inv.Metadata["paid_at"] = cardPayDate.Format("2006-01-02")
				}
				inv.Metadata["paid_amount"] = inv.TotalAmount.StringFixed(2)
				inv.MarkAsPaid()
				if saveErr := invRepo.Save(ctx, inv); saveErr != nil {
					logger.Warn("failed marking synced invoice paid", "invoice_id", inv.ID, "err", saveErr)
				} else {
					logger.Info("reconciled credit card invoice as PAID", "due_date", inv.DueDate.Format("2006-01-02"), "total", inv.TotalAmount.StringFixed(2))
				}
			}
		}

		if inv.Status != domain.InvoiceStatusPaid {
			unpaidSum = unpaidSum.Add(inv.TotalAmount)
		}
	}

	if cardAcc, getErr := accountRepo.GetByID(ctx, tenantID, cardAccountID); getErr == nil && cardAcc != nil {
		targetCardBal := unpaidSum.Neg().Round(2)
		if !cardAcc.Balance.Equal(targetCardBal) {
			_ = accountRepo.UpdateBalance(ctx, tenantID, cardAccountID, targetCardBal.Sub(cardAcc.Balance))
		}
	}
}

func reconcileDuplicateTransactionStatus(
	ctx context.Context,
	pool *pgxpool.Pool,
	tenantID, userID uuid.UUID,
	externalID string,
	targetStatus domain.TransactionStatus,
) {
	if externalID == "" {
		return
	}
	_ = postgres.WithTenantTx(ctx, pool, tenantID, userID, func(pgTx pgx.Tx) error {
		var query string
		if targetStatus == domain.TransactionStatusPending {
			query = "UPDATE transactions SET status = 'PENDING', metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{is_future}', 'true'::jsonb), updated_at = NOW() WHERE tenant_id = $1 AND external_id = $2 AND status = 'COMPLETED'"
		} else {
			query = "UPDATE transactions SET status = 'COMPLETED', metadata = COALESCE(metadata, '{}'::jsonb) - 'is_future', updated_at = NOW() WHERE tenant_id = $1 AND external_id = $2 AND status = 'PENDING'"
		}
		_, execErr := pgTx.Exec(ctx, query, tenantID, externalID)
		return execErr
	})
}
