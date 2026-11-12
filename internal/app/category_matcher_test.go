package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

type mockCategoryRepoForMatcher struct {
	categories []*domain.Category
}

func (m *mockCategoryRepoForMatcher) Create(ctx context.Context, category *domain.Category) error {
	m.categories = append(m.categories, category)
	return nil
}

func (m *mockCategoryRepoForMatcher) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Category, error) {
	for _, c := range m.categories {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func (m *mockCategoryRepoForMatcher) List(ctx context.Context, tenantID uuid.UUID, filter ports.CategoryFilter) ([]*domain.Category, error) {
	return m.categories, nil
}

func (m *mockCategoryRepoForMatcher) Update(ctx context.Context, category *domain.Category) error {
	return nil
}

func (m *mockCategoryRepoForMatcher) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return nil
}

func (m *mockCategoryRepoForMatcher) CountTransactionsByCategory(ctx context.Context, tenantID, categoryID uuid.UUID) (int64, error) {
	return 0, nil
}

func TestCategoryMatcher_ScanAndPropose(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	catAlimentacao, errA := domain.NewCategory(tenantID, nil, "Alimentação & Mercado", domain.TransactionTypeExpense, true)
	require.NoError(t, errA)
	catMoradia, errM := domain.NewCategory(tenantID, nil, "Moradia & Contas", domain.TransactionTypeExpense, true)
	require.NoError(t, errM)
	catLazer, errL := domain.NewCategory(tenantID, nil, "Lazer & Assinaturas", domain.TransactionTypeExpense, true)
	require.NoError(t, errL)
	catSalario, errS := domain.NewCategory(tenantID, nil, "Salário / Prolabore", domain.TransactionTypeIncome, true)
	require.NoError(t, errS)
	catSaude, errSa := domain.NewCategory(tenantID, nil, "Saúde & Farmácia", domain.TransactionTypeExpense, true)
	require.NoError(t, errSa)

	catRepo := &mockCategoryRepoForMatcher{
		categories: []*domain.Category{catAlimentacao, catMoradia, catLazer, catSalario, catSaude},
	}

	accRepo := newMockAccountRepo()
	account, _ := domain.NewAccount(tenantID, userID, "Conta Teste", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	_ = accRepo.Create(ctx, account)

	txRepo := newMockTxRepo(accRepo)
	actionRepo := newMockPendingActionRepo()

	matcher := app.NewCategoryMatcher(txRepo, catRepo, actionRepo)

	// 1. Transaction matching Open Finance hint
	txHint := &domain.Transaction{
		ID:               uuid.New(),
		TenantID:         tenantID,
		AccountID:        account.ID,
		Amount:           decimal.RequireFromString("8500.00"),
		Date:             time.Now(),
		Description:      "SALARIO EMPRESA XYZ LTDA",
		CleanDescription: "SALARIO EMPRESA XYZ LTDA",
		Type:             domain.TransactionTypeIncome,
		Status:           domain.TransactionStatusCompleted,
		Metadata: map[string]any{
			"category_hint": "Salary",
		},
	}
	require.NoError(t, txRepo.Create(ctx, txHint))

	// 2. Transaction matching keyword
	txKeyword := &domain.Transaction{
		ID:               uuid.New(),
		TenantID:         tenantID,
		AccountID:        account.ID,
		Amount:           decimal.RequireFromString("-21.90"),
		Date:             time.Now(),
		Description:      "SPOTIFY AB",
		CleanDescription: "SPOTIFY AB",
		Type:             domain.TransactionTypeExpense,
		Status:           domain.TransactionStatusCompleted,
	}
	require.NoError(t, txRepo.Create(ctx, txKeyword))

	// 3. Transaction that is already categorized
	txCategorized := &domain.Transaction{
		ID:                 uuid.New(),
		TenantID:           tenantID,
		AccountID:          account.ID,
		CategoryID:         &catMoradia.ID,
		Amount:             decimal.RequireFromString("-800.00"),
		Date:               time.Now(),
		Description:        "CONDOMINIO",
		CleanDescription:   "CONDOMINIO",
		Type:               domain.TransactionTypeExpense,
		Status:             domain.TransactionStatusCompleted,
		IsManuallyVerified: true,
	}
	require.NoError(t, txRepo.Create(ctx, txCategorized))

	// 4. Transfer transaction (should be skipped)
	txTransfer := &domain.Transaction{
		ID:               uuid.New(),
		TenantID:         tenantID,
		AccountID:        account.ID,
		Amount:           decimal.RequireFromString("-300.00"),
		Date:             time.Now(),
		Description:      "PIX enviado",
		CleanDescription: "PIX ENVIADO",
		Type:             domain.TransactionTypeTransfer,
		Status:           domain.TransactionStatusCompleted,
	}
	require.NoError(t, txRepo.Create(ctx, txTransfer))

	result, err := matcher.ScanAndPropose(ctx, tenantID)
	require.NoError(t, err)

	assert.Equal(t, 4, result.ScannedTransactionsCount)
	assert.Equal(t, 1, result.AlreadyCategorizedCount)
	assert.Equal(t, 2, result.ProposedActionsCount)

	pendingActions, _ := actionRepo.ListPending(ctx, tenantID)
	require.Len(t, pendingActions, 2)

	// Validate proposals
	var foundSalario, foundSpotify bool
	for _, act := range pendingActions {
		assert.Equal(t, domain.ActionTypeCategorizeTransaction, act.ActionType)
		if act.Payload["category_id"] == catSalario.ID.String() {
			foundSalario = true
			assert.Equal(t, catSalario.Name, act.Payload["category_name"])
		}
		if act.Payload["category_id"] == catLazer.ID.String() {
			foundSpotify = true
		}
	}
	assert.True(t, foundSalario, "Expected Salario proposal")
	assert.True(t, foundSpotify, "Expected Spotify proposal")

	// Test idempotency: second scan must propose 0
	result2, err2 := matcher.ScanAndPropose(ctx, tenantID)
	require.NoError(t, err2)
	assert.Equal(t, 0, result2.ProposedActionsCount)
}
