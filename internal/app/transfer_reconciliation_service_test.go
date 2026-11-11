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

type mockPendingActionRepo struct {
	actions map[uuid.UUID]*domain.PendingAction
}

func newMockPendingActionRepo() *mockPendingActionRepo {
	return &mockPendingActionRepo{actions: make(map[uuid.UUID]*domain.PendingAction)}
}

func (m *mockPendingActionRepo) Save(ctx context.Context, action *domain.PendingAction) error {
	clone := *action
	m.actions[action.ID] = &clone
	return nil
}

func (m *mockPendingActionRepo) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.PendingAction, error) {
	act, ok := m.actions[id]
	if !ok || act.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	clone := *act
	return &clone, nil
}

func (m *mockPendingActionRepo) ListPending(ctx context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error) {
	var res []*domain.PendingAction
	for _, act := range m.actions {
		if act.TenantID == tenantID && act.Status == domain.ActionStatusPending {
			clone := *act
			res = append(res, &clone)
		}
	}
	return res, nil
}

func (m *mockPendingActionRepo) UpdateDecision(ctx context.Context, tenantID, id uuid.UUID, status domain.PendingActionStatus, decidedBy *uuid.UUID, reason *string) error {
	act, ok := m.actions[id]
	if !ok || act.TenantID != tenantID {
		return domain.ErrNotFound
	}
	act.Status = status
	act.DecidedByUserID = decidedBy
	act.RejectionReason = reason
	return nil
}

func (m *mockPendingActionRepo) MarkExecuted(ctx context.Context, tenantID, id uuid.UUID) error {
	act, ok := m.actions[id]
	if !ok || act.TenantID != tenantID {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	act.Status = domain.ActionStatusExecuted
	act.ExecutedAt = &now
	return nil
}

func TestTransferReconciliationService_ScanAndPropose(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	accRepo := newMockAccountRepo()
	txRepo := newMockTxRepo(accRepo)
	invRepo := newMockInvoiceRepo()
	actionRepo := newMockPendingActionRepo()

	reconciler := app.NewTransferReconciliationService(txRepo, accRepo, invRepo, actionRepo)

	// Create Accounts: 2 checking, 1 credit card
	checkingA, _ := domain.NewAccount(tenantID, userID, "Itaú Corrente", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	_ = accRepo.Create(ctx, checkingA)

	checkingB, _ := domain.NewAccount(tenantID, userID, "Nubank Corrente", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	_ = accRepo.Create(ctx, checkingB)

	ccAcc, _ := domain.NewAccount(tenantID, userID, "Nubank Ultravioleta", domain.AccountTypeCreditCard, "BRL", domain.VisibilityShared)
	_ = accRepo.Create(ctx, ccAcc)

	now := time.Now().UTC()

	// 1. Internal Transfer pair between Checking A and Checking B: R$ 500.00
	outflowTransfer, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       checkingA.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromInt(-500),
		Description:     "Transferência enviada PIX",
		Type:            domain.TransactionTypeExpense,
		Date:            now,
	})
	_ = txRepo.Create(ctx, outflowTransfer)

	inflowTransfer, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       checkingB.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromInt(500),
		Description:     "Transferência recebida PIX",
		Type:            domain.TransactionTypeIncome,
		Date:            now.Add(time.Hour),
	})
	_ = txRepo.Create(ctx, inflowTransfer)

	// 2. Bill Payment pair: Checking A -> CC: R$ 1200.00
	outflowBill, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       checkingA.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromInt(-1200),
		Description:     "Pagamento Boleto Fatura",
		Type:            domain.TransactionTypeExpense,
		Date:            now.AddDate(0, 0, -1),
	})
	_ = txRepo.Create(ctx, outflowBill)

	inflowBill, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       ccAcc.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromInt(1200),
		Description:     "Pagamento recebido fatura",
		Type:            domain.TransactionTypeIncome,
		Date:            now,
	})
	_ = txRepo.Create(ctx, inflowBill)

	// 3. Unrelated single transaction: R$ 80.00 padaria
	unrelated, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       checkingA.ID,
		CreatedByUserID: &userID,
		Amount:          decimal.NewFromInt(-80),
		Description:     "Padaria Central",
		Type:            domain.TransactionTypeExpense,
		Date:            now,
	})
	_ = txRepo.Create(ctx, unrelated)

	// Run Scan
	result, err := reconciler.ScanAndPropose(ctx, tenantID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 5, result.ScannedTransactionsCount)
	assert.Equal(t, 2, result.ProposedActionsCount, "must identify exactly 2 pairs")

	// Verify generated actions in actionRepo
	pendingList, err := actionRepo.ListPending(ctx, tenantID)
	require.NoError(t, err)
	assert.Len(t, pendingList, 2)

	reconcilTypes := make(map[string]bool)
	for _, act := range pendingList {
		assert.Equal(t, domain.ActionTypeReconcileTransfer, act.ActionType)
		rType := act.Payload["reconciliation_type"].(string)
		reconcilTypes[rType] = true
	}
	assert.True(t, reconcilTypes["INTERNAL_TRANSFER"])
	assert.True(t, reconcilTypes["BILL_PAYMENT"])

	// Idempotency test: Running scan again should propose 0 new actions
	result2, err := reconciler.ScanAndPropose(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.ProposedActionsCount, "idempotent scan must not duplicate pending proposals")
}
