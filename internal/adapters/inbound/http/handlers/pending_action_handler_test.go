package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPendingActionService struct {
	listFn         func(ctx context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error)
	approveFn      func(ctx context.Context, tenantID, actionID, decidedBy uuid.UUID) (*domain.PendingAction, error)
	rejectFn       func(ctx context.Context, tenantID, actionID, decidedBy uuid.UUID, reason string) (*domain.PendingAction, error)
	approveBatchFn func(ctx context.Context, tenantID, decidedBy uuid.UUID, selector domain.PendingActionSelector) (*ports.BatchDecisionResult, error)
	rejectBatchFn  func(ctx context.Context, tenantID, decidedBy uuid.UUID, selector domain.PendingActionSelector, reason string) (*ports.BatchDecisionResult, error)
	gotTenantID    uuid.UUID
	gotActionID    uuid.UUID
	gotDecidedBy   uuid.UUID
	gotReason      string
	gotSelector    domain.PendingActionSelector
	approveCalled  bool
	rejectCalled   bool
	batchAppCalled bool
	batchRejCalled bool
}

var _ ports.PendingActionService = (*stubPendingActionService)(nil)

func (s *stubPendingActionService) ListPending(ctx context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error) {
	s.gotTenantID = tenantID
	if s.listFn != nil {
		return s.listFn(ctx, tenantID)
	}
	return []*domain.PendingAction{}, nil
}

func (s *stubPendingActionService) LoadPendingIndex(_ context.Context, _ uuid.UUID) (*domain.PendingActionIndex, error) {
	return domain.NewPendingActionIndex(nil), nil
}

func (s *stubPendingActionService) ProposeCategorization(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	_ domain.CategorizeTransactionProposal,
) (*domain.PendingAction, error) {
	return nil, errors.New("not used in handler tests")
}

func (s *stubPendingActionService) ProposeTransferReconciliation(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	_ domain.ReconcileTransferProposal,
) (*domain.PendingAction, error) {
	return nil, errors.New("not used in handler tests")
}

func (s *stubPendingActionService) Approve(ctx context.Context, tenantID, actionID, decidedBy uuid.UUID) (*domain.PendingAction, error) {
	s.approveCalled = true
	s.gotTenantID = tenantID
	s.gotActionID = actionID
	s.gotDecidedBy = decidedBy
	if s.approveFn != nil {
		return s.approveFn(ctx, tenantID, actionID, decidedBy)
	}
	return &domain.PendingAction{ID: actionID, TenantID: tenantID, Status: domain.ActionStatusExecuted}, nil
}

func (s *stubPendingActionService) Reject(ctx context.Context, tenantID, actionID, decidedBy uuid.UUID, reason string) (*domain.PendingAction, error) {
	s.rejectCalled = true
	s.gotTenantID = tenantID
	s.gotActionID = actionID
	s.gotDecidedBy = decidedBy
	s.gotReason = reason
	if s.rejectFn != nil {
		return s.rejectFn(ctx, tenantID, actionID, decidedBy, reason)
	}
	return &domain.PendingAction{ID: actionID, TenantID: tenantID, Status: domain.ActionStatusRejected}, nil
}

func (s *stubPendingActionService) ApproveBatch(
	ctx context.Context,
	tenantID, decidedBy uuid.UUID,
	selector domain.PendingActionSelector,
) (*ports.BatchDecisionResult, error) {
	s.batchAppCalled = true
	s.gotTenantID = tenantID
	s.gotDecidedBy = decidedBy
	s.gotSelector = selector
	if s.approveBatchFn != nil {
		return s.approveBatchFn(ctx, tenantID, decidedBy, selector)
	}
	return &ports.BatchDecisionResult{Errors: []string{}}, nil
}

func (s *stubPendingActionService) RejectBatch(
	ctx context.Context,
	tenantID, decidedBy uuid.UUID,
	selector domain.PendingActionSelector,
	reason string,
) (*ports.BatchDecisionResult, error) {
	s.batchRejCalled = true
	s.gotTenantID = tenantID
	s.gotDecidedBy = decidedBy
	s.gotSelector = selector
	s.gotReason = reason
	if s.rejectBatchFn != nil {
		return s.rejectBatchFn(ctx, tenantID, decidedBy, selector, reason)
	}
	return &ports.BatchDecisionResult{Errors: []string{}}, nil
}

func newPendingActionRouter(svc ports.PendingActionService) *chi.Mux {
	h := handlers.NewPendingActionHandler(svc)
	r := chi.NewRouter()
	r.Get("/pending-actions", h.ListPending)
	r.Post("/pending-actions/{id}/approve", h.Approve)
	r.Post("/pending-actions/{id}/reject", h.Reject)
	r.Post("/pending-actions/batch-approve", h.ApproveBatch)
	r.Post("/pending-actions/batch-reject", h.RejectBatch)
	return r
}

func decodeProblemResponse(t *testing.T, rec *httptest.ResponseRecorder) problem.Detail {
	t.Helper()
	assert.Equal(t, problem.ContentTypeProblemJSON, rec.Header().Get("Content-Type"))
	var prob problem.Detail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &prob))
	return prob
}

func TestPendingActionHandler_ListPending(t *testing.T) {
	t.Parallel()

	t.Run("200 JSON list", func(t *testing.T) {
		tenantID := uuid.New()
		act, err := domain.NewPendingAction(tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction, map[string]any{"description": "Padaria"})
		require.NoError(t, err)

		svc := &stubPendingActionService{
			listFn: func(_ context.Context, tid uuid.UUID) ([]*domain.PendingAction, error) {
				require.Equal(t, tenantID, tid)
				return []*domain.PendingAction{act}, nil
			},
		}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodGet, "/pending-actions", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		var got []domain.PendingAction
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Len(t, got, 1)
		assert.Equal(t, act.ID, got[0].ID)
	})

	t.Run("200 normalizes nil slice to empty JSON array", func(t *testing.T) {
		tenantID := uuid.New()
		svc := &stubPendingActionService{
			listFn: func(_ context.Context, _ uuid.UUID) ([]*domain.PendingAction, error) {
				return nil, nil
			},
		}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodGet, "/pending-actions", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "[]", strings.TrimSpace(rec.Body.String()))
	})

	t.Run("401 missing tenant context", func(t *testing.T) {
		r := newPendingActionRouter(&stubPendingActionService{})
		req := httptest.NewRequest(http.MethodGet, "/pending-actions", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		prob := decodeProblemResponse(t, rec)
		assert.Equal(t, problem.CodeUnauthorized, prob.Code)
	})

	t.Run("500 service error", func(t *testing.T) {
		tenantID := uuid.New()
		svc := &stubPendingActionService{
			listFn: func(_ context.Context, _ uuid.UUID) ([]*domain.PendingAction, error) {
				return nil, errors.New("db unavailable")
			},
		}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodGet, "/pending-actions", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		prob := decodeProblemResponse(t, rec)
		assert.Equal(t, problem.CodeInternalError, prob.Code)
	})
}

func TestPendingActionHandler_Approve(t *testing.T) {
	t.Parallel()

	t.Run("200 approved and executed with user ID", func(t *testing.T) {
		tenantID := uuid.New()
		userID := uuid.New()
		actionID := uuid.New()
		svc := &stubPendingActionService{}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+actionID.String()+"/approve", nil)
		ctx := middleware.WithTenantID(req.Context(), tenantID)
		ctx = middleware.WithUserID(ctx, userID)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, svc.approveCalled)
		assert.Equal(t, tenantID, svc.gotTenantID)
		assert.Equal(t, actionID, svc.gotActionID)
		assert.Equal(t, userID, svc.gotDecidedBy)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, actionID.String(), body["id"])
		assert.Equal(t, string(domain.ActionStatusExecuted), body["status"])
		assert.Equal(t, "Action approved and executed successfully.", body["message"])
	})

	t.Run("200 passes uuid.Nil when user ID absent from context", func(t *testing.T) {
		tenantID := uuid.New()
		actionID := uuid.New()
		svc := &stubPendingActionService{}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+actionID.String()+"/approve", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, uuid.Nil, svc.gotDecidedBy)
	})

	t.Run("401 missing tenant context", func(t *testing.T) {
		svc := &stubPendingActionService{}
		r := newPendingActionRouter(svc)
		req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+uuid.New().String()+"/approve", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		prob := decodeProblemResponse(t, rec)
		assert.Equal(t, problem.CodeUnauthorized, prob.Code)
		assert.False(t, svc.approveCalled)
	})

	t.Run("400 invalid UUID", func(t *testing.T) {
		tenantID := uuid.New()
		svc := &stubPendingActionService{}
		r := newPendingActionRouter(svc)

		for _, rawID := range []string{"not-a-uuid", uuid.Nil.String()} {
			req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+rawID+"/approve", nil)
			req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			prob := decodeProblemResponse(t, rec)
			assert.Equal(t, problem.CodeBadRequest, prob.Code)
			assert.False(t, svc.approveCalled)
		}
	})

	errorCases := []struct {
		name       string
		svcErr     error
		wantStatus int
		wantCode   string
		wantParam  string
	}{
		{
			name:       "404 ErrNotFound",
			svcErr:     domain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   problem.CodeNotFound,
		},
		{
			name:       "409 ErrPendingActionNotPending",
			svcErr:     domain.ErrPendingActionNotPending,
			wantStatus: http.StatusConflict,
			wantCode:   problem.CodeConflict,
		},
		{
			name:       "422 ErrInvalidPendingActionPayload",
			svcErr:     domain.ErrInvalidPendingActionPayload,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   problem.CodeValidationFailed,
			wantParam:  "payload",
		},
		{
			name:       "422 ErrUnsupportedPendingActionType",
			svcErr:     domain.ErrUnsupportedPendingActionType,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   problem.CodeValidationFailed,
			wantParam:  "action_type",
		},
		{
			name:       "500 internal error",
			svcErr:     errors.New("unexpected persistence failure"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   problem.CodeInternalError,
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			tenantID := uuid.New()
			actionID := uuid.New()
			svc := &stubPendingActionService{
				approveFn: func(_ context.Context, _, _, _ uuid.UUID) (*domain.PendingAction, error) {
					return nil, tc.svcErr
				},
			}
			r := newPendingActionRouter(svc)

			req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+actionID.String()+"/approve", nil)
			req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			prob := decodeProblemResponse(t, rec)
			assert.Equal(t, tc.wantStatus, prob.Status)
			assert.Equal(t, tc.wantCode, prob.Code)
			if tc.wantParam != "" {
				require.NotEmpty(t, prob.InvalidParams)
				assert.Equal(t, tc.wantParam, prob.InvalidParams[0].Name)
			}
		})
	}
}

func TestPendingActionHandler_Reject(t *testing.T) {
	t.Parallel()

	t.Run("200 rejected with reason", func(t *testing.T) {
		tenantID := uuid.New()
		userID := uuid.New()
		actionID := uuid.New()
		svc := &stubPendingActionService{}
		r := newPendingActionRouter(svc)

		body, err := json.Marshal(map[string]string{"reason": "Wrong proportions"})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+actionID.String()+"/reject", bytes.NewReader(body))
		ctx := middleware.WithTenantID(req.Context(), tenantID)
		ctx = middleware.WithUserID(ctx, userID)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, svc.rejectCalled)
		assert.Equal(t, tenantID, svc.gotTenantID)
		assert.Equal(t, actionID, svc.gotActionID)
		assert.Equal(t, userID, svc.gotDecidedBy)
		assert.Equal(t, "Wrong proportions", svc.gotReason)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, actionID.String(), resp["id"])
		assert.Equal(t, string(domain.ActionStatusRejected), resp["status"])
		assert.Equal(t, "Action rejected.", resp["message"])
	})

	t.Run("200 empty body allowed and passes uuid.Nil when user ID absent", func(t *testing.T) {
		tenantID := uuid.New()
		actionID := uuid.New()
		svc := &stubPendingActionService{}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+actionID.String()+"/reject", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, svc.rejectCalled)
		assert.Equal(t, uuid.Nil, svc.gotDecidedBy)
		assert.Equal(t, "", svc.gotReason)
	})

	t.Run("400 malformed JSON", func(t *testing.T) {
		tenantID := uuid.New()
		actionID := uuid.New()
		svc := &stubPendingActionService{}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+actionID.String()+"/reject", strings.NewReader("{invalid"))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		prob := decodeProblemResponse(t, rec)
		assert.Equal(t, problem.CodeBadRequest, prob.Code)
		assert.False(t, svc.rejectCalled)
	})

	t.Run("404 and 409 error mappings", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			err        error
			wantStatus int
			wantCode   string
		}{
			{name: "not found", err: domain.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: problem.CodeNotFound},
			{name: "not pending", err: domain.ErrPendingActionNotPending, wantStatus: http.StatusConflict, wantCode: problem.CodeConflict},
		} {
			tenantID := uuid.New()
			actionID := uuid.New()
			svc := &stubPendingActionService{
				rejectFn: func(_ context.Context, _, _, _ uuid.UUID, _ string) (*domain.PendingAction, error) {
					return nil, tc.err
				},
			}
			r := newPendingActionRouter(svc)

			req := httptest.NewRequest(http.MethodPost, "/pending-actions/"+actionID.String()+"/reject", nil)
			req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code, tc.name)
			prob := decodeProblemResponse(t, rec)
			assert.Equal(t, tc.wantCode, prob.Code)
		}
	})
}

func TestPendingActionHandler_ApproveBatchAndRejectBatch(t *testing.T) {
	t.Parallel()

	t.Run("ApproveBatch forwards All, ActionType, ActionIDs and returns partial failure result", func(t *testing.T) {
		tenantID := uuid.New()
		userID := uuid.New()
		id1 := uuid.New()
		id2 := uuid.New()
		catType := domain.ActionTypeCategorizeTransaction

		svc := &stubPendingActionService{
			approveBatchFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ domain.PendingActionSelector) (*ports.BatchDecisionResult, error) {
				return &ports.BatchDecisionResult{
					ProcessedCount: 2,
					ApprovedCount:  1,
					FailedCount:    1,
					Errors:         []string{"action " + id2.String() + ": resource not found"},
				}, nil
			},
		}
		r := newPendingActionRouter(svc)

		body, err := json.Marshal(dto.BatchApproveRequest{
			ActionIDs:  []uuid.UUID{id1, id2},
			All:        true,
			ActionType: &catType,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/batch-approve", bytes.NewReader(body))
		ctx := middleware.WithTenantID(req.Context(), tenantID)
		ctx = middleware.WithUserID(ctx, userID)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, svc.batchAppCalled)
		assert.Equal(t, tenantID, svc.gotTenantID)
		assert.Equal(t, userID, svc.gotDecidedBy)
		assert.True(t, svc.gotSelector.All)
		require.NotNil(t, svc.gotSelector.ActionType)
		assert.Equal(t, catType, *svc.gotSelector.ActionType)
		assert.Equal(t, []uuid.UUID{id1, id2}, svc.gotSelector.ActionIDs)

		var resp dto.BatchDecisionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, 2, resp.ProcessedCount)
		assert.Equal(t, 1, resp.ApprovedCount)
		assert.Equal(t, 0, resp.RejectedCount)
		assert.Equal(t, 1, resp.FailedCount)
		require.Len(t, resp.Errors, 1)
		assert.Contains(t, resp.Errors[0], id2.String())
	})

	t.Run("ApproveBatch malformed JSON returns 400", func(t *testing.T) {
		tenantID := uuid.New()
		svc := &stubPendingActionService{}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/batch-approve", strings.NewReader("{bad-json"))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		prob := decodeProblemResponse(t, rec)
		assert.Equal(t, problem.CodeBadRequest, prob.Code)
		assert.False(t, svc.batchAppCalled)
	})

	t.Run("ApproveBatch 500 service error", func(t *testing.T) {
		tenantID := uuid.New()
		svc := &stubPendingActionService{
			approveBatchFn: func(_ context.Context, _, _ uuid.UUID, _ domain.PendingActionSelector) (*ports.BatchDecisionResult, error) {
				return nil, errors.New("list failed")
			},
		}
		r := newPendingActionRouter(svc)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/batch-approve", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		prob := decodeProblemResponse(t, rec)
		assert.Equal(t, problem.CodeInternalError, prob.Code)
	})

	t.Run("RejectBatch forwards All, ActionIDs, Reason and returns BatchDecisionResponse", func(t *testing.T) {
		tenantID := uuid.New()
		id1 := uuid.New()
		svc := &stubPendingActionService{
			rejectBatchFn: func(_ context.Context, _, _ uuid.UUID, _ domain.PendingActionSelector, _ string) (*ports.BatchDecisionResult, error) {
				return &ports.BatchDecisionResult{
					ProcessedCount: 1,
					RejectedCount:  1,
					Errors:         []string{},
				}, nil
			},
		}
		r := newPendingActionRouter(svc)

		body, err := json.Marshal(dto.BatchRejectRequest{
			ActionIDs: []uuid.UUID{id1},
			All:       false,
			Reason:    "Batch rejected",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/pending-actions/batch-reject", bytes.NewReader(body))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, svc.batchRejCalled)
		assert.Equal(t, uuid.Nil, svc.gotDecidedBy)
		assert.False(t, svc.gotSelector.All)
		assert.Equal(t, []uuid.UUID{id1}, svc.gotSelector.ActionIDs)
		assert.Equal(t, "Batch rejected", svc.gotReason)

		var resp dto.BatchDecisionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, 1, resp.ProcessedCount)
		assert.Equal(t, 1, resp.RejectedCount)
		assert.Equal(t, 0, resp.FailedCount)
	})

	t.Run("RejectBatch malformed JSON returns 400 and service error returns 500", func(t *testing.T) {
		tenantID := uuid.New()
		svc := &stubPendingActionService{
			rejectBatchFn: func(_ context.Context, _, _ uuid.UUID, _ domain.PendingActionSelector, _ string) (*ports.BatchDecisionResult, error) {
				return nil, errors.New("unexpected failure")
			},
		}
		r := newPendingActionRouter(svc)

		badReq := httptest.NewRequest(http.MethodPost, "/pending-actions/batch-reject", strings.NewReader("{bad"))
		badReq = badReq.WithContext(middleware.WithTenantID(badReq.Context(), tenantID))
		badRec := httptest.NewRecorder()
		r.ServeHTTP(badRec, badReq)
		require.Equal(t, http.StatusBadRequest, badRec.Code)
		assert.False(t, svc.batchRejCalled)

		errReq := httptest.NewRequest(http.MethodPost, "/pending-actions/batch-reject", nil)
		errReq = errReq.WithContext(middleware.WithTenantID(errReq.Context(), tenantID))
		errRec := httptest.NewRecorder()
		r.ServeHTTP(errRec, errReq)
		require.Equal(t, http.StatusInternalServerError, errRec.Code)
		prob := decodeProblemResponse(t, errRec)
		assert.Equal(t, problem.CodeInternalError, prob.Code)
	})
}
