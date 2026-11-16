package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// InvoiceHandler exposes HTTP endpoints for credit card invoice management and cashflow projection.
type InvoiceHandler struct {
	service ports.InvoiceService
}

// NewInvoiceHandler constructs a new InvoiceHandler instance.
func NewInvoiceHandler(service ports.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{service: service}
}

// ListInvoices handles GET /invoices?account_id=...
func (h *InvoiceHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var accountIDPtr *uuid.UUID
	if accStr := r.URL.Query().Get("account_id"); accStr != "" {
		accID, err := uuid.Parse(accStr)
		if err != nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid account_id parameter."))
			return
		}
		accountIDPtr = &accID
	}

	invoices, err := h.service.ListInvoices(r.Context(), tenantID, accountIDPtr)
	if err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed querying invoices."))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if invoices == nil {
		invoices = []*domain.CreditCardInvoice{}
	}
	_ = json.NewEncoder(w).Encode(invoices)
}

// GetInvoice handles GET /invoices/{id}
func (h *InvoiceHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idStr := chi.URLParam(r, "id")
	invoiceID, err := uuid.Parse(idStr)
	if err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid invoice ID format."))
		return
	}

	inv, txs, err := h.service.GetInvoiceDetails(r.Context(), tenantID, invoiceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Invoice not found."))
			return
		}
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed retrieving invoice details."))
		return
	}

	if txs == nil {
		txs = []*domain.Transaction{}
	}

	response := struct {
		Invoice      *domain.CreditCardInvoice `json:"invoice"`
		Transactions []*domain.Transaction     `json:"transactions"`
	}{
		Invoice:      inv,
		Transactions: txs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// PayInvoice handles POST /invoices/{id}/pay
func (h *InvoiceHandler) PayInvoice(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idStr := chi.URLParam(r, "id")
	invoiceID, err := uuid.Parse(idStr)
	if err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid invoice ID format."))
		return
	}

	var req struct {
		AccountID string `json:"account_id"`
		PaidAt    string `json:"paid_at"`
	}
	if r.Body != nil {
		if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil && !errors.Is(decErr, io.EOF) {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload."))
			return
		}
	}

	var accountIDPtr *uuid.UUID
	if strings.TrimSpace(req.AccountID) != "" {
		accID, parseErr := uuid.Parse(strings.TrimSpace(req.AccountID))
		if parseErr != nil || accID == uuid.Nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid account_id parameter."))
			return
		}
		accountIDPtr = &accID
	}

	var paidAtPtr *time.Time
	if strings.TrimSpace(req.PaidAt) != "" {
		t, parseErr := dto.ParseDate(req.PaidAt)
		if parseErr != nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid paid_at date format."))
			return
		}
		paidAtPtr = &t
	}

	userID, _ := middleware.UserIDFromContext(r.Context())
	inv, err := h.service.PayInvoice(r.Context(), tenantID, userID, invoiceID, accountIDPtr, paidAtPtr)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Invoice or checking account not found."))
			return
		}
		if errors.Is(err, domain.ErrInvalidAccountType) {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Payment account cannot be a credit card."))
			return
		}
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed paying invoice: "+err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(inv)
}

// UnpayInvoice handles POST /invoices/{id}/unpay
func (h *InvoiceHandler) UnpayInvoice(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idStr := chi.URLParam(r, "id")
	invoiceID, err := uuid.Parse(idStr)
	if err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid invoice ID format."))
		return
	}

	inv, err := h.service.UnpayInvoice(r.Context(), tenantID, invoiceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Invoice not found."))
			return
		}
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed reopening invoice: "+err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(inv)
}

// GetCashflowProjection handles GET /cashflow/projection
func (h *InvoiceHandler) GetCashflowProjection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	projection, err := h.service.GetCashflowProjection(r.Context(), tenantID)
	if err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed computing cashflow projection."))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(projection)
}
