package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// PendingActionHandler manages Human-in-the-Loop approval workflows for AI/MCP mutations.
type PendingActionHandler struct {
	repo      ports.PendingActionRepository
	txService ports.TransactionService
}

// NewPendingActionHandler constructs a new PendingActionHandler.
func NewPendingActionHandler(repo ports.PendingActionRepository, txService ports.TransactionService) *PendingActionHandler {
	return &PendingActionHandler{
		repo:      repo,
		txService: txService,
	}
}

// ListPending handles GET /pending-actions.
func (h *PendingActionHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	actions, err := h.repo.ListPending(r.Context(), tenantID)
	if err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed to query pending actions."))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if actions == nil {
		actions = []*domain.PendingAction{}
	}
	_ = json.NewEncoder(w).Encode(actions)
}

// Approve handles POST /pending-actions/{id}/approve.
func (h *PendingActionHandler) Approve(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idStr := chi.URLParam(r, "id")
	actionID, err := uuid.Parse(idStr)
	if err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid action ID format."))
		return
	}

	decidingUserID, _ := middleware.UserIDFromContext(r.Context())
	if decidingUserID == uuid.Nil {
		decidingUserID = uuid.New()
	}

	action, err := h.repo.FindByID(r.Context(), tenantID, actionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Pending action not found."))
			return
		}
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed to load pending action."))
		return
	}

	if action.Status != domain.ActionStatusPending {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Action is not in pending status."))
		return
	}

	// Execute payload mutation if it's a categorization proposal
	if action.ActionType == domain.ActionTypeCategorizeTransaction && h.txService != nil {
		if txIDStr, ok := action.Payload["transaction_id"].(string); ok {
			if txID, parseErr := uuid.Parse(txIDStr); parseErr == nil {
				var catID *uuid.UUID
				if catIDStr, ok := action.Payload["category_id"].(string); ok && catIDStr != "" {
					if cID, pErr := uuid.Parse(catIDStr); pErr == nil {
						catID = &cID
					}
				}
				tx, getErr := h.txService.GetByID(r.Context(), tenantID, txID)
				if getErr == nil && tx != nil {
					tx.CategoryID = catID
					tx.IsAICategorized = true
					tx.IsManuallyVerified = true
					if updateErr := h.txService.Update(r.Context(), tx); updateErr != nil {
						problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed applying approved categorization: "+updateErr.Error()))
						return
					}
				}
			}
		}
	}

	if err := h.repo.UpdateDecision(r.Context(), tenantID, actionID, domain.ActionStatusApproved, &decidingUserID, nil); err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed updating action approval."))
		return
	}

	_ = h.repo.MarkExecuted(r.Context(), tenantID, actionID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      actionID,
		"status":  domain.ActionStatusExecuted,
		"message": "Action approved and executed successfully.",
	})
}

// Reject handles POST /pending-actions/{id}/reject.
func (h *PendingActionHandler) Reject(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idStr := chi.URLParam(r, "id")
	actionID, err := uuid.Parse(idStr)
	if err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid action ID format."))
		return
	}

	decidingUserID, _ := middleware.UserIDFromContext(r.Context())
	if decidingUserID == uuid.Nil {
		decidingUserID = uuid.New()
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var reasonPtr *string
	if req.Reason != "" {
		reasonPtr = &req.Reason
	}

	if err := h.repo.UpdateDecision(r.Context(), tenantID, actionID, domain.ActionStatusRejected, &decidingUserID, reasonPtr); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Pending action not found or already decided."))
			return
		}
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed updating action rejection."))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      actionID,
		"status":  domain.ActionStatusRejected,
		"message": "Action rejected.",
	})
}
