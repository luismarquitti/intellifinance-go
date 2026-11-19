package app_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type stubPendingActionRepo struct {
	mu               sync.Mutex
	actions          map[uuid.UUID]*domain.PendingAction
	returnNilOnEmpty bool
	listErr          error
	saveErr          error
	findErr          error
	updateDecErr     error
	markExecErr      error
}

func newStubPendingActionRepo() *stubPendingActionRepo {
	return &stubPendingActionRepo{
		actions: make(map[uuid.UUID]*domain.PendingAction),
	}
}

func clonePendingAction(src *domain.PendingAction) *domain.PendingAction {
	if src == nil {
		return nil
	}
	cp := *src
	if src.Payload != nil {
		cp.Payload = make(map[string]any, len(src.Payload))
		for k, v := range src.Payload {
			cp.Payload[k] = v
		}
	}
	if src.Metadata != nil {
		cp.Metadata = make(map[string]any, len(src.Metadata))
		for k, v := range src.Metadata {
			cp.Metadata[k] = v
		}
	}
	if src.RejectionReason != nil {
		r := *src.RejectionReason
		cp.RejectionReason = &r
	}
	if src.DecidedByUserID != nil {
		u := *src.DecidedByUserID
		cp.DecidedByUserID = &u
	}
	if src.DecidedAt != nil {
		t := *src.DecidedAt
		cp.DecidedAt = &t
	}
	if src.ExecutedAt != nil {
		t := *src.ExecutedAt
		cp.ExecutedAt = &t
	}
	return &cp
}

func (r *stubPendingActionRepo) Save(_ context.Context, action *domain.PendingAction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return r.saveErr
	}
	r.actions[action.ID] = clonePendingAction(action)
	return nil
}

func (r *stubPendingActionRepo) FindByID(_ context.Context, tenantID, id uuid.UUID) (*domain.PendingAction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findErr != nil {
		return nil, r.findErr
	}
	act, ok := r.actions[id]
	if !ok || act.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return clonePendingAction(act), nil
}

func (r *stubPendingActionRepo) ListPending(_ context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	var out []*domain.PendingAction
	for _, act := range r.actions {
		if act.TenantID == tenantID && act.Status == domain.ActionStatusPending {
			out = append(out, clonePendingAction(act))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	if len(out) == 0 && r.returnNilOnEmpty {
		return nil, nil
	}
	return out, nil
}

func (r *stubPendingActionRepo) UpdateDecision(
	_ context.Context,
	tenantID, id uuid.UUID,
	status domain.PendingActionStatus,
	decidedBy *uuid.UUID,
	reason *string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateDecErr != nil {
		return r.updateDecErr
	}
	act, ok := r.actions[id]
	if !ok || act.TenantID != tenantID || act.Status != domain.ActionStatusPending {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	act.Status = status
	if decidedBy != nil {
		u := *decidedBy
		act.DecidedByUserID = &u
	} else {
		act.DecidedByUserID = nil
	}
	if reason != nil {
		s := *reason
		act.RejectionReason = &s
	} else {
		act.RejectionReason = nil
	}
	act.DecidedAt = &now
	act.UpdatedAt = now
	return nil
}

func (r *stubPendingActionRepo) MarkExecuted(_ context.Context, tenantID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.markExecErr != nil {
		return r.markExecErr
	}
	act, ok := r.actions[id]
	if !ok || act.TenantID != tenantID || act.Status != domain.ActionStatusApproved {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	act.Status = domain.ActionStatusExecuted
	act.ExecutedAt = &now
	act.UpdatedAt = now
	return nil
}

type stubTxRepo struct {
	mu        sync.Mutex
	txs       map[uuid.UUID]*domain.Transaction
	getErr    error
	returnNil bool
	updateErr error
}

func newStubTxRepo() *stubTxRepo {
	return &stubTxRepo{txs: make(map[uuid.UUID]*domain.Transaction)}
}

func (r *stubTxRepo) Create(_ context.Context, tx *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *tx
	r.txs[tx.ID] = &cp
	return nil
}
func (r *stubTxRepo) BatchCreate(_ context.Context, txs []*domain.Transaction) error {
	for _, tx := range txs {
		if err := r.Create(context.Background(), tx); err != nil {
			return err
		}
	}
	return nil
}
func (r *stubTxRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.returnNil {
		return nil, nil
	}
	tx, ok := r.txs[id]
	if !ok || tx.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	cp := *tx
	return &cp, nil
}
func (r *stubTxRepo) List(_ context.Context, tenantID uuid.UUID, _ ports.TransactionFilter) ([]*domain.Transaction, int64, error) {
	return nil, 0, nil
}
func (r *stubTxRepo) Summarize(_ context.Context, _ uuid.UUID, _ ports.TransactionFilter) (*ports.TransactionSummary, error) {
	return nil, nil
}
func (r *stubTxRepo) Update(_ context.Context, tx *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	existing, ok := r.txs[tx.ID]
	if !ok || existing.TenantID != tx.TenantID {
		return domain.ErrNotFound
	}
	cp := *tx
	r.txs[tx.ID] = &cp
	return nil
}
func (r *stubTxRepo) Delete(_ context.Context, _ uuid.UUID, _ uuid.UUID) error { return nil }
func (r *stubTxRepo) BatchDelete(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}
func (r *stubTxRepo) BatchUpdateCategory(_ context.Context, _ uuid.UUID, _ []uuid.UUID, _ *uuid.UUID) error {
	return nil
}
func (r *stubTxRepo) FindExistingFingerprints(_ context.Context, _ uuid.UUID, _ []string) (map[string]bool, error) {
	return nil, nil
}
func (r *stubTxRepo) CreateSplits(_ context.Context, _, _ uuid.UUID, _ []domain.Split) error {
	return nil
}
func (r *stubTxRepo) GetSplits(_ context.Context, _, _ uuid.UUID) ([]domain.Split, error) {
	return nil, nil
}
func (r *stubTxRepo) SetBillID(_ context.Context, _, _, _ uuid.UUID) error { return nil }
func (r *stubTxRepo) ListByBillID(_ context.Context, _, _ uuid.UUID) ([]*domain.Transaction, error) {
	return nil, nil
}
func (r *stubTxRepo) ReconcileExternalTransaction(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID, _ domain.TransactionStatus) error {
	return nil
}

type stubCategoryRepo struct {
	mu               sync.Mutex
	cats             map[uuid.UUID]*domain.Category
	getErr           error
	ignoreTenantOnID bool
}

func newStubCategoryRepo() *stubCategoryRepo {
	return &stubCategoryRepo{cats: make(map[uuid.UUID]*domain.Category)}
}

func (r *stubCategoryRepo) Create(_ context.Context, cat *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *cat
	r.cats[cat.ID] = &cp
	return nil
}
func (r *stubCategoryRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	c, ok := r.cats[id]
	if !ok || (!r.ignoreTenantOnID && c.TenantID != tenantID) {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}
func (r *stubCategoryRepo) List(_ context.Context, _ uuid.UUID, _ ports.CategoryFilter) ([]*domain.Category, error) {
	return nil, nil
}
func (r *stubCategoryRepo) Update(_ context.Context, _ *domain.Category) error { return nil }
func (r *stubCategoryRepo) Delete(_ context.Context, _, _ uuid.UUID) error     { return nil }
func (r *stubCategoryRepo) CountTransactionsByCategory(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}

type stubAccountRepo struct {
	mu               sync.Mutex
	accounts         map[uuid.UUID]*domain.Account
	getErr           error
	updateBalanceErr error
	updateBalCalls   int
}

func newStubAccountRepo() *stubAccountRepo {
	return &stubAccountRepo{accounts: make(map[uuid.UUID]*domain.Account)}
}

func (r *stubAccountRepo) Create(_ context.Context, acc *domain.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *acc
	r.accounts[acc.ID] = &cp
	return nil
}
func (r *stubAccountRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	acc, ok := r.accounts[id]
	if !ok || acc.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	cp := *acc
	return &cp, nil
}
func (r *stubAccountRepo) List(_ context.Context, _ uuid.UUID, _ ports.AccountFilter) ([]*domain.Account, error) {
	return nil, nil
}
func (r *stubAccountRepo) Update(_ context.Context, acc *domain.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *acc
	r.accounts[acc.ID] = &cp
	return nil
}
func (r *stubAccountRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (r *stubAccountRepo) UpdateBalance(_ context.Context, tenantID, id uuid.UUID, delta decimal.Decimal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updateBalCalls++
	if r.updateBalanceErr != nil {
		return r.updateBalanceErr
	}
	acc, ok := r.accounts[id]
	if !ok || acc.TenantID != tenantID {
		return domain.ErrNotFound
	}
	acc.Balance = acc.Balance.Add(delta)
	return nil
}

type stubInvoiceRepo struct {
	mu       sync.Mutex
	invoices map[uuid.UUID]*domain.CreditCardInvoice
	findErr  error
	saveErr  error
	listErr  error
}

func newStubInvoiceRepo() *stubInvoiceRepo {
	return &stubInvoiceRepo{invoices: make(map[uuid.UUID]*domain.CreditCardInvoice)}
}

func cloneInvoice(src *domain.CreditCardInvoice) *domain.CreditCardInvoice {
	cp := *src
	if src.Metadata != nil {
		cp.Metadata = make(map[string]any, len(src.Metadata))
		for k, v := range src.Metadata {
			cp.Metadata[k] = v
		}
	}
	return &cp
}

func (r *stubInvoiceRepo) Save(_ context.Context, inv *domain.CreditCardInvoice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return r.saveErr
	}
	r.invoices[inv.ID] = cloneInvoice(inv)
	return nil
}
func (r *stubInvoiceRepo) FindByID(_ context.Context, tenantID, id uuid.UUID) (*domain.CreditCardInvoice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findErr != nil {
		return nil, r.findErr
	}
	inv, ok := r.invoices[id]
	if !ok || inv.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return cloneInvoice(inv), nil
}
func (r *stubInvoiceRepo) FindByExternalID(_ context.Context, _ uuid.UUID, _ string) (*domain.CreditCardInvoice, error) {
	return nil, domain.ErrNotFound
}
func (r *stubInvoiceRepo) List(_ context.Context, tenantID uuid.UUID, accountID *uuid.UUID) ([]*domain.CreditCardInvoice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	var out []*domain.CreditCardInvoice
	for _, inv := range r.invoices {
		if inv.TenantID == tenantID && (accountID == nil || inv.AccountID == *accountID) {
			out = append(out, cloneInvoice(inv))
		}
	}
	return out, nil
}
func (r *stubInvoiceRepo) ListByAccount(ctx context.Context, tenantID, accountID uuid.UUID) ([]*domain.CreditCardInvoice, error) {
	return r.List(ctx, tenantID, &accountID)
}

type stubTransferService struct {
	txRepo     *stubTxRepo
	confirmErr error
	calls      int
	fn         func(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error)
}

func (s *stubTransferService) ConfirmTransfer(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	s.calls++
	if s.confirmErr != nil {
		return nil, nil, s.confirmErr
	}
	if s.fn != nil {
		return s.fn(ctx, tenantID, legAID, legBID)
	}
	if s.txRepo == nil {
		return nil, nil, errors.New("tx repo missing in stubTransferService")
	}
	legA, errA := s.txRepo.GetByID(ctx, tenantID, legAID)
	if errA != nil {
		return nil, nil, errA
	}
	legB, errB := s.txRepo.GetByID(ctx, tenantID, legBID)
	if errB != nil {
		return nil, nil, errB
	}
	legA.Type = domain.TransactionTypeTransfer
	legA.TransferPeerTransactionID = &legBID
	legB.Type = domain.TransactionTypeTransfer
	legB.TransferPeerTransactionID = &legAID
	if updErrA := s.txRepo.Update(ctx, legA); updErrA != nil {
		return nil, nil, updErrA
	}
	if updErrB := s.txRepo.Update(ctx, legB); updErrB != nil {
		return nil, nil, updErrB
	}
	return legA, legB, nil
}

func (s *stubTransferService) CreateTransfer(_ context.Context, _ ports.CreateTransferParams) (*domain.Transaction, *domain.Transaction, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *stubTransferService) UnlinkTransfer(_ context.Context, _, _ uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	return nil, nil, errors.New("not implemented")
}

func TestPendingActionService_ListPendingAndLoadPendingIndex(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	otherTenantID := uuid.New()

	actionRepo := newStubPendingActionRepo()
	var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, nil, nil, nil)

	t.Run("rejects nil tenant ID", func(t *testing.T) {
		_, err := svc.ListPending(ctx, uuid.Nil)
		require.ErrorIs(t, err, domain.ErrInvalidID)

		_, err = svc.LoadPendingIndex(ctx, uuid.Nil)
		require.ErrorIs(t, err, domain.ErrInvalidID)
	})

	t.Run("normalizes nil repo result to empty non-nil slice and propagates list error", func(t *testing.T) {
		actionRepo.returnNilOnEmpty = true
		got, err := svc.ListPending(ctx, tenantID)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
		actionRepo.returnNilOnEmpty = false

		actionRepo.listErr = errors.New("list failed")
		_, listErr := svc.ListPending(ctx, tenantID)
		require.ErrorContains(t, listErr, "list failed")
		_, idxErr := svc.LoadPendingIndex(ctx, tenantID)
		require.ErrorContains(t, idxErr, "list failed")
		actionRepo.listErr = nil
	})

	t.Run("LoadPendingIndex indexes only active PENDING actions for tenant", func(t *testing.T) {
		txPending := uuid.New()
		txApproved := uuid.New()
		txOtherTenant := uuid.New()
		catID := uuid.New()

		actPending, err := domain.NewCategorizePendingAction(tenantID, "mcp-server", domain.CategorizeTransactionProposal{
			TransactionID:    txPending,
			Description:      "UBER",
			CleanDescription: "UBER",
			Amount:           decimal.RequireFromString("-25.00"),
			Date:             "2026-10-15",
			CategoryID:       catID,
			CategoryName:     "Transporte",
			Confidence:       decimal.RequireFromString("0.90"),
			Reason:           "Uber ride",
		})
		require.NoError(t, err)
		require.NoError(t, actionRepo.Save(ctx, actPending))

		actApproved, err := domain.NewCategorizePendingAction(tenantID, "mcp-server", domain.CategorizeTransactionProposal{
			TransactionID:    txApproved,
			Description:      "IFOOD",
			CleanDescription: "IFOOD",
			Amount:           decimal.RequireFromString("-50.00"),
			Date:             "2026-10-15",
			CategoryID:       catID,
			CategoryName:     "Alimentação",
			Confidence:       decimal.RequireFromString("0.90"),
			Reason:           "Food",
		})
		require.NoError(t, err)
		require.NoError(t, actApproved.Approve(uuid.New()))
		require.NoError(t, actionRepo.Save(ctx, actApproved))

		actOther, err := domain.NewCategorizePendingAction(otherTenantID, "mcp-server", domain.CategorizeTransactionProposal{
			TransactionID:    txOtherTenant,
			Description:      "PADARIA",
			CleanDescription: "PADARIA",
			Amount:           decimal.RequireFromString("-12.00"),
			Date:             "2026-10-15",
			CategoryID:       catID,
			CategoryName:     "Alimentação",
			Confidence:       decimal.RequireFromString("0.90"),
			Reason:           "Bread",
		})
		require.NoError(t, err)
		require.NoError(t, actionRepo.Save(ctx, actOther))

		listed, err := svc.ListPending(ctx, tenantID)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, actPending.ID, listed[0].ID)

		idx, err := svc.LoadPendingIndex(ctx, tenantID)
		require.NoError(t, err)
		require.True(t, idx.HasCategorizationFor(txPending))
		require.False(t, idx.HasCategorizationFor(txApproved))
		require.False(t, idx.HasCategorizationFor(txOtherTenant))
	})
}

func TestPendingActionService_ProposeCategorization(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	otherTenantID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	cat, err := domain.NewCategory(tenantID, nil, "Transporte", domain.TransactionTypeExpense, false)
	require.NoError(t, err)
	otherCat, err := domain.NewCategory(otherTenantID, nil, "Outro", domain.TransactionTypeExpense, false)
	require.NoError(t, err)

	// 2026-10-16 01:30 UTC is 2026-10-15 22:30 in America/Sao_Paulo
	txDate := time.Date(2026, 10, 16, 1, 30, 0, 0, time.UTC)
	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       accountID,
		CreatedByUserID: &userID,
		TransactedAt:    txDate,
		Date:            txDate,
		Amount:          decimal.RequireFromString("-42.50"),
		Description:     "COMPRA CARTAO - UBER *TRIP 15/10",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
	})
	require.NoError(t, err)

	t.Run("hydrates ID-only MCP proposal from Transaction and Category and suppresses duplicates", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		txRepo := newStubTxRepo()
		catRepo := newStubCategoryRepo()
		require.NoError(t, txRepo.Create(ctx, tx))
		require.NoError(t, catRepo.Create(ctx, cat))

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, txRepo, catRepo, nil, nil, nil)

		created, propErr := svc.ProposeCategorization(ctx, tenantID, "mcp-server", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID,
			CategoryID:    cat.ID,
			Confidence:    decimal.RequireFromString("0.91"),
			Reason:        "  Corrida Uber  ",
		})
		require.NoError(t, propErr)
		require.NotNil(t, created)
		require.Equal(t, domain.ActionStatusPending, created.Status)
		require.Equal(t, domain.ActionTypeCategorizeTransaction, created.ActionType)
		require.Equal(t, "mcp-server", created.CreatedByAgent)
		require.Equal(t, tx.Description, created.Payload["description"])
		require.Equal(t, tx.CleanDescription, created.Payload["clean_description"])
		require.Equal(t, "-42.50", created.Payload["amount"])
		require.Equal(t, "2026-10-15", created.Payload["date"])
		require.Equal(t, cat.Name, created.Payload["category_name"])
		require.Equal(t, "0.91", created.Payload["confidence"])
		require.Equal(t, "Corrida Uber", created.Payload["reason"])
		require.Equal(t, "categorize_"+tx.ID.String(), created.Metadata["deduplication_key"])

		// Duplicate proposal for same transaction must fail closed
		otherValidCat, cErr := domain.NewCategory(tenantID, nil, "Lazer", domain.TransactionTypeExpense, false)
		require.NoError(t, cErr)
		require.NoError(t, catRepo.Create(ctx, otherValidCat))

		_, dupErr := svc.ProposeCategorization(ctx, tenantID, "mcp-server", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID,
			CategoryID:    otherValidCat.ID,
			Reason:        "Second try",
		})
		require.ErrorIs(t, dupErr, domain.ErrDuplicatePendingAction)

		listed, listErr := svc.ListPending(ctx, tenantID)
		require.NoError(t, listErr)
		require.Len(t, listed, 1)
	})

	t.Run("accepts pre-hydrated proposal without requiring tx/category repositories", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, nil, nil, nil)

		created, propErr := svc.ProposeCategorization(ctx, tenantID, "ai_categorization_agent", domain.CategorizeTransactionProposal{
			TransactionID:    tx.ID,
			Description:      tx.Description,
			CleanDescription: tx.CleanDescription,
			Amount:           tx.Amount,
			Date:             "2026-10-15",
			CategoryID:       cat.ID,
			CategoryName:     cat.Name,
			Confidence:       decimal.RequireFromString("0.88"),
			Reason:           "Matched keyword",
		})
		require.NoError(t, propErr)
		require.Equal(t, "-42.50", created.Payload["amount"])
	})

	t.Run("allows new proposal after prior proposal was rejected", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		txRepo := newStubTxRepo()
		catRepo := newStubCategoryRepo()
		require.NoError(t, txRepo.Create(ctx, tx))
		require.NoError(t, catRepo.Create(ctx, cat))

		prior, pErr := domain.NewCategorizePendingAction(tenantID, "mcp-server", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, Description: tx.Description, CleanDescription: tx.CleanDescription,
			Amount: tx.Amount, Date: "2026-10-15", CategoryID: cat.ID, CategoryName: cat.Name,
			Confidence: decimal.RequireFromString("0.50"), Reason: "old",
		})
		require.NoError(t, pErr)
		require.NoError(t, prior.Reject(userID, "wrong"))
		require.NoError(t, actionRepo.Save(ctx, prior))

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, txRepo, catRepo, nil, nil, nil)
		created, propErr := svc.ProposeCategorization(ctx, tenantID, "mcp-server", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID,
			CategoryID:    cat.ID,
			Reason:        "retry",
		})
		require.NoError(t, propErr)
		require.Equal(t, domain.ActionStatusPending, created.Status)
	})

	t.Run("validates input IDs, confidence, and pre-hydrated monetary precision", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, nil, nil, nil)

		_, propErr := svc.ProposeCategorization(ctx, uuid.Nil, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: cat.ID,
		})
		require.ErrorIs(t, propErr, domain.ErrInvalidID)

		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: uuid.Nil, CategoryID: cat.ID,
		})
		require.ErrorIs(t, propErr, domain.ErrInvalidPendingActionPayload)
		require.ErrorIs(t, propErr, domain.ErrInvalidID)

		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: uuid.Nil,
		})
		require.ErrorIs(t, propErr, domain.ErrInvalidPendingActionPayload)
		require.ErrorIs(t, propErr, domain.ErrInvalidID)

		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: cat.ID, Confidence: decimal.RequireFromString("1.05"),
		})
		require.ErrorIs(t, propErr, domain.ErrInvalidPendingActionPayload)

		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, Description: "Test", Amount: decimal.RequireFromString("-10.123"),
			Date: "2026-10-15", CategoryID: cat.ID, CategoryName: "Cat",
		})
		require.ErrorIs(t, propErr, domain.ErrInvalidPendingActionPayload)
		require.ErrorIs(t, propErr, domain.ErrInvalidAmount)
	})

	t.Run("fails closed during hydration on missing entities, nil returns, void/transfer txs, or cross-tenant", func(t *testing.T) {
		// Nil repos when hydration needed
		svcNoRepos := app.NewPendingActionService(newStubPendingActionRepo(), nil, nil, nil, nil, nil)
		_, propErr := svcNoRepos.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: cat.ID,
		})
		require.Error(t, propErr)

		// Missing tx (ErrNotFound) and (nil, nil) tx return
		txRepo := newStubTxRepo()
		catRepo := newStubCategoryRepo()
		require.NoError(t, catRepo.Create(ctx, cat))
		svc := app.NewPendingActionService(newStubPendingActionRepo(), txRepo, catRepo, nil, nil, nil)

		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: cat.ID,
		})
		require.ErrorIs(t, propErr, domain.ErrNotFound)

		txRepo.returnNil = true
		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: cat.ID,
		})
		require.ErrorIs(t, propErr, domain.ErrNotFound)
		txRepo.returnNil = false

		// Missing category returning (nil, nil)
		require.NoError(t, txRepo.Create(ctx, tx))
		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: uuid.New(),
		})
		require.ErrorIs(t, propErr, domain.ErrNotFound)

		// Cross-tenant category leaked by repository
		catRepo.ignoreTenantOnID = true
		require.NoError(t, catRepo.Create(ctx, otherCat))
		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: otherCat.ID,
		})
		require.ErrorIs(t, propErr, domain.ErrUnauthorized)
		catRepo.ignoreTenantOnID = false

		// VOID transaction
		voidTx := *tx
		voidTx.ID = uuid.New()
		voidTx.Void()
		require.NoError(t, txRepo.Create(ctx, &voidTx))
		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: voidTx.ID, CategoryID: cat.ID,
		})
		require.ErrorIs(t, propErr, domain.ErrInvalidStatus)

		// TRANSFER transaction
		transferTx := *tx
		transferTx.ID = uuid.New()
		transferTx.Type = domain.TransactionTypeTransfer
		require.NoError(t, txRepo.Create(ctx, &transferTx))
		_, propErr = svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: transferTx.ID, CategoryID: cat.ID,
		})
		require.ErrorIs(t, propErr, domain.ErrInvalidTransactionType)
	})
}

func TestPendingActionService_ProposeTransferReconciliation(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	srcAccID := uuid.New()
	dstAccID := uuid.New()
	outflowID := uuid.New()
	inflowID := uuid.New()
	invoiceID := uuid.New()

	baseProposal := func() domain.ReconcileTransferProposal {
		return domain.ReconcileTransferProposal{
			OutflowTransactionID:   outflowID,
			InflowTransactionID:    inflowID,
			Amount:                 decimal.RequireFromString("750.00"),
			SourceAccountID:        srcAccID,
			SourceAccountName:      "Itaú CC",
			DestinationAccountID:   dstAccID,
			DestinationAccountName: "Cartão Black",
			OutflowDate:            "2026-10-14",
			InflowDate:             "2026-10-15",
			ReconciliationType:     domain.ReconciliationTypeBillPayment,
			Description:            "Pagamento de fatura",
			InvoiceID:              &invoiceID,
		}
	}

	t.Run("persists valid proposal and suppresses exact pair and single-leg collisions", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, nil, nil, nil)

		p := baseProposal()
		act, err := svc.ProposeTransferReconciliation(ctx, tenantID, "transfer-reconciler", p)
		require.NoError(t, err)
		require.Equal(t, domain.ActionStatusPending, act.Status)
		require.Equal(t, domain.ActionTypeReconcileTransfer, act.ActionType)
		require.Equal(t, p.DeduplicationKey(), act.Metadata["deduplication_key"])
		require.Equal(t, invoiceID.String(), act.Payload["invoice_id"])

		// Exact pair duplicate
		_, dupErr := svc.ProposeTransferReconciliation(ctx, tenantID, "transfer-reconciler", p)
		require.ErrorIs(t, dupErr, domain.ErrDuplicatePendingAction)

		// Outflow leg collision with different inflow
		outflowCollision := baseProposal()
		outflowCollision.InflowTransactionID = uuid.New()
		_, outErr := svc.ProposeTransferReconciliation(ctx, tenantID, "transfer-reconciler", outflowCollision)
		require.ErrorIs(t, outErr, domain.ErrDuplicatePendingAction)

		// Inflow leg collision with different outflow
		inflowCollision := baseProposal()
		inflowCollision.OutflowTransactionID = uuid.New()
		_, inErr := svc.ProposeTransferReconciliation(ctx, tenantID, "transfer-reconciler", inflowCollision)
		require.ErrorIs(t, inErr, domain.ErrDuplicatePendingAction)
	})

	t.Run("fails fast on invalid tenant or proposal fields", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, nil, nil, nil)

		_, err := svc.ProposeTransferReconciliation(ctx, uuid.Nil, "rec", baseProposal())
		require.ErrorIs(t, err, domain.ErrInvalidID)

		sameLeg := baseProposal()
		sameLeg.InflowTransactionID = sameLeg.OutflowTransactionID
		_, err = svc.ProposeTransferReconciliation(ctx, tenantID, "rec", sameLeg)
		require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
		require.ErrorIs(t, err, domain.ErrInvalidTransferPair)

		sameAcc := baseProposal()
		sameAcc.DestinationAccountID = sameAcc.SourceAccountID
		_, err = svc.ProposeTransferReconciliation(ctx, tenantID, "rec", sameAcc)
		require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
		require.ErrorIs(t, err, domain.ErrTransferSameAccount)

		subCent := baseProposal()
		subCent.Amount = decimal.RequireFromString("100.005")
		_, err = svc.ProposeTransferReconciliation(ctx, tenantID, "rec", subCent)
		require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
		require.ErrorIs(t, err, domain.ErrInvalidAmount)
	})
}

func TestPendingActionService_ApproveCategorization(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	cat, err := domain.NewCategory(tenantID, nil, "Transporte", domain.TransactionTypeExpense, false)
	require.NoError(t, err)

	newExpenseTx := func() *domain.Transaction {
		tx, txErr := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:        tenantID,
			AccountID:       accountID,
			CreatedByUserID: &userID,
			Date:            time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
			Amount:          decimal.RequireFromString("-45.00"),
			Description:     "UBER *TRIP",
			Type:            domain.TransactionTypeExpense,
			Status:          domain.TransactionStatusCompleted,
		})
		require.NoError(t, txErr)
		return tx
	}

	t.Run("approves categorization, updates transaction with confidence, and transitions PENDING -> APPROVED -> EXECUTED", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		txRepo := newStubTxRepo()
		catRepo := newStubCategoryRepo()
		tx := newExpenseTx()
		require.NoError(t, txRepo.Create(ctx, tx))
		require.NoError(t, catRepo.Create(ctx, cat))

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, txRepo, catRepo, nil, nil, nil)
		proposed, propErr := svc.ProposeCategorization(ctx, tenantID, "ai_categorization_agent", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID,
			CategoryID:    cat.ID,
			Confidence:    decimal.RequireFromString("0.93"),
			Reason:        "Uber match",
		})
		require.NoError(t, propErr)

		approved, appErr := svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.NoError(t, appErr)
		require.Equal(t, domain.ActionStatusExecuted, approved.Status)
		require.NotNil(t, approved.DecidedByUserID)
		require.Equal(t, userID, *approved.DecidedByUserID)
		require.NotNil(t, approved.DecidedAt)
		require.NotNil(t, approved.ExecutedAt)

		persistedAct, findErr := actionRepo.FindByID(ctx, tenantID, proposed.ID)
		require.NoError(t, findErr)
		require.Equal(t, domain.ActionStatusExecuted, persistedAct.Status)

		updatedTx, getErr := txRepo.GetByID(ctx, tenantID, tx.ID)
		require.NoError(t, getErr)
		require.NotNil(t, updatedTx.CategoryID)
		require.Equal(t, cat.ID, *updatedTx.CategoryID)
		require.True(t, updatedTx.IsAICategorized)
		require.True(t, updatedTx.IsManuallyVerified)
		require.NotNil(t, updatedTx.AIConfidence)
		require.True(t, decimal.RequireFromString("0.93").Equal(*updatedTx.AIConfidence))
		require.Equal(t, "ai_categorization_agent", updatedTx.AIModelVersion)
	})

	t.Run("leaves AIConfidence nil when zero and preserves nil DecidedByUserID when decidedBy is uuid.Nil", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		txRepo := newStubTxRepo()
		catRepo := newStubCategoryRepo()
		tx := newExpenseTx()
		require.NoError(t, txRepo.Create(ctx, tx))
		require.NoError(t, catRepo.Create(ctx, cat))

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, txRepo, catRepo, nil, nil, nil)
		proposed, propErr := svc.ProposeCategorization(ctx, tenantID, "mcp-server", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID,
			CategoryID:    cat.ID,
			Reason:        "MCP categorization",
		})
		require.NoError(t, propErr)

		approved, appErr := svc.Approve(ctx, tenantID, proposed.ID, uuid.Nil)
		require.NoError(t, appErr)
		require.Equal(t, domain.ActionStatusExecuted, approved.Status)
		require.Nil(t, approved.DecidedByUserID)

		updatedTx, getErr := txRepo.GetByID(ctx, tenantID, tx.ID)
		require.NoError(t, getErr)
		require.Nil(t, updatedTx.AIConfidence)
		require.Equal(t, "mcp-server", updatedTx.AIModelVersion)
	})

	t.Run("fails closed and leaves action PENDING when transaction missing, corrupt payload, or Update fails", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		txRepo := newStubTxRepo()
		catRepo := newStubCategoryRepo()
		tx := newExpenseTx()
		require.NoError(t, txRepo.Create(ctx, tx))
		require.NoError(t, catRepo.Create(ctx, cat))

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, txRepo, catRepo, nil, nil, nil)

		// Validation & lookup checks
		_, appErr := svc.Approve(ctx, uuid.Nil, uuid.New(), userID)
		require.ErrorIs(t, appErr, domain.ErrInvalidID)
		_, appErr = svc.Approve(ctx, tenantID, uuid.Nil, userID)
		require.ErrorIs(t, appErr, domain.ErrInvalidID)
		_, appErr = svc.Approve(ctx, tenantID, uuid.New(), userID)
		require.ErrorIs(t, appErr, domain.ErrNotFound)

		// Corrupt payload stays PENDING
		corruptAct, cErr := domain.NewPendingAction(tenantID, "mcp", domain.ActionTypeCategorizeTransaction, map[string]any{
			"transaction_id": "not-a-uuid",
		})
		require.NoError(t, cErr)
		require.NoError(t, actionRepo.Save(ctx, corruptAct))
		_, appErr = svc.Approve(ctx, tenantID, corruptAct.ID, userID)
		require.ErrorIs(t, appErr, domain.ErrInvalidPendingActionPayload)
		stillPending, _ := actionRepo.FindByID(ctx, tenantID, corruptAct.ID)
		require.Equal(t, domain.ActionStatusPending, stillPending.Status)

		// Missing transaction at approval time stays PENDING
		missingTxAct, mErr := domain.NewCategorizePendingAction(tenantID, "ai", domain.CategorizeTransactionProposal{
			TransactionID: uuid.New(), Description: "Ghost", CleanDescription: "GHOST",
			Amount: decimal.RequireFromString("-10.00"), Date: "2026-10-15",
			CategoryID: cat.ID, CategoryName: cat.Name, Confidence: decimal.RequireFromString("0.80"),
		})
		require.NoError(t, mErr)
		require.NoError(t, actionRepo.Save(ctx, missingTxAct))
		_, appErr = svc.Approve(ctx, tenantID, missingTxAct.ID, userID)
		require.ErrorIs(t, appErr, domain.ErrNotFound)
		stillPending, _ = actionRepo.FindByID(ctx, tenantID, missingTxAct.ID)
		require.Equal(t, domain.ActionStatusPending, stillPending.Status)

		// Transaction Update failure leaves action PENDING
		proposed, pErr := svc.ProposeCategorization(ctx, tenantID, "mcp", domain.CategorizeTransactionProposal{
			TransactionID: tx.ID, CategoryID: cat.ID, Reason: "Will fail update",
		})
		require.NoError(t, pErr)
		txRepo.updateErr = errors.New("db write failed")
		_, appErr = svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.ErrorContains(t, appErr, "db write failed")
		txRepo.updateErr = nil
		stillPending, findErr := actionRepo.FindByID(ctx, tenantID, proposed.ID)
		require.NoError(t, findErr)
		require.Equal(t, domain.ActionStatusPending, stillPending.Status)

		// UpdateDecision failure leaves action PENDING
		actionRepo.updateDecErr = errors.New("decision error")
		_, appErr = svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.ErrorContains(t, appErr, "decision error")
		actionRepo.updateDecErr = nil
		stillPending, findErr = actionRepo.FindByID(ctx, tenantID, proposed.ID)
		require.NoError(t, findErr)
		require.Equal(t, domain.ActionStatusPending, stillPending.Status)

		// Transaction voided after proposal creation fails closed with ErrInvalidStatus
		voidedTx := *tx
		voidedTx.Void()
		require.NoError(t, txRepo.Update(ctx, &voidedTx))
		_, appErr = svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.ErrorIs(t, appErr, domain.ErrInvalidStatus)
		stillPending, findErr = actionRepo.FindByID(ctx, tenantID, proposed.ID)
		require.NoError(t, findErr)
		require.Equal(t, domain.ActionStatusPending, stillPending.Status)

		// Restore completed transaction, let approval succeed, then verify re-approving returns ErrPendingActionNotPending
		require.NoError(t, txRepo.Update(ctx, tx))
		_, appErr = svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.NoError(t, appErr)
		_, appErr = svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.ErrorIs(t, appErr, domain.ErrPendingActionNotPending)
	})
}

func TestPendingActionService_ApproveTransferReconciliationAndUnsupported(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	t.Run("approves plain INTERNAL_TRANSFER without requiring invoice or account repos", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		txRepo := newStubTxRepo()
		transferSvc := &stubTransferService{txRepo: txRepo}

		srcAccID := uuid.New()
		dstAccID := uuid.New()
		outTx, err := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: srcAccID, CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-500.00"),
			Description: "PIX enviado", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, err)
		inTx, err := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: dstAccID, CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("500.00"),
			Description: "PIX recebido", Type: domain.TransactionTypeIncome, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, err)
		require.NoError(t, txRepo.Create(ctx, outTx))
		require.NoError(t, txRepo.Create(ctx, inTx))

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, txRepo, nil, nil, nil, transferSvc)
		proposal := domain.ReconcileTransferProposal{
			OutflowTransactionID:   outTx.ID,
			InflowTransactionID:    inTx.ID,
			Amount:                 decimal.RequireFromString("500.00"),
			SourceAccountID:        srcAccID,
			SourceAccountName:      "Itaú",
			DestinationAccountID:   dstAccID,
			DestinationAccountName: "Nubank",
			OutflowDate:            "2026-10-15",
			InflowDate:             "2026-10-15",
			ReconciliationType:     domain.ReconciliationTypeInternalTransfer,
			Description:            "Transferência interna",
		}
		proposed, propErr := svc.ProposeTransferReconciliation(ctx, tenantID, "transfer-reconciler", proposal)
		require.NoError(t, propErr)

		approved, appErr := svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.NoError(t, appErr)
		require.Equal(t, domain.ActionStatusExecuted, approved.Status)
		require.Equal(t, 1, transferSvc.calls)

		gotOut, _ := txRepo.GetByID(ctx, tenantID, outTx.ID)
		gotIn, _ := txRepo.GetByID(ctx, tenantID, inTx.ID)
		require.Equal(t, domain.TransactionTypeTransfer, gotOut.Type)
		require.Equal(t, domain.TransactionTypeTransfer, gotIn.Type)
		require.Equal(t, &inTx.ID, gotOut.TransferPeerTransactionID)
		require.Equal(t, &outTx.ID, gotIn.TransferPeerTransactionID)
	})

	t.Run("approves BILL_PAYMENT invoice settlement, stamps metadata, and syncs credit card liability balance", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		txRepo := newStubTxRepo()
		accRepo := newStubAccountRepo()
		invRepo := newStubInvoiceRepo()
		transferSvc := &stubTransferService{txRepo: txRepo}

		checkingAcc, err := domain.NewAccount(tenantID, userID, "Itaú Corrente", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
		require.NoError(t, err)
		checkingAcc.Balance = decimal.RequireFromString("2000.00")
		require.NoError(t, accRepo.Create(ctx, checkingAcc))

		cardAcc, err := domain.NewAccount(tenantID, userID, "Itaú Black", domain.AccountTypeCreditCard, "BRL", domain.VisibilityShared)
		require.NoError(t, err)
		// Card has closed invoice (-800.00) + open invoice (-250.00) = -1050.00 liability
		cardAcc.Balance = decimal.RequireFromString("-1050.00")
		require.NoError(t, accRepo.Create(ctx, cardAcc))

		closedInv, err := domain.NewCreditCardInvoice(tenantID, cardAcc.ID, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), decimal.RequireFromString("800.00"), domain.InvoiceStatusClosed)
		require.NoError(t, err)
		require.NoError(t, invRepo.Save(ctx, closedInv))

		openInv, err := domain.NewCreditCardInvoice(tenantID, cardAcc.ID, time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), decimal.RequireFromString("250.00"), domain.InvoiceStatusOpen)
		require.NoError(t, err)
		require.NoError(t, invRepo.Save(ctx, openInv))

		// 2026-10-15 02:00 UTC -> 2026-10-14 23:00 BRT
		outTime := time.Date(2026, 10, 15, 2, 0, 0, 0, time.UTC)
		outTx, err := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: checkingAcc.ID, CreatedByUserID: &userID,
			TransactedAt: outTime, Date: outTime, Amount: decimal.RequireFromString("-800.00"),
			Description: "PAGAMENTO FATURA ITAU BLACK", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, err)
		inTx, err := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: cardAcc.ID, CreatedByUserID: &userID,
			TransactedAt: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC), Date: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
			Amount: decimal.RequireFromString("800.00"), Description: "Pagamento recebido",
			Type: domain.TransactionTypeIncome, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, err)
		require.NoError(t, txRepo.Create(ctx, outTx))
		require.NoError(t, txRepo.Create(ctx, inTx))

		proposal, propErr := domain.NewReconcileTransferProposalFromEntities(outTx, inTx, checkingAcc, cardAcc, &closedInv.ID)
		require.NoError(t, propErr)

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, txRepo, nil, accRepo, invRepo, transferSvc)
		proposed, pErr := svc.ProposeTransferReconciliation(ctx, tenantID, "transfer-reconciler", proposal)
		require.NoError(t, pErr)

		approved, appErr := svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.NoError(t, appErr)
		require.Equal(t, domain.ActionStatusExecuted, approved.Status)

		settledInv, findErr := invRepo.FindByID(ctx, tenantID, closedInv.ID)
		require.NoError(t, findErr)
		require.Equal(t, domain.InvoiceStatusPaid, settledInv.Status)
		require.Equal(t, outTx.ID.String(), settledInv.Metadata["payment_transaction_id"])
		require.Equal(t, checkingAcc.ID.String(), settledInv.Metadata["payment_account_id"])
		require.Equal(t, "Itaú Corrente", settledInv.Metadata["payment_account_name"])
		require.Equal(t, "2026-10-14", settledInv.Metadata["paid_at"])
		require.Equal(t, "800.00", settledInv.Metadata["paid_amount"])

		// Liability balance synced to remaining open invoice (-250.00)
		updatedCardAcc, cardErr := accRepo.GetByID(ctx, tenantID, cardAcc.ID)
		require.NoError(t, cardErr)
		require.True(t, decimal.RequireFromString("-250.00").Equal(updatedCardAcc.Balance), "got %s", updatedCardAcc.Balance)
		require.Equal(t, 1, accRepo.updateBalCalls)
	})

	t.Run("skips UpdateBalance when credit card liability already matches unpaid invoices and falls back to proposal fields", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		accRepo := newStubAccountRepo()
		invRepo := newStubInvoiceRepo()
		transferSvc := &stubTransferService{
			fn: func(_ context.Context, _ uuid.UUID, _, _ uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
				// Simulate a minimal transfer service returning nil legs
				return nil, nil, nil
			},
		}

		checkingAcc, err := domain.NewAccount(tenantID, userID, "Conta Corrente", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
		require.NoError(t, err)
		require.NoError(t, accRepo.Create(ctx, checkingAcc))

		cardAcc, err := domain.NewAccount(tenantID, userID, "Cartão", domain.AccountTypeCreditCard, "BRL", domain.VisibilityShared)
		require.NoError(t, err)
		cardAcc.Balance = decimal.Zero
		require.NoError(t, accRepo.Create(ctx, cardAcc))

		inv, err := domain.NewCreditCardInvoice(tenantID, cardAcc.ID, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), decimal.RequireFromString("300.00"), domain.InvoiceStatusClosed)
		require.NoError(t, err)
		require.NoError(t, invRepo.Save(ctx, inv))

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, accRepo, invRepo, transferSvc)
		proposed, pErr := svc.ProposeTransferReconciliation(ctx, tenantID, "transfer-reconciler", domain.ReconcileTransferProposal{
			OutflowTransactionID:   uuid.New(),
			InflowTransactionID:    uuid.New(),
			Amount:                 decimal.RequireFromString("300.00"),
			SourceAccountID:        checkingAcc.ID,
			SourceAccountName:      checkingAcc.Name,
			DestinationAccountID:   cardAcc.ID,
			DestinationAccountName: cardAcc.Name,
			OutflowDate:            "2026-10-13",
			InflowDate:             "2026-10-14",
			ReconciliationType:     domain.ReconciliationTypeBillPayment,
			InvoiceID:              &inv.ID,
		})
		require.NoError(t, pErr)

		approved, appErr := svc.Approve(ctx, tenantID, proposed.ID, userID)
		require.NoError(t, appErr)
		require.Equal(t, domain.ActionStatusExecuted, approved.Status)

		settledInv, findErr := invRepo.FindByID(ctx, tenantID, inv.ID)
		require.NoError(t, findErr)
		require.Equal(t, domain.InvoiceStatusPaid, settledInv.Status)
		require.Equal(t, "2026-10-13", settledInv.Metadata["paid_at"])
		require.Equal(t, 0, accRepo.updateBalCalls)
	})

	t.Run("fails closed and leaves action PENDING on transfer confirm, invoice lookup/mismatch/save, or balance update errors", func(t *testing.T) {
		setup := func() (
			*stubPendingActionRepo,
			*stubTxRepo,
			*stubAccountRepo,
			*stubInvoiceRepo,
			*stubTransferService,
			ports.PendingActionService,
			*domain.PendingAction,
			*domain.Account,
			*domain.CreditCardInvoice,
		) {
			actionRepo := newStubPendingActionRepo()
			txRepo := newStubTxRepo()
			accRepo := newStubAccountRepo()
			invRepo := newStubInvoiceRepo()
			transferSvc := &stubTransferService{txRepo: txRepo}

			checkingAcc, _ := domain.NewAccount(tenantID, userID, "Checking", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
			_ = accRepo.Create(ctx, checkingAcc)
			cardAcc, _ := domain.NewAccount(tenantID, userID, "Card", domain.AccountTypeCreditCard, "BRL", domain.VisibilityShared)
			cardAcc.Balance = decimal.RequireFromString("-400.00")
			_ = accRepo.Create(ctx, cardAcc)
			inv, _ := domain.NewCreditCardInvoice(tenantID, cardAcc.ID, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), decimal.RequireFromString("400.00"), domain.InvoiceStatusClosed)
			_ = invRepo.Save(ctx, inv)

			outTx, _ := domain.NewTransaction(domain.NewTransactionParams{
				TenantID: tenantID, AccountID: checkingAcc.ID, CreatedByUserID: &userID,
				Date: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-400.00"),
				Description: "Fatura", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted,
			})
			inTx, _ := domain.NewTransaction(domain.NewTransactionParams{
				TenantID: tenantID, AccountID: cardAcc.ID, CreatedByUserID: &userID,
				Date: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("400.00"),
				Description: "Pagamento recebido", Type: domain.TransactionTypeIncome, Status: domain.TransactionStatusCompleted,
			})
			_ = txRepo.Create(ctx, outTx)
			_ = txRepo.Create(ctx, inTx)

			svc := app.NewPendingActionService(actionRepo, txRepo, nil, accRepo, invRepo, transferSvc)
			prop, pErr := domain.NewReconcileTransferProposalFromEntities(outTx, inTx, checkingAcc, cardAcc, &inv.ID)
			require.NoError(t, pErr)
			act, aErr := svc.ProposeTransferReconciliation(ctx, tenantID, "rec", prop)
			require.NoError(t, aErr)
			return actionRepo, txRepo, accRepo, invRepo, transferSvc, svc, act, cardAcc, inv
		}

		// 1. ConfirmTransfer error
		actionRepo, _, accRepo, invRepo, transferSvc, svc, act, _, inv := setup()
		transferSvc.confirmErr = errors.New("confirm failed")
		_, err := svc.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorContains(t, err, "confirm failed")
		persisted, _ := actionRepo.FindByID(ctx, tenantID, act.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		// 2. Invoice Save error
		actionRepo, _, accRepo, invRepo, transferSvc, svc, act, _, inv = setup()
		invRepo.saveErr = errors.New("invoice save failed")
		_, err = svc.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorContains(t, err, "invoice save failed")
		persisted, _ = actionRepo.FindByID(ctx, tenantID, act.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		// 3. Account UpdateBalance error
		actionRepo, _, accRepo, invRepo, transferSvc, svc, act, _, inv = setup()
		accRepo.updateBalanceErr = errors.New("balance update failed")
		_, err = svc.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorContains(t, err, "balance update failed")
		persisted, _ = actionRepo.FindByID(ctx, tenantID, act.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		// 4. Invoice belongs to a different card account than DestinationAccountID (fails pre-flight before ConfirmTransfer)
		actionRepo, _, accRepo, invRepo, transferSvc, svc, act, _, inv = setup()
		wrongCardInv := *inv
		wrongCardInv.AccountID = uuid.New()
		require.NoError(t, invRepo.Save(ctx, &wrongCardInv))
		_, err = svc.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
		require.Equal(t, 0, transferSvc.calls)
		persisted, _ = actionRepo.FindByID(ctx, tenantID, act.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		// 5. Missing invoice (fails pre-flight before ConfirmTransfer)
		actionRepo, _, accRepo, invRepo, transferSvc, svc, act, _, inv = setup()
		delete(invRepo.invoices, inv.ID)
		_, err = svc.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.Equal(t, 0, transferSvc.calls)
		persisted, _ = actionRepo.FindByID(ctx, tenantID, act.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		// 6. Invoice ListByAccount error during pre-flight aborts before ConfirmTransfer
		actionRepo, _, accRepo, invRepo, transferSvc, svc, act, _, inv = setup()
		invRepo.listErr = errors.New("list invoices failed")
		_, err = svc.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorContains(t, err, "list invoices failed")
		require.Equal(t, 0, transferSvc.calls)
		persisted, _ = actionRepo.FindByID(ctx, tenantID, act.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		// 7. Account GetByID error during pre-flight aborts before ConfirmTransfer
		actionRepo, _, accRepo, invRepo, transferSvc, svc, act, _, inv = setup()
		accRepo.getErr = errors.New("account lookup failed")
		_, err = svc.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorContains(t, err, "account lookup failed")
		require.Equal(t, 0, transferSvc.calls)
		persisted, _ = actionRepo.FindByID(ctx, tenantID, act.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		// 8. Missing card account or nil invoice/account repo fails pre-flight before ConfirmTransfer
		actionRepo, _, accRepo, invRepo, transferSvc, svc, act, cardAcc, inv := setup()
		delete(accRepo.accounts, cardAcc.ID)
		_, err = svc.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.Equal(t, 0, transferSvc.calls)
		persisted, _ = actionRepo.FindByID(ctx, tenantID, act.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		svcMissingRepos := app.NewPendingActionService(actionRepo, nil, nil, nil, nil, transferSvc)
		_, err = svcMissingRepos.Approve(ctx, tenantID, act.ID, userID)
		require.ErrorContains(t, err, "invoice and account repositories are required")
		require.Equal(t, 0, transferSvc.calls)

		// 9. Post-confirm inflowTx account mismatch when proposal DestinationAccountID is nil
		actionRepo, txRepo, accRepo, invRepo, transferSvc, _, _, cardAcc, inv := setup()
		otherCardAcc, _ := domain.NewAccount(tenantID, userID, "Other Card", domain.AccountTypeCreditCard, "BRL", domain.VisibilityShared)
		require.NoError(t, accRepo.Create(ctx, otherCardAcc))
		otherCheckingAcc, _ := domain.NewAccount(tenantID, userID, "Other Checking", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
		require.NoError(t, accRepo.Create(ctx, otherCheckingAcc))
		outTx, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: otherCheckingAcc.ID, CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-400.00"),
			Description: "Fatura", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted,
		})
		mismatchedInTx, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: otherCardAcc.ID, CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("400.00"),
			Description: "Pagamento recebido", Type: domain.TransactionTypeIncome, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, txRepo.Create(ctx, outTx))
		require.NoError(t, txRepo.Create(ctx, mismatchedInTx))
		svcNoDest := app.NewPendingActionService(actionRepo, txRepo, nil, accRepo, invRepo, transferSvc)
		noDestAct, ndErr := svcNoDest.ProposeTransferReconciliation(ctx, tenantID, "rec", domain.ReconcileTransferProposal{
			OutflowTransactionID: outTx.ID,
			InflowTransactionID:  mismatchedInTx.ID,
			Amount:               decimal.RequireFromString("400.00"),
			SourceAccountID:      otherCheckingAcc.ID,
			ReconciliationType:   domain.ReconciliationTypeBillPayment,
			InvoiceID:            &inv.ID,
		})
		require.NoError(t, ndErr)
		_, err = svcNoDest.Approve(ctx, tenantID, noDestAct.ID, userID)
		require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
		require.Equal(t, 1, transferSvc.calls)
		persisted, _ = actionRepo.FindByID(ctx, tenantID, noDestAct.ID)
		require.Equal(t, domain.ActionStatusPending, persisted.Status)

		// 10. Post-confirm reloads liquidAcc when outflowTx.AccountID differs from proposal.SourceAccountID
		inValidCardTx, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: cardAcc.ID, CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("400.00"),
			Description: "Pagamento recebido", Type: domain.TransactionTypeIncome, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, txRepo.Create(ctx, inValidCardTx))
		altCheckingAcc, _ := domain.NewAccount(tenantID, userID, "Actual Liquid Account", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
		require.NoError(t, accRepo.Create(ctx, altCheckingAcc))
		altOutTx, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: altCheckingAcc.ID, CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-400.00"),
			Description: "Fatura", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, txRepo.Create(ctx, altOutTx))
		transferSvc.calls = 0
		reloadAct, rlErr := svcNoDest.ProposeTransferReconciliation(ctx, tenantID, "rec", domain.ReconcileTransferProposal{
			OutflowTransactionID: altOutTx.ID,
			InflowTransactionID:  inValidCardTx.ID,
			Amount:               decimal.RequireFromString("400.00"),
			SourceAccountID:      otherCheckingAcc.ID, // differs from altOutTx.AccountID
			DestinationAccountID: cardAcc.ID,
			ReconciliationType:   domain.ReconciliationTypeBillPayment,
			InvoiceID:            &inv.ID,
		})
		require.NoError(t, rlErr)
		approvedReload, rlAppErr := svcNoDest.Approve(ctx, tenantID, reloadAct.ID, userID)
		require.NoError(t, rlAppErr)
		require.Equal(t, domain.ActionStatusExecuted, approvedReload.Status)
		settledReloadInv, fErr := invRepo.FindByID(ctx, tenantID, inv.ID)
		require.NoError(t, fErr)
		require.Equal(t, altCheckingAcc.ID.String(), settledReloadInv.Metadata["payment_account_id"])
		require.Equal(t, "Actual Liquid Account", settledReloadInv.Metadata["payment_account_name"])
	})

	t.Run("rejects unsupported action types with ErrUnsupportedPendingActionType and keeps them PENDING", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, nil, nil, nil)

		for _, actionType := range []domain.PendingActionType{
			domain.ActionTypeSplitTransaction,
			domain.ActionTypeBalanceAdjustment,
			domain.PendingActionType("UNKNOWN_ACTION"),
		} {
			act, err := domain.NewPendingAction(tenantID, "mcp", actionType, map[string]any{"note": "parked"})
			require.NoError(t, err)
			require.NoError(t, actionRepo.Save(ctx, act))

			_, appErr := svc.Approve(ctx, tenantID, act.ID, userID)
			require.ErrorIs(t, appErr, domain.ErrUnsupportedPendingActionType)

			persisted, findErr := actionRepo.FindByID(ctx, tenantID, act.ID)
			require.NoError(t, findErr)
			require.Equal(t, domain.ActionStatusPending, persisted.Status)
		}
	})
}

func TestPendingActionService_RejectAndBatchDecisions(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	cat, err := domain.NewCategory(tenantID, nil, "Alimentação", domain.TransactionTypeExpense, false)
	require.NoError(t, err)

	t.Run("Reject trims reason, handles nil reason and nil decider, and rejects non-pending actions", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, nil, nil, nil)

		_, rejErr := svc.Reject(ctx, uuid.Nil, uuid.New(), userID, "x")
		require.ErrorIs(t, rejErr, domain.ErrInvalidID)
		_, rejErr = svc.Reject(ctx, tenantID, uuid.Nil, userID, "x")
		require.ErrorIs(t, rejErr, domain.ErrInvalidID)
		_, rejErr = svc.Reject(ctx, tenantID, uuid.New(), userID, "x")
		require.ErrorIs(t, rejErr, domain.ErrNotFound)

		act1, aErr := domain.NewPendingAction(tenantID, "mcp", domain.ActionTypeCategorizeTransaction, map[string]any{"k": "1"})
		require.NoError(t, aErr)
		require.NoError(t, actionRepo.Save(ctx, act1))

		rejected1, rErr := svc.Reject(ctx, tenantID, act1.ID, userID, "  categoria errada  ")
		require.NoError(t, rErr)
		require.Equal(t, domain.ActionStatusRejected, rejected1.Status)
		require.NotNil(t, rejected1.RejectionReason)
		require.Equal(t, "categoria errada", *rejected1.RejectionReason)
		require.NotNil(t, rejected1.DecidedByUserID)
		require.Equal(t, userID, *rejected1.DecidedByUserID)

		// Re-rejecting already rejected action fails with ErrPendingActionNotPending
		_, rErr2 := svc.Reject(ctx, tenantID, act1.ID, userID, "again")
		require.ErrorIs(t, rErr2, domain.ErrPendingActionNotPending)

		// Blank reason & nil decider
		act2, aErr2 := domain.NewPendingAction(tenantID, "mcp", domain.ActionTypeCategorizeTransaction, map[string]any{"k": "2"})
		require.NoError(t, aErr2)
		require.NoError(t, actionRepo.Save(ctx, act2))

		rejected2, rErr3 := svc.Reject(ctx, tenantID, act2.ID, uuid.Nil, "   \t\n ")
		require.NoError(t, rErr3)
		require.Equal(t, domain.ActionStatusRejected, rejected2.Status)
		require.Nil(t, rejected2.RejectionReason)
		require.Nil(t, rejected2.DecidedByUserID)
	})

	t.Run("ApproveBatch filters by selector and isolates per-action execution failures", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		txRepo := newStubTxRepo()
		catRepo := newStubCategoryRepo()
		require.NoError(t, catRepo.Create(ctx, cat))
		transferSvc := &stubTransferService{txRepo: txRepo}

		validTx, txErr := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: accountID, CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-60.00"),
			Description: "IFOOD", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, txErr)
		require.NoError(t, txRepo.Create(ctx, validTx))

		outTx, oErr := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: accountID, CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-100.00"),
			Description: "PIX enviado", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, oErr)
		inTx, iErr := domain.NewTransaction(domain.NewTransactionParams{
			TenantID: tenantID, AccountID: uuid.New(), CreatedByUserID: &userID,
			Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("100.00"),
			Description: "PIX recebido", Type: domain.TransactionTypeIncome, Status: domain.TransactionStatusCompleted,
		})
		require.NoError(t, iErr)
		require.NoError(t, txRepo.Create(ctx, outTx))
		require.NoError(t, txRepo.Create(ctx, inTx))

		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, txRepo, catRepo, nil, nil, transferSvc)

		_, invErr := svc.ApproveBatch(ctx, uuid.Nil, userID, domain.PendingActionSelector{All: true})
		require.ErrorIs(t, invErr, domain.ErrInvalidID)

		// Empty selector returns zero counts and non-nil empty Errors slice
		emptyRes, emptyErr := svc.ApproveBatch(ctx, tenantID, userID, domain.PendingActionSelector{})
		require.NoError(t, emptyErr)
		require.Equal(t, 0, emptyRes.ProcessedCount)
		require.NotNil(t, emptyRes.Errors)
		require.Empty(t, emptyRes.Errors)

		// Propose 1 valid categorization, 1 broken categorization (missing tx), 1 valid transfer
		actValidCat, err1 := svc.ProposeCategorization(ctx, tenantID, "ai", domain.CategorizeTransactionProposal{
			TransactionID: validTx.ID, CategoryID: cat.ID, Confidence: decimal.RequireFromString("0.90"),
		})
		require.NoError(t, err1)

		actBrokenCat, err2 := domain.NewCategorizePendingAction(tenantID, "ai", domain.CategorizeTransactionProposal{
			TransactionID: uuid.New(), Description: "Missing", CleanDescription: "MISSING",
			Amount: decimal.RequireFromString("-10.00"), Date: "2026-10-15",
			CategoryID: cat.ID, CategoryName: cat.Name, Confidence: decimal.RequireFromString("0.80"),
		})
		require.NoError(t, err2)
		require.NoError(t, actionRepo.Save(ctx, actBrokenCat))

		actValidTransfer, err3 := svc.ProposeTransferReconciliation(ctx, tenantID, "rec", domain.ReconcileTransferProposal{
			OutflowTransactionID: outTx.ID, InflowTransactionID: inTx.ID,
			Amount: decimal.RequireFromString("100.00"), SourceAccountID: outTx.AccountID, DestinationAccountID: inTx.AccountID,
			ReconciliationType: domain.ReconciliationTypeInternalTransfer,
		})
		require.NoError(t, err3)

		// Batch approve filtered to ActionTypeCategorizeTransaction only
		catType := domain.ActionTypeCategorizeTransaction
		catBatchRes, catBatchErr := svc.ApproveBatch(ctx, tenantID, userID, domain.PendingActionSelector{
			All:        true,
			ActionType: &catType,
		})
		require.NoError(t, catBatchErr)
		require.Equal(t, 2, catBatchRes.ProcessedCount)
		require.Equal(t, 1, catBatchRes.ApprovedCount)
		require.Equal(t, 0, catBatchRes.RejectedCount)
		require.Equal(t, 1, catBatchRes.FailedCount)
		require.Len(t, catBatchRes.Errors, 1)
		require.Contains(t, catBatchRes.Errors[0], "action "+actBrokenCat.ID.String()+":")

		// Verify valid categorization executed, broken stayed PENDING, and transfer was untouched
		gotValidCat, _ := actionRepo.FindByID(ctx, tenantID, actValidCat.ID)
		require.Equal(t, domain.ActionStatusExecuted, gotValidCat.Status)
		gotBrokenCat, _ := actionRepo.FindByID(ctx, tenantID, actBrokenCat.ID)
		require.Equal(t, domain.ActionStatusPending, gotBrokenCat.Status)
		gotTransfer, _ := actionRepo.FindByID(ctx, tenantID, actValidTransfer.ID)
		require.Equal(t, domain.ActionStatusPending, gotTransfer.Status)

		// Batch approve explicit ActionIDs for transfer
		trBatchRes, trBatchErr := svc.ApproveBatch(ctx, tenantID, userID, domain.PendingActionSelector{
			ActionIDs: []uuid.UUID{actValidTransfer.ID},
		})
		require.NoError(t, trBatchErr)
		require.Equal(t, 1, trBatchRes.ProcessedCount)
		require.Equal(t, 1, trBatchRes.ApprovedCount)
		require.Equal(t, 0, trBatchRes.FailedCount)
		require.NotNil(t, trBatchRes.Errors)
		require.Empty(t, trBatchRes.Errors)
	})

	t.Run("RejectBatch filters by selector, trims reason, and isolates UpdateDecision failures", func(t *testing.T) {
		actionRepo := newStubPendingActionRepo()
		var svc ports.PendingActionService = app.NewPendingActionService(actionRepo, nil, nil, nil, nil, nil)

		_, invErr := svc.RejectBatch(ctx, uuid.Nil, userID, domain.PendingActionSelector{All: true}, "reason")
		require.ErrorIs(t, invErr, domain.ErrInvalidID)

		act1, _ := domain.NewPendingAction(tenantID, "mcp", domain.ActionTypeCategorizeTransaction, nil)
		act2, _ := domain.NewPendingAction(tenantID, "rec", domain.ActionTypeReconcileTransfer, nil)
		act3, _ := domain.NewPendingAction(tenantID, "mcp", domain.ActionTypeCategorizeTransaction, nil)
		require.NoError(t, actionRepo.Save(ctx, act1))
		require.NoError(t, actionRepo.Save(ctx, act2))
		require.NoError(t, actionRepo.Save(ctx, act3))

		// Reject only act1 and act3 by ActionIDs
		res, rejErr := svc.RejectBatch(ctx, tenantID, userID, domain.PendingActionSelector{
			ActionIDs: []uuid.UUID{act1.ID, act3.ID},
		}, "  batch rejected  ")
		require.NoError(t, rejErr)
		require.Equal(t, 2, res.ProcessedCount)
		require.Equal(t, 0, res.ApprovedCount)
		require.Equal(t, 2, res.RejectedCount)
		require.Equal(t, 0, res.FailedCount)
		require.NotNil(t, res.Errors)
		require.Empty(t, res.Errors)

		got1, _ := actionRepo.FindByID(ctx, tenantID, act1.ID)
		require.Equal(t, domain.ActionStatusRejected, got1.Status)
		require.NotNil(t, got1.RejectionReason)
		require.Equal(t, "batch rejected", *got1.RejectionReason)

		got2, _ := actionRepo.FindByID(ctx, tenantID, act2.ID)
		require.Equal(t, domain.ActionStatusPending, got2.Status)

		// Simulate UpdateDecision error on remaining act2
		actionRepo.updateDecErr = errors.New("decision write error")
		failRes, failErr := svc.RejectBatch(ctx, tenantID, userID, domain.PendingActionSelector{All: true}, "cleanup")
		require.NoError(t, failErr)
		require.Equal(t, 1, failRes.ProcessedCount)
		require.Equal(t, 0, failRes.RejectedCount)
		require.Equal(t, 1, failRes.FailedCount)
		require.Len(t, failRes.Errors, 1)
		require.Contains(t, failRes.Errors[0], "action "+act2.ID.String()+":")
	})
}
