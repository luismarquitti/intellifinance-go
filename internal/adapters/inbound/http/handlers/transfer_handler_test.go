package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type mockTransferServiceForHandler struct {
	confirmFunc func(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error)
	createFunc  func(ctx context.Context, params ports.CreateTransferParams) (*domain.Transaction, *domain.Transaction, error)
	unlinkFunc  func(ctx context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, *domain.Transaction, error)
}

func (m *mockTransferServiceForHandler) ConfirmTransfer(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	if m.confirmFunc != nil {
		return m.confirmFunc(ctx, tenantID, legAID, legBID)
	}
	t1 := &domain.Transaction{ID: legAID, TenantID: tenantID, Amount: decimal.NewFromFloat(-100), Type: domain.TransactionTypeTransfer, TransferPeerTransactionID: &legBID}
	t2 := &domain.Transaction{ID: legBID, TenantID: tenantID, Amount: decimal.NewFromFloat(100), Type: domain.TransactionTypeTransfer, TransferPeerTransactionID: &legAID}
	return t1, t2, nil
}

func (m *mockTransferServiceForHandler) CreateTransfer(ctx context.Context, params ports.CreateTransferParams) (*domain.Transaction, *domain.Transaction, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, params)
	}
	id1, id2 := uuid.New(), uuid.New()
	t1 := &domain.Transaction{ID: id1, TenantID: params.TenantID, AccountID: params.SourceAccountID, Amount: params.Amount.Neg(), Type: domain.TransactionTypeTransfer, TransferPeerTransactionID: &id2}
	t2 := &domain.Transaction{ID: id2, TenantID: params.TenantID, AccountID: params.DestinationAccountID, Amount: params.Amount, Type: domain.TransactionTypeTransfer, TransferPeerTransactionID: &id1}
	return t1, t2, nil
}

func (m *mockTransferServiceForHandler) UnlinkTransfer(ctx context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	if m.unlinkFunc != nil {
		return m.unlinkFunc(ctx, tenantID, txID)
	}
	id2 := uuid.New()
	t1 := &domain.Transaction{ID: txID, TenantID: tenantID, Amount: decimal.NewFromFloat(-100), Type: domain.TransactionTypeExpense}
	t2 := &domain.Transaction{ID: id2, TenantID: tenantID, Amount: decimal.NewFromFloat(100), Type: domain.TransactionTypeIncome}
	return t1, t2, nil
}

type mockReconcileServiceForHandler struct {
	result *ports.ReconciliationScanResult
	err    error
}

func (m *mockReconcileServiceForHandler) ScanAndPropose(ctx context.Context, tenantID uuid.UUID) (*ports.ReconciliationScanResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &ports.ReconciliationScanResult{
		ScannedTransactionsCount: 10,
		ProposedActionsCount:     1,
		ProposedActions:          []*domain.PendingAction{},
	}, nil
}

func TestTransferHandler_ReconcileScan(t *testing.T) {
	tenantID := uuid.New()
	reconcileSvc := &mockReconcileServiceForHandler{}
	handler := handlers.NewTransferHandler(nil, reconcileSvc)

	req := httptest.NewRequest(http.MethodPost, "/transfers/reconcile-scan", nil)
	req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

	rr := httptest.NewRecorder()
	handler.ReconcileScan(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp ports.ReconciliationScanResult
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, 10, resp.ScannedTransactionsCount)
	require.Equal(t, 1, resp.ProposedActionsCount)
}

func TestTransferHandler_ConfirmTransfer(t *testing.T) {
	tenantID := uuid.New()
	idA := uuid.New()
	idB := uuid.New()

	svc := &mockTransferServiceForHandler{}
	handler := handlers.NewTransferHandler(svc, nil)

	t.Run("success 200", func(t *testing.T) {
		body, _ := json.Marshal(dto.ConfirmTransferRequest{
			SourceTransactionID:      idA.String(),
			DestinationTransactionID: idB.String(),
		})

		req := httptest.NewRequest(http.MethodPost, "/transfers/confirm", bytes.NewReader(body))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rr := httptest.NewRecorder()
		handler.ConfirmTransfer(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.TransferResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		require.Equal(t, idA, resp.Outflow.ID)
		require.Equal(t, idB, resp.Inflow.ID)
	})

	t.Run("validation failed 422 same transaction", func(t *testing.T) {
		body, _ := json.Marshal(dto.ConfirmTransferRequest{
			SourceTransactionID:      idA.String(),
			DestinationTransactionID: idA.String(), // Same!
		})

		req := httptest.NewRequest(http.MethodPost, "/transfers/confirm", bytes.NewReader(body))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rr := httptest.NewRecorder()
		handler.ConfirmTransfer(rr, req)

		require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})

	t.Run("conflict 409 already linked", func(t *testing.T) {
		svc.confirmFunc = func(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
			return nil, nil, domain.ErrTransferAlreadyLinked
		}

		body, _ := json.Marshal(dto.ConfirmTransferRequest{
			SourceTransactionID:      idA.String(),
			DestinationTransactionID: idB.String(),
		})

		req := httptest.NewRequest(http.MethodPost, "/transfers/confirm", bytes.NewReader(body))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rr := httptest.NewRecorder()
		handler.ConfirmTransfer(rr, req)

		require.Equal(t, http.StatusConflict, rr.Code)
	})
}

func TestTransferHandler_CreateTransfer(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	accA := uuid.New()
	accB := uuid.New()

	svc := &mockTransferServiceForHandler{}
	handler := handlers.NewTransferHandler(svc, nil)

	t.Run("success 201", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateTransferRequest{
			SourceAccountID:      accA.String(),
			DestinationAccountID: accB.String(),
			Amount:               "250.00",
			Date:                 time.Now().Format("2006-01-02"),
			Description:          "Transferência entre contas",
			IdempotencyKey:       "synthetic-handler",
		})

		req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewReader(body))
		ctx := middleware.WithTenantID(req.Context(), tenantID)
		ctx = middleware.WithUserID(ctx, userID)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.CreateTransfer(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)

		var resp dto.TransferResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		require.Equal(t, accA, resp.Outflow.AccountID)
		require.Equal(t, accB, resp.Inflow.AccountID)
	})

	t.Run("validation failed 422 same account", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateTransferRequest{
			SourceAccountID:      accA.String(),
			DestinationAccountID: accA.String(),
			Amount:               "250.00",
			Date:                 time.Now().Format("2006-01-02"),
		})

		req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewReader(body))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rr := httptest.NewRecorder()
		handler.CreateTransfer(rr, req)

		require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})

	t.Run("validation failed 422 negative amount", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateTransferRequest{
			SourceAccountID:      accA.String(),
			DestinationAccountID: accB.String(),
			Amount:               "-250.00",
			Date:                 time.Now().Format("2006-01-02"),
		})

		req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewReader(body))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rr := httptest.NewRecorder()
		handler.CreateTransfer(rr, req)

		require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})
}

func TestTransferHandler_UnlinkTransfer(t *testing.T) {
	tenantID := uuid.New()
	txID := uuid.New()

	svc := &mockTransferServiceForHandler{}
	handler := handlers.NewTransferHandler(svc, nil)

	t.Run("success 200", func(t *testing.T) {
		body, _ := json.Marshal(dto.UnlinkTransferRequest{
			TransactionID: txID.String(),
		})

		req := httptest.NewRequest(http.MethodPost, "/transfers/unlink", bytes.NewReader(body))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rr := httptest.NewRecorder()
		handler.UnlinkTransfer(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("not linked 422", func(t *testing.T) {
		svc.unlinkFunc = func(ctx context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
			return nil, nil, domain.ErrTransferNotLinked
		}

		body, _ := json.Marshal(dto.UnlinkTransferRequest{
			TransactionID: txID.String(),
		})

		req := httptest.NewRequest(http.MethodPost, "/transfers/unlink", bytes.NewReader(body))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rr := httptest.NewRecorder()
		handler.UnlinkTransfer(rr, req)

		require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})
}
