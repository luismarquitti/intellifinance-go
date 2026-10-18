package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestTenantAndUser(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name, type, currency)
		VALUES ($1, 'Test Tenant', 'HOUSEHOLD', 'BRL')
	`, tenantID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, full_name)
		VALUES ($1, $2, 'hash', 'Test User')
	`, userID, "user_"+userID.String()[:8]+"@example.com")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO tenant_members (id, tenant_id, user_id, role)
		VALUES ($1, $2, $3, 'ADMIN')
	`, uuid.New(), tenantID, userID)
	require.NoError(t, err)

	return tenantID, userID
}

func TestAccountRepository_CRUD(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID, userID := createTestTenantAndUser(t, pool)
	repo := postgres.NewAccountRepository(pool)
	ctx := context.Background()

	// 1. Create Account
	acc, err := domain.NewAccount(tenantID, userID, "Carteira Principal", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)

	err = repo.Create(ctx, acc)
	require.NoError(t, err)

	// 2. GetByID
	fetched, err := repo.GetByID(ctx, tenantID, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, acc.Name, fetched.Name)
	assert.Equal(t, acc.Currency, fetched.Currency)
	assert.True(t, acc.Balance.Equal(fetched.Balance))

	// 3. Update Balance
	err = repo.UpdateBalance(ctx, tenantID, acc.ID, decimal.NewFromFloat(150.50))
	require.NoError(t, err)

	fetchedAfterBalance, err := repo.GetByID(ctx, tenantID, acc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.NewFromFloat(150.50).Equal(fetchedAfterBalance.Balance))

	// 4. List Accounts with filter
	accs, err := repo.List(ctx, tenantID, ports.AccountFilter{})
	require.NoError(t, err)
	assert.Len(t, accs, 1)

	// 5. Soft Delete
	err = repo.Delete(ctx, tenantID, acc.ID)
	require.NoError(t, err)

	// GetByID after soft delete should return ErrNotFound
	_, err = repo.GetByID(ctx, tenantID, acc.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestCategoryRepository_CRUD(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID, _ := createTestTenantAndUser(t, pool)
	repo := postgres.NewCategoryRepository(pool)
	ctx := context.Background()

	// 1. Create Category
	cat, err := domain.NewCategory(tenantID, nil, "Alimentação", domain.TransactionTypeExpense, false)
	require.NoError(t, err)

	err = repo.Create(ctx, cat)
	require.NoError(t, err)

	// 2. GetByID
	fetched, err := repo.GetByID(ctx, tenantID, cat.ID)
	require.NoError(t, err)
	assert.Equal(t, cat.Name, fetched.Name)
	assert.Equal(t, cat.Type, fetched.Type)

	// 3. List Categories
	cats, err := repo.List(ctx, tenantID, ports.CategoryFilter{})
	require.NoError(t, err)
	assert.Len(t, cats, 1)

	// 4. Update Category
	cat.Name = "Alimentação & Supermercado"
	err = repo.Update(ctx, cat)
	require.NoError(t, err)

	fetchedUpdated, err := repo.GetByID(ctx, tenantID, cat.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alimentação & Supermercado", fetchedUpdated.Name)

	// 5. Delete Category
	err = repo.Delete(ctx, tenantID, cat.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, tenantID, cat.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTransactionRepository_FiltersAndSummarize(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID, userID := createTestTenantAndUser(t, pool)
	accRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	ctx := context.Background()

	acc, err := domain.NewAccount(tenantID, userID, "Conta Corrente", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accRepo.Create(ctx, acc))

	// Create 2 transactions: 1 Income (100.00) and 1 Expense (40.00)
	now := time.Now().UTC()
	tx1, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       acc.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromFloat(100.00),
		Description:     "Salário Empresa",
		Date:            now,
		Type:            domain.TransactionTypeIncome,
	})
	require.NoError(t, err)

	tx2, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       acc.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromFloat(-40.00),
		Description:     "Padaria Central",
		Date:            now,
		Type:            domain.TransactionTypeExpense,
	})
	require.NoError(t, err)

	require.NoError(t, txRepo.BatchCreate(ctx, []*domain.Transaction{tx1, tx2}))

	// Summarize
	summary, err := txRepo.Summarize(ctx, tenantID, ports.TransactionFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.Count)
	assert.True(t, decimal.NewFromFloat(100.00).Equal(summary.TotalIncome))
	assert.True(t, decimal.NewFromFloat(40.00).Equal(summary.TotalExpense))
	assert.True(t, decimal.NewFromFloat(60.00).Equal(summary.NetBalance))

	// Filter by search
	list, total, err := txRepo.List(ctx, tenantID, ports.TransactionFilter{
		Search: "Padaria",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)
	assert.Equal(t, "Padaria Central", list[0].Description)

	// Batch delete
	err = txRepo.BatchDelete(ctx, tenantID, []uuid.UUID{tx1.ID, tx2.ID})
	require.NoError(t, err)

	listAfter, totalAfter, err := txRepo.List(ctx, tenantID, ports.TransactionFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), totalAfter)
	assert.Len(t, listAfter, 0)
}
