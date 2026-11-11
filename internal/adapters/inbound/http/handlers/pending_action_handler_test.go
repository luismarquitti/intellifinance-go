package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nativeMockPendingActionRepo struct {
	actions map[uuid.UUID]*domain.PendingAction
}

func newNativeMockPendingActionRepo() *nativeMockPendingActionRepo {
	return &nativeMockPendingActionRepo{
		actions: make(map[uuid.UUID]*domain.PendingAction),
	}
}

func (m *nativeMockPendingActionRepo) Save(ctx context.Context, action *domain.PendingAction) error {
	m.actions[action.ID] = action
	return nil
}

func (m *nativeMockPendingActionRepo) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.PendingAction, error) {
	a, exists := m.actions[id]
	if !exists || a.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (m *nativeMockPendingActionRepo) ListPending(ctx context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error) {
	var list []*domain.PendingAction
	for _, a := range m.actions {
		if a.TenantID == tenantID && a.Status == domain.ActionStatusPending {
			list = append(list, a)
		}
	}
	return list, nil
}

func (m *nativeMockPendingActionRepo) UpdateDecision(ctx context.Context, tenantID, id uuid.UUID, status domain.PendingActionStatus, decidedBy *uuid.UUID, reason *string) error {
	a, exists := m.actions[id]
	if !exists || a.TenantID != tenantID {
		return domain.ErrNotFound
	}
	a.Status = status
	a.DecidedByUserID = decidedBy
	a.RejectionReason = reason
	now := time.Now().UTC()
	a.DecidedAt = &now
	return nil
}

func (m *nativeMockPendingActionRepo) MarkExecuted(ctx context.Context, tenantID, id uuid.UUID) error {
	a, exists := m.actions[id]
	if !exists || a.TenantID != tenantID {
		return domain.ErrNotFound
	}
	a.Status = domain.ActionStatusExecuted
	now := time.Now().UTC()
	a.ExecutedAt = &now
	return nil
}

func TestPendingActionHandler_ListPending(t *testing.T) {
	mockRepo := newNativeMockPendingActionRepo()
	handler := handlers.NewPendingActionHandler(mockRepo, nil, nil, nil)

	tenantID := uuid.New()
	action, err := domain.NewPendingAction(tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction, nil)
	require.NoError(t, err)
	require.NoError(t, mockRepo.Save(context.Background(), action))

	req := httptest.NewRequest(http.MethodGet, "/pending-actions", nil)
	ctx := middleware.WithTenantID(req.Context(), tenantID)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ListPending(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp []domain.PendingAction
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp, 1)
	require.Equal(t, action.ID, resp[0].ID)
}

func TestPendingActionHandler_ApproveAndReject(t *testing.T) {
	mockRepo := newNativeMockPendingActionRepo()
	transferSvc := &mockTransferServiceForHandler{}
	handler := handlers.NewPendingActionHandler(mockRepo, nil, transferSvc, nil)

	tenantID := uuid.New()
	userID := uuid.New()
	action, err := domain.NewPendingAction(tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction, nil)
	require.NoError(t, err)
	require.NoError(t, mockRepo.Save(context.Background(), action))

	r := chi.NewRouter()
	r.Post("/pending-actions/{id}/approve", handler.Approve)
	r.Post("/pending-actions/{id}/reject", handler.Reject)

	// Test Approve
	req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+action.ID.String()+"/approve", nil)
	ctx := middleware.WithTenantID(req.Context(), tenantID)
	ctx = middleware.WithUserID(ctx, userID)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, domain.ActionStatusExecuted, mockRepo.actions[action.ID].Status)

	// Test Reject on a new action
	action2, err := domain.NewPendingAction(tenantID, "mcp-server", domain.ActionTypeSplitTransaction, nil)
	require.NoError(t, err)
	require.NoError(t, mockRepo.Save(context.Background(), action2))

	rejectBody, _ := json.Marshal(map[string]string{"reason": "Wrong proportions"})
	req2 := httptest.NewRequest(http.MethodPost, "/pending-actions/"+action2.ID.String()+"/reject", bytes.NewReader(rejectBody))
	req2 = req2.WithContext(ctx)

	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusOK, rec2.Code)
	require.Equal(t, domain.ActionStatusRejected, mockRepo.actions[action2.ID].Status)
	require.Equal(t, "Wrong proportions", *mockRepo.actions[action2.ID].RejectionReason)

	// Test Approve Reconcile Transfer
	outflowID := uuid.New()
	inflowID := uuid.New()
	actionReconcile, err := domain.NewPendingAction(tenantID, "transfer-reconciler", domain.ActionTypeReconcileTransfer, map[string]any{
		"outflow_transaction_id": outflowID.String(),
		"inflow_transaction_id":  inflowID.String(),
		"amount":                 "150.00",
	})
	require.NoError(t, err)
	require.NoError(t, mockRepo.Save(context.Background(), actionReconcile))

	reqReconcile := httptest.NewRequest(http.MethodPost, "/pending-actions/"+actionReconcile.ID.String()+"/approve", nil)
	reqReconcile = reqReconcile.WithContext(ctx)

	recReconcile := httptest.NewRecorder()
	r.ServeHTTP(recReconcile, reqReconcile)

	assert.Equal(t, http.StatusOK, recReconcile.Code)
	assert.Equal(t, domain.ActionStatusExecuted, mockRepo.actions[actionReconcile.ID].Status)
}
