package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// CategoryHandler handles HTTP requests for budget categories.
type CategoryHandler struct {
	service ports.CategoryService
}

// NewCategoryHandler constructs a new CategoryHandler injecting the CategoryService port.
func NewCategoryHandler(service ports.CategoryService) *CategoryHandler {
	return &CategoryHandler{service: service}
}

// ListCategories handles GET /categories.
func (h *CategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	query := r.URL.Query()
	filter := ports.CategoryFilter{}

	if typeStr := query.Get("type"); typeStr != "" {
		txType := domain.TransactionType(strings.ToUpper(strings.TrimSpace(typeStr)))
		if !txType.IsValid() {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'type': must be INCOME, EXPENSE, or TRANSFER."))
			return
		}
		filter.Type = &txType
	}

	if includeSysStr := query.Get("include_system"); includeSysStr != "" {
		val, err := strconv.ParseBool(includeSysStr)
		if err != nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'include_system': must be true or false."))
			return
		}
		filter.IncludeSystem = &val
	}

	if parentStr := query.Get("parent_id"); parentStr != "" {
		parentID, err := uuid.Parse(parentStr)
		if err != nil || parentID == uuid.Nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'parent_id': must be a valid non-nil UUID."))
			return
		}
		filter.ParentID = &parentID
	}

	categories, err := h.service.List(r.Context(), tenantID, filter)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToCategoryHierarchy(categories))
}

// CreateCategory handles POST /categories.
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	var req dto.CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Category validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	var parentID *uuid.UUID
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		pid, _ := uuid.Parse(strings.TrimSpace(*req.ParentID))
		parentID = &pid
	}

	txType := domain.TransactionType(strings.ToUpper(strings.TrimSpace(req.Type)))
	isSystem := false
	if req.IsSystem != nil {
		isSystem = *req.IsSystem
	}

	cat, err := domain.NewCategory(tenantID, parentID, req.Name, txType, isSystem)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	if req.ColorHex != "" {
		cat.ColorHex = strings.TrimSpace(req.ColorHex)
	}
	if req.IconName != "" {
		cat.IconName = strings.TrimSpace(req.IconName)
	}

	if err := h.service.Create(r.Context(), cat); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dto.ToCategoryResponse(cat))
}

// UpdateCategory handles PATCH /categories/{id}.
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil || id == uuid.Nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid category ID: must be a valid UUID."))
		return
	}

	var req dto.UpdateCategoryRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}


	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Category update validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	cat, err := h.service.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Category not found."))
			return
		}
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}
	if cat == nil {
		problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Category not found."))
		return
	}

	if req.Name != nil {
		cat.Name = strings.TrimSpace(*req.Name)
	}
	if req.ParentID != nil {
		trimmed := strings.TrimSpace(*req.ParentID)
		if trimmed == "" {
			cat.ParentID = nil
		} else {
			pid, _ := uuid.Parse(trimmed)
			cat.ParentID = &pid
		}
	}
	if req.ColorHex != nil {
		cat.ColorHex = strings.TrimSpace(*req.ColorHex)
	}
	if req.IconName != nil {
		cat.IconName = strings.TrimSpace(*req.IconName)
	}

	if err := cat.Validate(); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	if err := h.service.Update(r.Context(), cat); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToCategoryResponse(cat))
}
