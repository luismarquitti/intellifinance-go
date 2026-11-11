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
