package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// PendingActionHandler manages Human-in-the-Loop approval workflows for AI/MCP mutations.
type PendingActionHandler struct {
	service ports.PendingActionService
}

// NewPendingActionHandler constructs a new PendingActionHandler.
func NewPendingActionHandler(service ports.PendingActionService) *PendingActionHandler {
	return &PendingActionHandler{service: service}
}

// ListPending handles GET /pending-actions.
func (h *PendingActionHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenantAndService(w, r)
	if !ok {
		return
	}

	actions, err := h.service.ListPending(r.Context(), tenantID)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	if actions == nil {
		actions = []*domain.PendingAction{}
	}
	renderJSON(w, http.StatusOK, actions)
}

// Approve handles POST /pending-actions/{id}/approve.
func (h *PendingActionHandler) Approve(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenantAndService(w, r)
	if !ok {
		return
	}

	actionID, ok := parseActionID(w, r)
	if !ok {
		return
	}

	decidingUserID, _ := middleware.UserIDFromContext(r.Context())
	executed, err := h.service.Approve(r.Context(), tenantID, actionID, decidingUserID)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	status := domain.ActionStatusExecuted
	if executed != nil {
		status = executed.Status
	}
	renderJSON(w, http.StatusOK, map[string]any{
		"id":      actionID,
		"status":  status,
		"message": "Action approved and executed successfully.",
	})
}

// Reject handles POST /pending-actions/{id}/reject.
func (h *PendingActionHandler) Reject(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenantAndService(w, r)
	if !ok {
		return
	}

	actionID, ok := parseActionID(w, r)
	if !ok {
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if decErr := decodeOptionalJSON(r, &req); decErr != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	decidingUserID, _ := middleware.UserIDFromContext(r.Context())
	rejected, err := h.service.Reject(r.Context(), tenantID, actionID, decidingUserID, req.Reason)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	status := domain.ActionStatusRejected
	if rejected != nil {
		status = rejected.Status
	}
	renderJSON(w, http.StatusOK, map[string]any{
		"id":      actionID,
		"status":  status,
		"message": "Action rejected.",
	})
}

// ApproveBatch handles POST /pending-actions/batch-approve.
func (h *PendingActionHandler) ApproveBatch(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenantAndService(w, r)
	if !ok {
		return
	}

	var req dto.BatchApproveRequest
	if decErr := decodeOptionalJSON(r, &req); decErr != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	decidingUserID, _ := middleware.UserIDFromContext(r.Context())
	selector := domain.PendingActionSelector{
		ActionIDs:  req.ActionIDs,
		All:        req.All,
		ActionType: req.ActionType,
	}

	result, err := h.service.ApproveBatch(r.Context(), tenantID, decidingUserID, selector)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	renderJSON(w, http.StatusOK, toBatchDecisionResponse(result))
}

// RejectBatch handles POST /pending-actions/batch-reject.
func (h *PendingActionHandler) RejectBatch(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenantAndService(w, r)
	if !ok {
		return
	}

	var req dto.BatchRejectRequest
	if decErr := decodeOptionalJSON(r, &req); decErr != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	decidingUserID, _ := middleware.UserIDFromContext(r.Context())
	selector := domain.PendingActionSelector{
		ActionIDs: req.ActionIDs,
		All:       req.All,
	}

	result, err := h.service.RejectBatch(r.Context(), tenantID, decidingUserID, selector, req.Reason)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	renderJSON(w, http.StatusOK, toBatchDecisionResponse(result))
}

func (h *PendingActionHandler) requireTenantAndService(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return uuid.Nil, false
	}
	if h == nil || h.service == nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Pending action service is not configured."))
		return uuid.Nil, false
	}
	return tenantID, true
}

func parseActionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	actionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || actionID == uuid.Nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid action ID format."))
		return uuid.Nil, false
	}
	return actionID, true
}

func decodeOptionalJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	err := json.NewDecoder(r.Body).Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func toBatchDecisionResponse(result *ports.BatchDecisionResult) dto.BatchDecisionResponse {
	if result == nil {
		return dto.BatchDecisionResponse{Errors: []string{}}
	}
	errs := result.Errors
	if errs == nil {
		errs = []string{}
	}
	return dto.BatchDecisionResponse{
		ProcessedCount: result.ProcessedCount,
		ApprovedCount:  result.ApprovedCount,
		RejectedCount:  result.RejectedCount,
		FailedCount:    result.FailedCount,
		Errors:         errs,
	}
}

func renderJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
