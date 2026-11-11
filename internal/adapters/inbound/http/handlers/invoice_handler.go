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
