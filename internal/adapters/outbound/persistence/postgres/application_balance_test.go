package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Exercise the application with real persistence: mocks cannot detect a second
// balance adjustment on top of the PostgreSQL ledger trigger.
func TestApplication_TransactionBalanceExactlyOnce(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenantID, userID := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	account, err := domain.NewAccount(tenantID, userID, "Synthetic ledger", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accounts.Create(ctx, account))
	service := app.NewTransactionService(postgres.NewTransactionRepository(pool))
	checkBalance := func(expected string) {
		t.Helper()
		actual, getErr := accounts.GetByID(ctx, tenantID, account.ID)
		require.NoError(t, getErr)
		require.Equal(t, expected, actual.Balance.StringFixed(2), "ledger effect must be applied exactly once")
	}
	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: account.ID, CreatedByUserID: &userID,
		Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-100.00"),
		Description: "Synthetic expense", Type: domain.TransactionTypeExpense, Source: domain.SourceTypeManual,
	})
	require.NoError(t, err)
	require.NoError(t, service.Create(ctx, tx))
	checkBalance("-100.00")
	tx.Amount = decimal.RequireFromString("-125.00")
	require.NoError(t, service.Update(ctx, tx))
	checkBalance("-125.00")
	require.NoError(t, service.Delete(ctx, tenantID, tx.ID))
	checkBalance("0.00")
	tx.ID = uuid.New()
	require.NoError(t, service.Create(ctx, tx))
	require.NoError(t, service.BatchDelete(ctx, tenantID, []uuid.UUID{tx.ID}))
	checkBalance("0.00")
}

func TestApplication_TransactionBalanceStatusAndAccountTransitions(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenantID, userID := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	first, err := domain.NewAccount(tenantID, userID, "Synthetic source", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	second, err := domain.NewAccount(tenantID, userID, "Synthetic destination", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accounts.Create(ctx, first))
	require.NoError(t, accounts.Create(ctx, second))
	service := app.NewTransactionService(postgres.NewTransactionRepository(pool))
	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: first.ID, CreatedByUserID: &userID,
		Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-12.34"),
		Description: "Synthetic state transitions", Type: domain.TransactionTypeExpense, Source: domain.SourceTypeManual,
	})
	require.NoError(t, err)
	tx.Status = domain.TransactionStatusPending
	require.NoError(t, service.Create(ctx, tx))
	steps := []struct {
		name          string
		status        domain.TransactionStatus
		move          bool
		firstBalance  string
		secondBalance string
	}{
		{"pending", domain.TransactionStatusPending, false, "0.00", "0.00"},
		{"completed", domain.TransactionStatusCompleted, false, "-12.34", "0.00"},
		{"reconciled_without_double_count", domain.TransactionStatusReconciled, false, "-12.34", "0.00"},
		{"void_reverses_balance", domain.TransactionStatusVoid, false, "0.00", "0.00"},
		{"completed_again", domain.TransactionStatusCompleted, false, "-12.34", "0.00"},
		{"move_to_other_account", domain.TransactionStatusCompleted, true, "0.00", "-12.34"},
		{"pending_on_destination", domain.TransactionStatusPending, true, "0.00", "0.00"},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			tx.Status = step.status
			if step.move {
				tx.AccountID = second.ID
			}
			require.NoError(t, service.Update(ctx, tx))
			source, getErr := accounts.GetByID(ctx, tenantID, first.ID)
			require.NoError(t, getErr)
			destination, getErr := accounts.GetByID(ctx, tenantID, second.ID)
			require.NoError(t, getErr)
			require.Equal(t, step.firstBalance, source.Balance.StringFixed(2))
			require.Equal(t, step.secondBalance, destination.Balance.StringFixed(2))
		})
	}
}

func TestApplication_LedgerMutationRollback(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenantID, userID := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	account, err := domain.NewAccount(tenantID, userID, "Synthetic atomic ledger", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accounts.Create(ctx, account))
	repo := postgres.NewTransactionRepository(pool)
	first, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: account.ID, CreatedByUserID: &userID,
		Date: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-10.00"),
		Description: "Synthetic valid batch row", Type: domain.TransactionTypeExpense, Source: domain.SourceTypeManual,
	})
	require.NoError(t, err)
	second, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: uuid.New(), CreatedByUserID: &userID,
		Date: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-20.00"),
		Description: "Synthetic missing batch account", Type: domain.TransactionTypeExpense, Source: domain.SourceTypeManual,
	})
	require.NoError(t, err)
	require.Error(t, repo.BatchCreate(ctx, []*domain.Transaction{first, second}))
	_, err = repo.GetByID(ctx, tenantID, first.ID)
	require.ErrorIs(t, err, domain.ErrNotFound, "a later failure must roll back the earlier insert")
	actual, err := accounts.GetByID(ctx, tenantID, account.ID)
	require.NoError(t, err)
	require.Equal(t, "0.00", actual.Balance.StringFixed(2), "failed batch must roll back trigger effects")
	service := app.NewTransactionService(repo)
	require.NoError(t, service.Create(ctx, first))
	first.AccountID = uuid.New()
	first.Amount = decimal.RequireFromString("-99.00")
	require.Error(t, service.Update(ctx, first))
	persisted, err := service.GetByID(ctx, tenantID, first.ID)
	require.NoError(t, err)
	require.Equal(t, account.ID, persisted.AccountID)
	require.Equal(t, "-10.00", persisted.Amount.StringFixed(2))
	actual, err = accounts.GetByID(ctx, tenantID, account.ID)
	require.NoError(t, err)
	require.Equal(t, "-10.00", actual.Balance.StringFixed(2), "failed update must preserve the old ledger")
}

func TestApplication_CSVBalanceExactlyOnce(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenantID, userID := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	account, err := domain.NewAccount(tenantID, userID, "Synthetic CSV ledger", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accounts.Create(ctx, account))
	service := app.NewCSVIngestionService(parsers.NewCSVParser(), postgres.NewTransactionRepository(pool), accounts)
	const csv = "Data;Lancamento;Valor;Categoria;Identificador\n01/01/2026;SYNTHETIC EXPENSE;-10,50;Outros;synthetic-1\n"
	for attempt := 0; attempt < 2; attempt++ {
		result, ingestErr := service.IngestCSV(ctx, tenantID, userID, account.ID, strings.NewReader(csv))
		require.NoError(t, ingestErr)
		require.Equal(t, 1, result.TotalParsed)
		require.Equal(t, 1-attempt, result.Inserted)
		require.Equal(t, attempt, result.Duplicates)
		actual, getErr := accounts.GetByID(ctx, tenantID, account.ID)
		require.NoError(t, getErr)
		require.Equal(t, "-10.50", actual.Balance.StringFixed(2))
	}
}
