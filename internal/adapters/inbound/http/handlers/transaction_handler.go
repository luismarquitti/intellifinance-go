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

// parseTransactionFilter extracts filtering and pagination parameters from the request query string.
func parseTransactionFilter(r *http.Request) (ports.TransactionFilter, *problem.Detail) {
	query := r.URL.Query()
	filter := ports.TransactionFilter{
		Page:      1,
		Limit:     50,
		SortBy:    "date",
		SortOrder: "desc",
	}

	if pageStr := query.Get("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'page': must be an integer >= 1.")
			return filter, &prob
		}
		filter.Page = p
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 200 {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'limit': must be an integer between 1 and 200.")
			return filter, &prob
		}
		filter.Limit = l
	}

	if accStr := query.Get("account_id"); accStr != "" {
		id, err := uuid.Parse(accStr)
		if err != nil || id == uuid.Nil {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'account_id': must be a valid UUID.")
			return filter, &prob
		}
		filter.AccountID = &id
	}

	if catStr := query.Get("category_id"); catStr != "" {
		id, err := uuid.Parse(catStr)
		if err != nil || id == uuid.Nil {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'category_id': must be a valid UUID.")
			return filter, &prob
		}
		filter.CategoryID = &id
	}

	if uncatStr := query.Get("uncategorized"); uncatStr != "" {
		val, err := strconv.ParseBool(uncatStr)
		if err != nil {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'uncategorized': must be true or false.")
			return filter, &prob
		}
		filter.Uncategorized = &val
	}

	if fromStr := query.Get("from_date"); fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'from_date': must be formatted as YYYY-MM-DD.")
			return filter, &prob
		}
		filter.FromDate = &t
	}

	if toStr := query.Get("to_date"); toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'to_date': must be formatted as YYYY-MM-DD.")
			return filter, &prob
		}
		filter.ToDate = &t
	}

	if typeStr := query.Get("type"); typeStr != "" {
		txType := domain.TransactionType(strings.ToUpper(strings.TrimSpace(typeStr)))
		if !txType.IsValid() {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'type'.")
			return filter, &prob
		}
		filter.Type = &txType
	}

	if statusStr := query.Get("status"); statusStr != "" {
		st := domain.TransactionStatus(strings.ToUpper(strings.TrimSpace(statusStr)))
		if !st.IsValid() {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'status'.")
			return filter, &prob
		}
		filter.Status = &st
	}

	if visStr := query.Get("visibility"); visStr != "" {
		vis := domain.Visibility(strings.ToUpper(strings.TrimSpace(visStr)))
		if !vis.IsValid() {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'visibility'.")
			return filter, &prob
		}
		filter.Visibility = &vis
	}

	if searchStr := query.Get("search"); searchStr != "" {
		filter.Search = strings.TrimSpace(searchStr)
	}

	if sortBy := query.Get("sort_by"); sortBy != "" {
		filter.SortBy = strings.TrimSpace(sortBy)
	}

	if sortOrder := query.Get("sort_order"); sortOrder != "" {
		filter.SortOrder = strings.TrimSpace(sortOrder)
	}

	if aiReviewStr := query.Get("ai_review_required"); aiReviewStr != "" {
		val, err := strconv.ParseBool(aiReviewStr)
		if err != nil {
			prob := problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'ai_review_required': must be true or false.")
			return filter, &prob
		}
		filter.AIReviewRequired = &val
	}

	return filter, nil
}

// ListTransactions handles GET /transactions.
func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	filter, prob := parseTransactionFilter(r)
	if prob != nil {
		problem.Render(w, r, *prob)
		return
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

// SummarizeTransactions handles GET /transactions/summary.
func (h *TransactionHandler) SummarizeTransactions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	filter, prob := parseTransactionFilter(r)
	if prob != nil {
		problem.Render(w, r, *prob)
		return
	}

	summary, err := h.service.Summarize(r.Context(), tenantID, filter)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	resp := dto.TransactionSummaryResponse{
		TotalIncome:  summary.TotalIncome.StringFixed(2),
		TotalExpense: summary.TotalExpense.StringFixed(2),
		NetBalance:   summary.NetBalance.StringFixed(2),
		Count:        summary.Count,
	}

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

// UpdateTransaction handles PATCH and PUT /transactions/{id}.
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

	if req.AccountID != nil && strings.TrimSpace(*req.AccountID) != "" {
		aid, _ := uuid.Parse(strings.TrimSpace(*req.AccountID))
		tx.AccountID = aid
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

	if req.Date != nil && strings.TrimSpace(*req.Date) != "" {
		parsedDate, _ := dto.ParseDate(*req.Date)
		tx.Date = parsedDate
	}

	if req.Type != nil && strings.TrimSpace(*req.Type) != "" {
		tx.Type = domain.TransactionType(strings.ToUpper(strings.TrimSpace(*req.Type)))
	}

	if req.Amount != nil && strings.TrimSpace(*req.Amount) != "" {
		amt, _ := decimal.NewFromString(strings.TrimSpace(*req.Amount))
		tx.Amount = amt
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

	// Recompute fingerprint
	tx.Fingerprint = domain.GenerateTransactionFingerprint(
		tx.TenantID,
		tx.AccountID,
		tx.Date,
		tx.Amount,
		tx.CleanDescription,
		tx.ExternalID,
		tx.SequenceIndex,
	)

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

// BatchDeleteTransactions handles POST /transactions/batch-delete.
func (h *TransactionHandler) BatchDeleteTransactions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.BatchDeleteTransactionsRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		problem.Render(w, r, problem.NewValidationFailed(r.URL.Path, "Batch delete validation failed.", invalidParams))
		return
	}

	ids := make([]uuid.UUID, len(req.IDs))
	for i, idStr := range req.IDs {
		ids[i], _ = uuid.Parse(strings.TrimSpace(idStr))
	}

	if err := h.service.BatchDelete(r.Context(), tenantID, ids); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// BatchUpdateCategory handles POST /transactions/batch-category.
func (h *TransactionHandler) BatchUpdateCategory(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.BatchUpdateCategoryRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		problem.Render(w, r, problem.NewValidationFailed(r.URL.Path, "Batch update category validation failed.", invalidParams))
		return
	}

	ids := make([]uuid.UUID, len(req.IDs))
	for i, idStr := range req.IDs {
		ids[i], _ = uuid.Parse(strings.TrimSpace(idStr))
	}

	var catIDPtr *uuid.UUID
	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) != "" {
		cid, _ := uuid.Parse(strings.TrimSpace(*req.CategoryID))
		catIDPtr = &cid
	}

	if err := h.service.BatchUpdateCategory(r.Context(), tenantID, ids, catIDPtr); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"updated": len(ids)})
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
