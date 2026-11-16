package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func getIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("TEST_DATABASE_URL")
	}
	require.NotEmpty(t, dbURL, "DATABASE_URL is required for repository tests")
	require.NoError(t, config.ValidateDisposableDatabase(dbURL, os.Getenv("ENVIRONMENT")))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	return pool
}

func createTestTenantAndAccount(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	// Insert tenant
	_, err := pool.Exec(ctx, "INSERT INTO tenants (id, name, type, currency) VALUES ($1, $2, 'HOUSEHOLD', 'BRL')", tenantID, "Test Tenant")
	require.NoError(t, err)

	// Insert user
	_, err = pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'hash', 'Test User')",
		userID, "user-"+userID.String()[:8]+"@example.com")
	require.NoError(t, err)

	// Link user to tenant
	_, err = pool.Exec(ctx, "INSERT INTO tenant_members (tenant_id, user_id, role) VALUES ($1, $2, 'OWNER')", tenantID, userID)
	require.NoError(t, err)

	// Insert account with tenant RLS set
	err = postgres.WithTenantTx(ctx, pool, tenantID, userID, func(tx pgx.Tx) error {
		_, insertErr := tx.Exec(ctx, `
			INSERT INTO accounts (id, tenant_id, owner_user_id, name, type, currency, balance, visibility)
			VALUES ($1, $2, $3, $4, 'CREDIT_CARD', 'BRL', 0.00, 'SHARED')
		`, accountID, tenantID, userID, "Nubank CC "+accountID.String()[:6])
		return insertErr
	})
	require.NoError(t, err)

	return tenantID, userID, accountID
}

func TestInvoiceRepository_SaveAndQuery(t *testing.T) {
	pool := getIntegrationPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantID, _, accountID := createTestTenantAndAccount(t, pool)
	repo := postgres.NewInvoiceRepository(pool)

	extID := "pluggy-bill-9988"
	dueDate := time.Now().UTC().AddDate(0, 0, 10).Truncate(24 * time.Hour)
	total := decimal.NewFromFloat(850.75)

	inv, err := domain.NewCreditCardInvoice(tenantID, accountID, dueDate, total, domain.InvoiceStatusOpen)
	require.NoError(t, err)
	inv.ExternalID = &extID
	inv.MinimumPayment = decimal.NewFromFloat(150.00)

	// Save
	err = repo.Save(ctx, inv)
	require.NoError(t, err)

	// FindByID
	found, err := repo.FindByID(ctx, tenantID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, inv.ID, found.ID)
	require.True(t, total.Equal(found.TotalAmount))
	require.Equal(t, &extID, found.ExternalID)
	require.Equal(t, domain.InvoiceStatusOpen, found.Status)

	// FindByExternalID
	byExt, err := repo.FindByExternalID(ctx, tenantID, extID)
	require.NoError(t, err)
	require.Equal(t, inv.ID, byExt.ID)

	// Upsert update
	newTotal := decimal.NewFromFloat(920.00)
	inv.TotalAmount = newTotal
	inv.CloseInvoice()
	err = repo.Save(ctx, inv)
	require.NoError(t, err)

	updated, err := repo.FindByID(ctx, tenantID, inv.ID)
	require.NoError(t, err)
	require.True(t, newTotal.Equal(updated.TotalAmount))
	require.Equal(t, domain.InvoiceStatusClosed, updated.Status)

	// ListByAccount
	list, err := repo.ListByAccount(ctx, tenantID, accountID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	// Tenant isolation check
	otherTenantID := uuid.New()
	_, err = repo.FindByID(ctx, otherTenantID, inv.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestProviderBalanceSnapshotRepository_SaveAndQuery(t *testing.T) {
	pool := getIntegrationPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantID, _, accountID := createTestTenantAndAccount(t, pool)
	repo := postgres.NewProviderBalanceSnapshotRepository(pool)

	snap1, err := domain.NewProviderBalanceSnapshot(
		tenantID, accountID, "PLUGGY", decimal.NewFromFloat(1200.50), "BRL", time.Now().UTC().Add(-2*time.Hour),
	)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, snap1))

	snap2, err := domain.NewProviderBalanceSnapshot(
		tenantID, accountID, "PLUGGY", decimal.NewFromFloat(1350.00), "BRL", time.Now().UTC(),
	)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, snap2))

	latest, err := repo.GetLatestByAccount(ctx, tenantID, accountID)
	require.NoError(t, err)
	require.Equal(t, snap2.ID, latest.ID)
	require.True(t, decimal.NewFromFloat(1350.00).Equal(latest.Balance))

	list, err := repo.ListByAccount(ctx, tenantID, accountID, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestPendingActionRepository_WorkflowAndIsolation(t *testing.T) {
	pool := getIntegrationPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantID, userID, _ := createTestTenantAndAccount(t, pool)
	repo := postgres.NewPendingActionRepository(pool)

	targetID := uuid.New()
	action, err := domain.NewPendingAction(
		tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction,
		map[string]any{"transaction_id": targetID.String(), "suggested_category": "Food"},
	)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, action))

	// FindByID
	found, err := repo.FindByID(ctx, tenantID, action.ID)
	require.NoError(t, err)
	require.Equal(t, action.ID, found.ID)
	require.Equal(t, domain.ActionStatusPending, found.Status)
	require.Equal(t, "Food", found.Payload["suggested_category"])

	// ListPending
	pendingList, err := repo.ListPending(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, pendingList, 1)

	// Approve
	decider := userID
	require.NoError(t, repo.UpdateDecision(ctx, tenantID, action.ID, domain.ActionStatusApproved, &decider, nil))

	approved, err := repo.FindByID(ctx, tenantID, action.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ActionStatusApproved, approved.Status)
	require.NotNil(t, approved.DecidedAt)
	require.Equal(t, &decider, approved.DecidedByUserID)

	// After approval, ListPending should be empty
	pendingAfter, err := repo.ListPending(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, pendingAfter, 0)

	// Mark executed
	require.NoError(t, repo.MarkExecuted(ctx, tenantID, action.ID))
	executed, err := repo.FindByID(ctx, tenantID, action.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ActionStatusExecuted, executed.Status)
	require.NotNil(t, executed.ExecutedAt)
}

func TestTransactionRepository_SummarizeMonthlyBalances(t *testing.T) {
	pool := getIntegrationPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantID, userID, ccAccountID := createTestTenantAndAccount(t, pool)
	accRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)

	// Create a checking account with opening anchor balance of 1000.00
	checkingAcc, err := domain.NewAccount(tenantID, userID, "Checking Main", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	checkingAcc.Balance = decimal.RequireFromString("1000.00")
	require.NoError(t, accRepo.Create(ctx, checkingAcc))

	// Past month: October (relative to current Brazil month, use 2 months ago and current month)
	nowBr := time.Now().In(domain.BrazilLocation)
	curYear, curMonth, _ := nowBr.Date()
	curStart := time.Date(curYear, curMonth, 1, 0, 0, 0, 0, time.UTC)
	curEnd := curStart.AddDate(0, 1, -1)
	prevStart := curStart.AddDate(0, -1, 0)
	prevEnd := curStart.AddDate(0, 0, -1)

	prevIncome, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: checkingAcc.ID, CreatedByUserID: &userID,
		Date: prevStart.AddDate(0, 0, 4), Amount: decimal.RequireFromString("500.00"),
		Description: "Prev Salary", Type: domain.TransactionTypeIncome, Source: domain.SourceTypeManual,
	})
	require.NoError(t, err)
	require.NoError(t, txRepo.Create(ctx, prevIncome))

	prevExpense, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: checkingAcc.ID, CreatedByUserID: &userID,
		Date: prevStart.AddDate(0, 0, 9), Amount: decimal.RequireFromString("-200.00"),
		Description: "Prev Bill Payment", Type: domain.TransactionTypeExpense, Source: domain.SourceTypeManual,
	})
	require.NoError(t, err)
	require.NoError(t, txRepo.Create(ctx, prevExpense))

	curExpense, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: checkingAcc.ID, CreatedByUserID: &userID,
		Date: curStart.AddDate(0, 0, 2), Amount: decimal.RequireFromString("-150.00"),
		Description: "Cur Groceries", Type: domain.TransactionTypeExpense, Source: domain.SourceTypeManual,
	})
	require.NoError(t, err)
	require.NoError(t, txRepo.Create(ctx, curExpense))

	// Credit card purchase in current month should NOT inflate liquid summary when AccountID is nil
	ccPurchase, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: ccAccountID, CreatedByUserID: &userID,
		Date: curStart.AddDate(0, 0, 2), Amount: decimal.RequireFromString("-300.00"),
		Description: "CC Store", Type: domain.TransactionTypeExpense, Source: domain.SourceTypeManual,
	})
	require.NoError(t, err)
	require.NoError(t, txRepo.Create(ctx, ccPurchase))

	// Summarize previous month: initial 1000.00, income 500.00, expense 200.00, final 1300.00
	prevSum, err := txRepo.Summarize(ctx, tenantID, ports.TransactionFilter{
		FromDate: &prevStart,
		ToDate:   &prevEnd,
	})
	require.NoError(t, err)
	require.Equal(t, "1000.00", prevSum.InitialBalance.StringFixed(2))
	require.Equal(t, "500.00", prevSum.TotalIncome.StringFixed(2))
	require.Equal(t, "200.00", prevSum.TotalExpense.StringFixed(2))
	require.Equal(t, "1300.00", prevSum.FinalBalance.StringFixed(2))
	require.False(t, prevSum.IsCurrentMonth)
	require.Equal(t, int64(2), prevSum.Count)

	// Summarize current month: initial 1300.00, income 0.00, expense 150.00, final 1150.00
	curSum, err := txRepo.Summarize(ctx, tenantID, ports.TransactionFilter{
		FromDate: &curStart,
		ToDate:   &curEnd,
	})
	require.NoError(t, err)
	require.Equal(t, "1300.00", curSum.InitialBalance.StringFixed(2))
	require.Equal(t, "0.00", curSum.TotalIncome.StringFixed(2))
	require.Equal(t, "150.00", curSum.TotalExpense.StringFixed(2))
	require.Equal(t, "1150.00", curSum.FinalBalance.StringFixed(2))
	require.True(t, curSum.IsCurrentMonth)
	require.Equal(t, int64(1), curSum.Count)

	// Summarize credit card account explicitly
	ccSum, err := txRepo.Summarize(ctx, tenantID, ports.TransactionFilter{
		AccountID: &ccAccountID,
		FromDate:  &curStart,
		ToDate:    &curEnd,
	})
	require.NoError(t, err)
	require.Equal(t, "0.00", ccSum.InitialBalance.StringFixed(2))
	require.Equal(t, "300.00", ccSum.TotalExpense.StringFixed(2))
	require.Equal(t, "-300.00", ccSum.FinalBalance.StringFixed(2))
}
