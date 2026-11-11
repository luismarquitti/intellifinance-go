package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

// TransferHandler handles HTTP requests for confirmed internal transfers.
type TransferHandler struct {
	service          ports.TransferService
	reconcileService ports.TransferReconciliationService
}

// NewTransferHandler constructs a new TransferHandler.
func NewTransferHandler(service ports.TransferService, reconcileService ports.TransferReconciliationService) *TransferHandler {
	return &TransferHandler{
		service:          service,
		reconcileService: reconcileService,
	}
}

// ReconcileScan handles POST /transfers/reconcile-scan.
func (h *TransferHandler) ReconcileScan(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	if h.reconcileService == nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Reconciliation service not configured."))
		return
	}

	result, err := h.reconcileService.ScanAndPropose(r.Context(), tenantID)
	if err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed executing transfer reconciliation scan: "+err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// ConfirmTransfer handles POST /transfers/confirm.
func (h *TransferHandler) ConfirmTransfer(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.ConfirmTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Transfer confirmation validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	sourceID, _ := uuid.Parse(strings.TrimSpace(req.SourceTransactionID))
	destID, _ := uuid.Parse(strings.TrimSpace(req.DestinationTransactionID))

	legA, legB, err := h.service.ConfirmTransfer(r.Context(), tenantID, sourceID, destID)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToTransferResponse(legA, legB))
}

// CreateTransfer handles POST /transfers.
func (h *TransferHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.CreateTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Transfer creation validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	srcAccountID, _ := uuid.Parse(strings.TrimSpace(req.SourceAccountID))
	dstAccountID, _ := uuid.Parse(strings.TrimSpace(req.DestinationAccountID))
	amount, _ := decimal.NewFromString(strings.TrimSpace(req.Amount))
	date, _ := dto.ParseDate(req.Date)

	var userIDPtr *uuid.UUID
	if userID, ok := middleware.UserIDFromContext(r.Context()); ok && userID != uuid.Nil {
		userIDPtr = &userID
	}

	outflow, inflow, err := h.service.CreateTransfer(r.Context(), ports.CreateTransferParams{
		TenantID:             tenantID,
		IdempotencyKey:       req.IdempotencyKey,
		SourceAccountID:      srcAccountID,
		DestinationAccountID: dstAccountID,
		Amount:               amount,
		Date:                 date,
		Description:          strings.TrimSpace(req.Description),
		CreatedByUserID:      userIDPtr,
	})
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dto.ToTransferResponse(outflow, inflow))
}

// UnlinkTransfer handles POST /transfers/unlink.
func (h *TransferHandler) UnlinkTransfer(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.UnlinkTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Transfer unlink validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	txID, _ := uuid.Parse(strings.TrimSpace(req.TransactionID))

	legA, legB, err := h.service.UnlinkTransfer(r.Context(), tenantID, txID)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToTransferResponse(legA, legB))
}
