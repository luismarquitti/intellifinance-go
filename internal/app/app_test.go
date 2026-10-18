package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAccountRepo struct {
	accounts map[uuid.UUID]*domain.Account
}

func newMockAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{accounts: make(map[uuid.UUID]*domain.Account)}
}

func (m *mockAccountRepo) Create(ctx context.Context, account *domain.Account) error {
	m.accounts[account.ID] = account
	return nil
}
func (m *mockAccountRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Account, error) {
	acc, ok := m.accounts[id]
	if !ok || acc.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return acc, nil
}
func (m *mockAccountRepo) List(ctx context.Context, tenantID uuid.UUID, filter ports.AccountFilter) ([]*domain.Account, error) {
	var res []*domain.Account
	for _, a := range m.accounts {
		if a.TenantID == tenantID {
			res = append(res, a)
		}
	}
	return res, nil
}
func (m *mockAccountRepo) Update(ctx context.Context, account *domain.Account) error {
	m.accounts[account.ID] = account
	return nil
}
func (m *mockAccountRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	delete(m.accounts, id)
	return nil
}
func (m *mockAccountRepo) UpdateBalance(ctx context.Context, tenantID, id uuid.UUID, delta decimal.Decimal) error {
	acc, ok := m.accounts[id]
	if !ok || acc.TenantID != tenantID {
		return domain.ErrNotFound
	}
	acc.Balance = acc.Balance.Add(delta)
	return nil
}

type mockTxRepo struct {
	txs map[uuid.UUID]*domain.Transaction
}

func newMockTxRepo() *mockTxRepo {
	return &mockTxRepo{txs: make(map[uuid.UUID]*domain.Transaction)}
}

func (m *mockTxRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	clone := *tx
	m.txs[tx.ID] = &clone
	return nil
}
func (m *mockTxRepo) BatchCreate(ctx context.Context, txs []*domain.Transaction) error {
	for _, tx := range txs {
		clone := *tx
		m.txs[tx.ID] = &clone
	}
	return nil
}
func (m *mockTxRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error) {
	tx, ok := m.txs[id]
	if !ok || tx.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	clone := *tx
	return &clone, nil
}
func (m *mockTxRepo) List(ctx context.Context, tenantID uuid.UUID, filter ports.TransactionFilter) ([]*domain.Transaction, int64, error) {
	var res []*domain.Transaction
	for _, tx := range m.txs {
		if tx.TenantID == tenantID {
			res = append(res, tx)
		}
	}
	return res, int64(len(res)), nil
}
func (m *mockTxRepo) Summarize(ctx context.Context, tenantID uuid.UUID, filter ports.TransactionFilter) (*ports.TransactionSummary, error) {
	summary := &ports.TransactionSummary{
		TotalIncome:  decimal.Zero,
		TotalExpense: decimal.Zero,
		NetBalance:   decimal.Zero,
	}
	for _, tx := range m.txs {
		if tx.TenantID != tenantID {
			continue
		}
		summary.Count++
		if tx.Type == domain.TransactionTypeIncome {
			summary.TotalIncome = summary.TotalIncome.Add(tx.Amount)
			summary.NetBalance = summary.NetBalance.Add(tx.Amount)
		} else if tx.Type == domain.TransactionTypeExpense {
			summary.TotalExpense = summary.TotalExpense.Add(tx.Amount.Abs())
			summary.NetBalance = summary.NetBalance.Add(tx.Amount)
		}
	}
	return summary, nil
}
func (m *mockTxRepo) Update(ctx context.Context, tx *domain.Transaction) error {
	clone := *tx
	m.txs[tx.ID] = &clone
	return nil
}
func (m *mockTxRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	delete(m.txs, id)
	return nil
}
func (m *mockTxRepo) BatchDelete(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error {
	for _, id := range ids {
		delete(m.txs, id)
	}
	return nil
}
func (m *mockTxRepo) BatchUpdateCategory(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, categoryID *uuid.UUID) error {
	for _, id := range ids {
		if tx, ok := m.txs[id]; ok && tx.TenantID == tenantID {
			tx.CategoryID = categoryID
		}
	}
	return nil
}
func (m *mockTxRepo) FindExistingFingerprints(ctx context.Context, tenantID uuid.UUID, fingerprints []string) (map[string]bool, error) {
	found := make(map[string]bool)
	for _, tx := range m.txs {
		if tx.TenantID == tenantID {
			for _, fp := range fingerprints {
				if tx.Fingerprint == fp {
					found[fp] = true
				}
			}
		}
	}
	return found, nil
}
func (m *mockTxRepo) CreateSplits(ctx context.Context, tenantID, txID uuid.UUID, splits []domain.Split) error {
	return nil
}
func (m *mockTxRepo) GetSplits(ctx context.Context, tenantID, txID uuid.UUID) ([]domain.Split, error) {
	return nil, nil
}

func TestCSVIngestionService_EndToEnd(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	ctx := context.Background()

	accRepo := newMockAccountRepo()
	txRepo := newMockTxRepo()
	parser := parsers.NewCSVParser()

	acc, err := domain.NewAccount(tenantID, userID, "NuBank", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accRepo.Create(ctx, acc))

	ingestionSvc := app.NewCSVIngestionService(parser, txRepo, accRepo)

	csvContent := `Data,Descricao,Valor
2026-02-01,Supermercado Pao de Acucar,-120.50
2026-02-02,Transferencia Recebida PIX,500.00
`

	// 1. First Ingestion: both inserted
	result, err := ingestionSvc.IngestCSV(ctx, tenantID, userID, acc.ID, strings.NewReader(csvContent))
	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalParsed)
	assert.Equal(t, 2, result.Inserted)
	assert.Equal(t, 0, result.Duplicates)

	// Account balance updated: 500.00 - 120.50 = 379.50
	fetchedAcc, _ := accRepo.GetByID(ctx, tenantID, acc.ID)
	assert.True(t, decimal.NewFromFloat(379.50).Equal(fetchedAcc.Balance))

	// 2. Second Ingestion (Exact same file): 100% duplicate immunity
	result2, err := ingestionSvc.IngestCSV(ctx, tenantID, userID, acc.ID, strings.NewReader(csvContent))
	require.NoError(t, err)
	assert.Equal(t, 2, result2.TotalParsed)
	assert.Equal(t, 0, result2.Inserted)
	assert.Equal(t, 2, result2.Duplicates)

	// Balance unchanged
	fetchedAcc2, _ := accRepo.GetByID(ctx, tenantID, acc.ID)
	assert.True(t, decimal.NewFromFloat(379.50).Equal(fetchedAcc2.Balance))
}

func TestTransactionService_BalanceSyncOnDeleteAndUpdate(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	ctx := context.Background()

	accRepo := newMockAccountRepo()
	txRepo := newMockTxRepo()

	acc, err := domain.NewAccount(tenantID, userID, "Itau", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accRepo.Create(ctx, acc))

	txSvc := app.NewTransactionService(txRepo, accRepo)

	// Create Expense of -100.00
	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       acc.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromFloat(-100.00),
		Description:     "Conta de Luz",
		Date:            time.Now(),
		Type:            domain.TransactionTypeExpense,
	})
	require.NoError(t, err)

	err = txSvc.Create(ctx, tx)
	require.NoError(t, err)

	// Account balance should be -100.00
	fetchedAcc, _ := accRepo.GetByID(ctx, tenantID, acc.ID)
	assert.True(t, decimal.NewFromFloat(-100.00).Equal(fetchedAcc.Balance))

	// Update amount to -150.00
	tx.Amount = decimal.NewFromFloat(-150.00)
	err = txSvc.Update(ctx, tx)
	require.NoError(t, err)

	fetchedAccAfterUpdate, _ := accRepo.GetByID(ctx, tenantID, acc.ID)
	assert.True(t, decimal.NewFromFloat(-150.00).Equal(fetchedAccAfterUpdate.Balance))

	// Delete transaction -> balance should revert to 0.00
	err = txSvc.Delete(ctx, tenantID, tx.ID)
	require.NoError(t, err)

	fetchedAccAfterDelete, _ := accRepo.GetByID(ctx, tenantID, acc.ID)
	assert.True(t, decimal.Zero.Equal(fetchedAccAfterDelete.Balance))
}
