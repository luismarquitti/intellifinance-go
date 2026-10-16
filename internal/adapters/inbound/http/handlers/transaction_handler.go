package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

// TransactionHandler handles HTTP requests for financial transactions.
type TransactionHandler struct {
	service ports.TransactionService
}

// NewTransactionHandler constructs a new TransactionHandler injecting the TransactionService port.
func NewTransactionHandler(service ports.TransactionService) *TransactionHandler {
	return &TransactionHandler{service: service}
}

// CreateTransaction handles POST /transactions.
func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Transaction validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	accountID, _ := uuid.Parse(strings.TrimSpace(req.AccountID))

	var categoryID *uuid.UUID
	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) != "" {
		cid, _ := uuid.Parse(strings.TrimSpace(*req.CategoryID))
		categoryID = &cid
	}

	date, _ := dto.ParseDate(req.Date)
	amount, _ := decimal.NewFromString(strings.TrimSpace(req.Amount))
	txType := domain.TransactionType(strings.ToUpper(strings.TrimSpace(req.Type)))

	status := domain.TransactionStatusCompleted
	if req.Status != "" {
		status = domain.TransactionStatus(strings.ToUpper(strings.TrimSpace(req.Status)))
	}

	visibility := domain.VisibilityShared
	if req.Visibility != "" {
		visibility = domain.Visibility(strings.ToUpper(strings.TrimSpace(req.Visibility)))
	}

	var userIDPtr *uuid.UUID
	if userID, ok := middleware.UserIDFromContext(r.Context()); ok && userID != uuid.Nil {
		userIDPtr = &userID
	}

	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       accountID,
		CategoryID:      categoryID,
		CreatedByUserID: userIDPtr,
		Date:            date,
		Amount:          amount,
		Description:     req.Description,
		Type:            txType,
		Status:          status,
		Visibility:      visibility,
		Source:          domain.SourceTypeManual,
		Notes:           req.Notes,
		Metadata:        req.Metadata,
	})
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	if err := h.service.Create(r.Context(), tx); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dto.ToTransactionResponse(tx))
}

// ListTransactions handles GET /transactions.
func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	query := r.URL.Query()
	filter := ports.TransactionFilter{
		Page:  1,
		Limit: 50,
	}

	if pageStr := query.Get("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'page': must be an integer >= 1."))
			return
		}
		filter.Page = p
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 200 {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'limit': must be an integer between 1 and 200."))
			return
		}
		filter.Limit = l
	}

	if accStr := query.Get("account_id"); accStr != "" {
		id, err := uuid.Parse(accStr)
		if err != nil || id == uuid.Nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'account_id': must be a valid UUID."))
			return
		}
		filter.AccountID = &id
	}

	if catStr := query.Get("category_id"); catStr != "" {
		id, err := uuid.Parse(catStr)
		if err != nil || id == uuid.Nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'category_id': must be a valid UUID."))
			return
		}
		filter.CategoryID = &id
	}

	if fromStr := query.Get("from_date"); fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'from_date': must be formatted as YYYY-MM-DD."))
			return
		}
		filter.FromDate = &t
	}

	if toStr := query.Get("to_date"); toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'to_date': must be formatted as YYYY-MM-DD."))
			return
		}
		filter.ToDate = &t
	}

	if typeStr := query.Get("type"); typeStr != "" {
		txType := domain.TransactionType(strings.ToUpper(strings.TrimSpace(typeStr)))
		if !txType.IsValid() {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'type'."))
			return
		}
		filter.Type = &txType
	}

	if statusStr := query.Get("status"); statusStr != "" {
		st := domain.TransactionStatus(strings.ToUpper(strings.TrimSpace(statusStr)))
		if !st.IsValid() {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'status'."))
			return
		}
		filter.Status = &st
	}

	if visStr := query.Get("visibility"); visStr != "" {
		vis := domain.Visibility(strings.ToUpper(strings.TrimSpace(visStr)))
		if !vis.IsValid() {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'visibility'."))
			return
		}
		filter.Visibility = &vis
	}

	if searchStr := query.Get("search"); searchStr != "" {
		filter.Search = strings.TrimSpace(searchStr)
	}

	if aiReviewStr := query.Get("ai_review_required"); aiReviewStr != "" {
		val, err := strconv.ParseBool(aiReviewStr)
		if err != nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'ai_review_required': must be true or false."))
			return
		}
		filter.AIReviewRequired = &val
	}

	txs, total, err := h.service.List(r.Context(), tenantID, filter)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	pagination := dto.NewPaginationResponse(filter.Page, filter.Limit, total)
	resp := dto.ToTransactionListResponse(txs, pagination)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetTransaction handles GET /transactions/{id}.
func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil || id == uuid.Nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid transaction ID: must be a valid UUID."))
		return
	}

	tx, err := h.service.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Transaction not found."))
			return
		}
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}
	if tx == nil {
		problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Transaction not found."))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToTransactionResponse(tx))
}

// UpdateTransaction handles PATCH /transactions/{id}.
func (h *TransactionHandler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil || id == uuid.Nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid transaction ID: must be a valid UUID."))
		return
	}

	var req dto.UpdateTransactionRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}


	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Transaction update validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	tx, err := h.service.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Transaction not found."))
			return
		}
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}
	if tx == nil {
		problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Transaction not found."))
		return
	}

	if req.CategoryID != nil {
		trimmed := strings.TrimSpace(*req.CategoryID)
		if trimmed == "" {
			tx.CategoryID = nil
		} else {
			cid, _ := uuid.Parse(trimmed)
			tx.CategoryID = &cid
			tx.IsManuallyVerified = true
		}
	}

	if req.Description != nil {
		tx.Description = strings.TrimSpace(*req.Description)
		tx.CleanDescription = domain.CleanDescription(tx.Description)
	}

	if req.Visibility != nil {
		tx.Visibility = domain.Visibility(strings.ToUpper(strings.TrimSpace(*req.Visibility)))
	}

	if req.Notes != nil {
		tx.Notes = strings.TrimSpace(*req.Notes)
	}

	if req.Status != nil {
		tx.Status = domain.TransactionStatus(strings.ToUpper(strings.TrimSpace(*req.Status)))
	}

	if req.Metadata != nil {
		tx.Metadata = req.Metadata
	}

	tx.UpdatedAt = time.Now().UTC()

	if err := tx.Validate(); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	if err := h.service.Update(r.Context(), tx); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToTransactionResponse(tx))
}

// DeleteTransaction handles DELETE /transactions/{id}.
func (h *TransactionHandler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil || id == uuid.Nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid transaction ID: must be a valid UUID."))
		return
	}

	tx, err := h.service.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Transaction not found."))
			return
		}
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}
	if tx == nil {
		problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Transaction not found."))
		return
	}

	if err := h.service.Delete(r.Context(), tenantID, id); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
