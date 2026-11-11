package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockInvoiceRepo struct {
	invoices map[uuid.UUID]*domain.CreditCardInvoice
}

func newMockInvoiceRepo() *mockInvoiceRepo {
	return &mockInvoiceRepo{invoices: make(map[uuid.UUID]*domain.CreditCardInvoice)}
}

func (m *mockInvoiceRepo) Save(ctx context.Context, inv *domain.CreditCardInvoice) error {
	clone := *inv
	m.invoices[inv.ID] = &clone
	return nil
}

func (m *mockInvoiceRepo) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.CreditCardInvoice, error) {
	inv, ok := m.invoices[id]
	if !ok || inv.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	clone := *inv
	return &clone, nil
}

func (m *mockInvoiceRepo) FindByExternalID(ctx context.Context, tenantID uuid.UUID, extID string) (*domain.CreditCardInvoice, error) {
	for _, inv := range m.invoices {
		if inv.TenantID == tenantID && inv.ExternalID != nil && *inv.ExternalID == extID {
			clone := *inv
			return &clone, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockInvoiceRepo) List(ctx context.Context, tenantID uuid.UUID, accountID *uuid.UUID) ([]*domain.CreditCardInvoice, error) {
	var res []*domain.CreditCardInvoice
	for _, inv := range m.invoices {
		if inv.TenantID == tenantID {
			if accountID != nil && inv.AccountID != *accountID {
				continue
			}
			clone := *inv
			res = append(res, &clone)
		}
	}
	return res, nil
}

func (m *mockInvoiceRepo) ListByAccount(ctx context.Context, tenantID, accountID uuid.UUID) ([]*domain.CreditCardInvoice, error) {
	return m.List(ctx, tenantID, &accountID)
}

func TestInvoiceService_CashflowProjection(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	accRepo := newMockAccountRepo()
	txRepo := newMockTxRepo(accRepo)
	invRepo := newMockInvoiceRepo()

	service := app.NewInvoiceService(invRepo, txRepo, accRepo)

	// 1. Create Accounts
	checkingAcc, err := domain.NewAccount(tenantID, userID, "Conta Corrente", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	checkingAcc.Balance = decimal.NewFromInt(1000)
	require.NoError(t, accRepo.Create(ctx, checkingAcc))

	savingsAcc, err := domain.NewAccount(tenantID, userID, "Poupança", domain.AccountTypeSavings, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	savingsAcc.Balance = decimal.NewFromInt(500)
	require.NoError(t, accRepo.Create(ctx, savingsAcc))

	investAcc, err := domain.NewAccount(tenantID, userID, "Investimentos", domain.AccountTypeInvestment, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	investAcc.Balance = decimal.NewFromInt(10000)
	require.NoError(t, accRepo.Create(ctx, investAcc))

	ccAcc, err := domain.NewAccount(tenantID, userID, "Cartão Black", domain.AccountTypeCreditCard, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accRepo.Create(ctx, ccAcc))

	// 2. Create Invoices
	openInv, err := domain.NewCreditCardInvoice(tenantID, ccAcc.ID, time.Now().AddDate(0, 0, 15), decimal.NewFromInt(400), domain.InvoiceStatusOpen)
	require.NoError(t, err)
	require.NoError(t, invRepo.Save(ctx, openInv))

	closedInv, err := domain.NewCreditCardInvoice(tenantID, ccAcc.ID, time.Now().AddDate(0, 0, 5), decimal.NewFromInt(300), domain.InvoiceStatusClosed)
	require.NoError(t, err)
	require.NoError(t, invRepo.Save(ctx, closedInv))

	paidInv, err := domain.NewCreditCardInvoice(tenantID, ccAcc.ID, time.Now().AddDate(0, -10, 0), decimal.NewFromInt(800), domain.InvoiceStatusPaid)
	require.NoError(t, err)
	require.NoError(t, invRepo.Save(ctx, paidInv))

	// 3. Test Projection
	projection, err := service.GetCashflowProjection(ctx, tenantID)
	require.NoError(t, err)
	require.NotNil(t, projection)

	// Available balance = Checking (1000) + Savings (500) = 1500
	assert.True(t, projection.AvailableBalance.Equal(decimal.NewFromInt(1500)), "available balance must match liquid sum")

	// Total pending bills = Open (400) + Closed (300) = 700
	assert.True(t, projection.TotalPendingBills.Equal(decimal.NewFromInt(700)), "total pending bills must match unpaid sum")

	// Projected net = 1500 - 700 = 800
	assert.True(t, projection.ProjectedNet.Equal(decimal.NewFromInt(800)), "projected net must equal available minus pending")

	assert.Len(t, projection.LiquidAccounts, 2)
	assert.Len(t, projection.UpcomingBills, 2)
}

func TestInvoiceService_GetInvoiceDetails(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	accRepo := newMockAccountRepo()
	txRepo := newMockTxRepo(accRepo)
	invRepo := newMockInvoiceRepo()

	service := app.NewInvoiceService(invRepo, txRepo, accRepo)

	ccAcc, _ := domain.NewAccount(tenantID, userID, "Cartão Black", domain.AccountTypeCreditCard, "BRL", domain.VisibilityShared)
	_ = accRepo.Create(ctx, ccAcc)

	inv, _ := domain.NewCreditCardInvoice(tenantID, ccAcc.ID, time.Now().AddDate(0, 0, 10), decimal.NewFromInt(500), domain.InvoiceStatusOpen)
	_ = invRepo.Save(ctx, inv)

	tx, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       ccAcc.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromInt(-150),
		Description:     "iFood",
		Type:            domain.TransactionTypeExpense,
		BillID:          &inv.ID,
	})
	_ = txRepo.Create(ctx, tx)

	retrievedInv, txs, err := service.GetInvoiceDetails(ctx, tenantID, inv.ID)
	require.NoError(t, err)
	assert.Equal(t, inv.ID, retrievedInv.ID)
	assert.Len(t, txs, 1)
	assert.Equal(t, "iFood", txs[0].Description)
}
