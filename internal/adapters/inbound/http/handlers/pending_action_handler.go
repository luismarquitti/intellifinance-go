package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	repo            ports.PendingActionRepository
	txService       ports.TransactionService
	transferService ports.TransferService
	invoiceRepo     ports.InvoiceRepository
}

// NewPendingActionHandler constructs a new PendingActionHandler.
func NewPendingActionHandler(
	repo ports.PendingActionRepository,
	txService ports.TransactionService,
	transferService ports.TransferService,
	invoiceRepo ports.InvoiceRepository,
) *PendingActionHandler {
	return &PendingActionHandler{
		repo:            repo,
		txService:       txService,
		transferService: transferService,
		invoiceRepo:     invoiceRepo,
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

	if err := h.executeApproval(r.Context(), tenantID, action, decidingUserID); err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed executing approved action: "+err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      actionID,
		"status":  domain.ActionStatusExecuted,
		"message": "Action approved and executed successfully.",
	})
}

// ApproveBatch handles POST /pending-actions/batch-approve.
func (h *PendingActionHandler) ApproveBatch(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.BatchApproveRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	decidingUserID, _ := middleware.UserIDFromContext(r.Context())
	if decidingUserID == uuid.Nil {
		decidingUserID = uuid.New()
	}

	// 1. Identify actions to approve
	allPending, err := h.repo.ListPending(r.Context(), tenantID)
	if err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed listing pending actions: "+err.Error()))
		return
	}

	selectedMap := make(map[uuid.UUID]bool)
	for _, id := range req.ActionIDs {
		selectedMap[id] = true
	}

	var toApprove []*domain.PendingAction
	for _, act := range allPending {
		if req.All {
			if req.ActionType == nil || act.ActionType == *req.ActionType {
				toApprove = append(toApprove, act)
			}
		} else if selectedMap[act.ID] {
			toApprove = append(toApprove, act)
		}
	}

	resp := dto.BatchDecisionResponse{
		ProcessedCount: len(toApprove),
		Errors:         []string{},
	}

	for _, act := range toApprove {
		if err := h.executeApproval(r.Context(), tenantID, act, decidingUserID); err != nil {
			resp.FailedCount++
			resp.Errors = append(resp.Errors, fmt.Sprintf("action %s: %v", act.ID, err))
		} else {
			resp.ApprovedCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// RejectBatch handles POST /pending-actions/batch-reject.
func (h *PendingActionHandler) RejectBatch(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.BatchRejectRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	decidingUserID, _ := middleware.UserIDFromContext(r.Context())
	if decidingUserID == uuid.Nil {
		decidingUserID = uuid.New()
	}

	allPending, err := h.repo.ListPending(r.Context(), tenantID)
	if err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed listing pending actions: "+err.Error()))
		return
	}

	selectedMap := make(map[uuid.UUID]bool)
	for _, id := range req.ActionIDs {
		selectedMap[id] = true
	}

	var reasonPtr *string
	if req.Reason != "" {
		reasonPtr = &req.Reason
	}

	resp := dto.BatchDecisionResponse{
		Errors: []string{},
	}

	for _, act := range allPending {
		if req.All || selectedMap[act.ID] {
			resp.ProcessedCount++
			if err := h.repo.UpdateDecision(r.Context(), tenantID, act.ID, domain.ActionStatusRejected, &decidingUserID, reasonPtr); err != nil {
				resp.FailedCount++
				resp.Errors = append(resp.Errors, fmt.Sprintf("action %s: %v", act.ID, err))
			} else {
				resp.RejectedCount++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *PendingActionHandler) executeApproval(
	ctx context.Context,
	tenantID uuid.UUID,
	action *domain.PendingAction,
	decidingUserID uuid.UUID,
) error {
	// 1. Reconcile transfer proposal
	if action.ActionType == domain.ActionTypeReconcileTransfer && h.transferService != nil {
		outflowStr, _ := action.Payload["outflow_transaction_id"].(string)
		inflowStr, _ := action.Payload["inflow_transaction_id"].(string)
		outflowID, errOut := uuid.Parse(outflowStr)
		inflowID, errIn := uuid.Parse(inflowStr)
		if errOut != nil || errIn != nil {
			return errors.New("invalid transaction IDs in action payload")
		}

		if _, _, err := h.transferService.ConfirmTransfer(ctx, tenantID, outflowID, inflowID); err != nil {
			return fmt.Errorf("failed confirming reconciled transfer: %w", err)
		}

		// Mark invoice as paid if invoice_id is present
		if invIDStr, ok := action.Payload["invoice_id"].(string); ok && invIDStr != "" && h.invoiceRepo != nil {
			if invID, pErr := uuid.Parse(invIDStr); pErr == nil {
				if inv, getErr := h.invoiceRepo.FindByID(ctx, tenantID, invID); getErr == nil && inv != nil {
					inv.MarkAsPaid()
					_ = h.invoiceRepo.Save(ctx, inv)
				}
			}
		}
	}

	// 2. Categorize transaction proposal
	if action.ActionType == domain.ActionTypeCategorizeTransaction && h.txService != nil {
		if txIDStr, ok := action.Payload["transaction_id"].(string); ok {
			if txID, parseErr := uuid.Parse(txIDStr); parseErr == nil {
				var catID *uuid.UUID
				if catIDStr, ok := action.Payload["category_id"].(string); ok && catIDStr != "" {
					if cID, pErr := uuid.Parse(catIDStr); pErr == nil {
						catID = &cID
					}
				}
				tx, getErr := h.txService.GetByID(ctx, tenantID, txID)
				if getErr == nil && tx != nil {
					tx.CategoryID = catID
					tx.IsAICategorized = true
					tx.IsManuallyVerified = true
					if updateErr := h.txService.Update(ctx, tx); updateErr != nil {
						return fmt.Errorf("failed applying approved categorization: %w", updateErr)
					}
				}
			}
		}
	}

	if err := h.repo.UpdateDecision(ctx, tenantID, action.ID, domain.ActionStatusApproved, &decidingUserID, nil); err != nil {
		return fmt.Errorf("failed updating action approval decision: %w", err)
	}

	return h.repo.MarkExecuted(ctx, tenantID, action.ID)
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
