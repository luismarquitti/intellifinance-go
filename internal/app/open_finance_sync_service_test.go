package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOpenFinanceConnector struct {
	accounts     []ports.ExternalAccount
	bills        map[string][]ports.ExternalBill
	transactions map[string][]ports.ExternalTransaction
}

func (f *fakeOpenFinanceConnector) CreateConnectToken(context.Context, uuid.UUID, *string) (string, time.Time, error) {
	return "", time.Time{}, nil
}
func (f *fakeOpenFinanceConnector) GetItem(context.Context, string) (*ports.ItemSyncResult, error) {
	return &ports.ItemSyncResult{Status: ports.ItemStatusUpdated}, nil
}
func (f *fakeOpenFinanceConnector) FetchAccounts(_ context.Context, _ string) ([]ports.ExternalAccount, error) {
	return f.accounts, nil
}
func (f *fakeOpenFinanceConnector) FetchBills(_ context.Context, accountID string) ([]ports.ExternalBill, error) {
	return f.bills[accountID], nil
}
func (f *fakeOpenFinanceConnector) FetchTransactions(_ context.Context, accountID string, _, _ time.Time) ([]ports.ExternalTransaction, error) {
	return f.transactions[accountID], nil
}
func (f *fakeOpenFinanceConnector) TriggerSync(context.Context, string) error      { return nil }
func (f *fakeOpenFinanceConnector) DeleteConnection(context.Context, string) error { return nil }

type mockSnapshotRepo struct {
	snapshots []*domain.ProviderBalanceSnapshot
}

func (m *mockSnapshotRepo) Save(_ context.Context, snap *domain.ProviderBalanceSnapshot) error {
	clone := *snap
	m.snapshots = append(m.snapshots, &clone)
	return nil
}
func (m *mockSnapshotRepo) GetLatestByAccount(_ context.Context, tenantID, accountID uuid.UUID) (*domain.ProviderBalanceSnapshot, error) {
	for i := len(m.snapshots) - 1; i >= 0; i-- {
		if m.snapshots[i].TenantID == tenantID && m.snapshots[i].AccountID == accountID {
			return m.snapshots[i], nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockSnapshotRepo) ListByAccount(_ context.Context, tenantID, accountID uuid.UUID, limit int) ([]*domain.ProviderBalanceSnapshot, error) {
	var res []*domain.ProviderBalanceSnapshot
	for _, s := range m.snapshots {
		if s.TenantID == tenantID && s.AccountID == accountID {
			res = append(res, s)
			if limit > 0 && len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func TestOpenFinanceSyncService_SyncItem_EndToEndAndIdempotentResync(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	accRepo := newMockAccountRepo()
	txRepo := newMockTxRepo(accRepo)
	invRepo := newMockInvoiceRepo()
	snapRepo := &mockSnapshotRepo{}
	actionRepo := newMockPendingActionRepo()

	foodCat, err := domain.NewCategory(tenantID, nil, "Alimentação & Mercado", domain.TransactionTypeExpense, true)
	require.NoError(t, err)
	catRepo := &mockCategoryRepoForMatcher{categories: []*domain.Category{foodCat}}

	dayOne := time.Date(2026, 11, 16, 14, 0, 0, 0, domain.BrazilLocation)
	closingNov := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	dueNov := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)

	connector := &fakeOpenFinanceConnector{
		accounts: []ports.ExternalAccount{
			{
				ID:           "ext-chk-1",
				ItemID:       "item-123",
				Type:         ports.ExternalAccountChecking,
				Name:         "Conta Corrente",
				Number:       "0001-9",
				Balance:      decimal.RequireFromString("1800.00"),
				CurrencyCode: "BRL",
			},
			{
				ID:           "ext-cc-1",
				ItemID:       "item-123",
				Type:         ports.ExternalAccountCreditCard,
				Name:         "Mastercard Black",
				Number:       "9999",
				Balance:      decimal.RequireFromString("350.00"),
				CurrencyCode: "BRL",
			},
		},
		bills: map[string][]ports.ExternalBill{
			"ext-cc-1": {
				{
					ID:          "bill-nov",
					DueDate:     dueNov,
					ClosingDate: &closingNov,
					TotalAmount: decimal.RequireFromString("500.00"),
					Status:      "CLOSED",
				},
			},
		},
		transactions: map[string][]ports.ExternalTransaction{
			"ext-chk-1": {
				{
					ID:          "tx-chk-pay-bill",
					AccountID:   "ext-chk-1",
					Date:        time.Date(2026, 11, 15, 12, 0, 0, 0, domain.BrazilLocation),
					Amount:      decimal.RequireFromString("-500.00"),
					Description: "Pagamento Fatura Itaucard Black",
					Status:      "COMPLETED",
				},
				{
					ID:          "tx-chk-future",
					AccountID:   "ext-chk-1",
					Date:        time.Date(2026, 11, 20, 12, 0, 0, 0, domain.BrazilLocation),
					Amount:      decimal.RequireFromString("-200.00"),
					Description: "Agendamento iFood",
					Status:      "COMPLETED",
				},
			},
			"ext-cc-1": {
				{
					ID:           "tx-cc-closed",
					AccountID:    "ext-cc-1",
					BillID:       "bill-nov",
					Date:         time.Date(2026, 11, 5, 12, 0, 0, 0, domain.BrazilLocation),
					Amount:       decimal.RequireFromString("500.00"), // Pluggy positive purchase -> negative expense
					Description:  "Supermercado Pao de Acucar",
					CategoryHint: "Supermarket",
					Status:       "COMPLETED",
				},
				{
					ID:          "tx-cc-future-cycle",
					AccountID:   "ext-cc-1",
					Date:        time.Date(2026, 11, 16, 12, 0, 0, 0, domain.BrazilLocation), // after Nov 8 closing -> 2026-12 synthetic invoice
					Amount:      decimal.RequireFromString("350.00"),
					Description: "Restaurante",
					Status:      "COMPLETED",
				},
			},
		},
	}

	pendingSvc := app.NewPendingActionService(actionRepo, txRepo, catRepo, accRepo, invRepo, nil)
	reconciler := app.NewTransferReconciliationService(txRepo, accRepo, invRepo, pendingSvc)
	matcher := app.NewCategoryMatcher(txRepo, catRepo, pendingSvc)
	var syncSvc ports.OpenFinanceSyncService = app.NewOpenFinanceSyncService(connector, accRepo, txRepo, invRepo, snapRepo, reconciler, matcher, nil)

	// 1. First sync on Nov 16: tx-chk-future (Nov 20) is future-dated -> PENDING, excluded from ledger balance.
	// Liquid balance target = 1800.00 + (-200.00) = 1600.00.
	// Nov closed invoice (500.00) matches checking debit (-500.00) -> marked PAID.
	// Dec synthetic invoice (350.00) stays OPEN -> CC balance = -350.00.
	res1, err := syncSvc.SyncItem(ctx, ports.SyncItemRequest{
		TenantID:      tenantID,
		UserID:        userID,
		ItemID:        "item-123",
		ReferenceTime: dayOne,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, res1.AccountsSynced)
	assert.Equal(t, 2, res1.SnapshotsSaved)
	assert.Equal(t, 1, res1.InvoicesUpserted)
	assert.Equal(t, 1, res1.InvoicesSettled)
	assert.Equal(t, 4, res1.TransactionsCreated)
	assert.Equal(t, 0, res1.TransactionsDeduplicated)
	assert.GreaterOrEqual(t, res1.CategoryActionsProposed, 1)

	accs, err := accRepo.List(ctx, tenantID, ports.AccountFilter{})
	require.NoError(t, err)
	require.Len(t, accs, 2)

	var chkAcc, ccAcc *domain.Account
	for _, a := range accs {
		switch a.Type {
		case domain.AccountTypeChecking:
			chkAcc = a
		case domain.AccountTypeCreditCard:
			ccAcc = a
		}
	}
	require.NotNil(t, chkAcc)
	require.NotNil(t, ccAcc)
	assert.Equal(t, "1600.00", chkAcc.Balance.StringFixed(2))
	assert.Equal(t, "-350.00", ccAcc.Balance.StringFixed(2))

	invoices, err := invRepo.ListByAccount(ctx, tenantID, ccAcc.ID)
	require.NoError(t, err)
	require.Len(t, invoices, 2)
	for _, inv := range invoices {
		if inv.ExternalID != nil && *inv.ExternalID == "bill-nov" {
			assert.Equal(t, domain.InvoiceStatusPaid, inv.Status)
			assert.Equal(t, "500.00", inv.Metadata["paid_amount"])
		} else {
			assert.Equal(t, domain.InvoiceStatusOpen, inv.Status)
			assert.Equal(t, "350.00", inv.TotalAmount.StringFixed(2))
		}
	}

	// Verify future checking tx is PENDING and card purchase has negative amount
	txs, _, err := txRepo.List(ctx, tenantID, ports.TransactionFilter{Limit: 50})
	require.NoError(t, err)
	for _, tx := range txs {
		switch tx.ExternalID {
		case "tx-chk-future":
			assert.Equal(t, domain.TransactionStatusPending, tx.Status)
		case "tx-cc-closed":
			assert.Equal(t, "-500.00", tx.Amount.StringFixed(2))
			assert.Equal(t, domain.TransactionTypeExpense, tx.Type)
			assert.NotNil(t, tx.BillID)
		}
	}

	// 2. Re-sync on Nov 21: tx-chk-future settles from PENDING -> COMPLETED (ledger balance becomes 1400.00)
	// and all 4 transactions are deduplicated without creating duplicates.
	dayTwo := time.Date(2026, 11, 21, 14, 0, 0, 0, domain.BrazilLocation)
	res2, err := syncSvc.SyncItem(ctx, ports.SyncItemRequest{
		TenantID:      tenantID,
		UserID:        userID,
		ItemID:        "item-123",
		ReferenceTime: dayTwo,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res2.TransactionsCreated)
	assert.Equal(t, 4, res2.TransactionsDeduplicated)

	chkAfter, err := accRepo.GetByID(ctx, tenantID, chkAcc.ID)
	require.NoError(t, err)
	assert.Equal(t, "1400.00", chkAfter.Balance.StringFixed(2))
}
