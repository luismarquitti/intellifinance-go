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
	"github.com/shopspring/decimal"
)

// AccountHandler handles HTTP requests for financial accounts.
type AccountHandler struct {
	service ports.AccountService
}

// NewAccountHandler constructs a new AccountHandler injecting the AccountService port.
func NewAccountHandler(service ports.AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

// CreateAccount handles POST /accounts.
func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	ownerUserID, _ := middleware.UserIDFromContext(r.Context())
	if ownerUserID == uuid.Nil {
		ownerUserID = uuid.New() // Fallback user identity for MVP
	}

	var req dto.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Account validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	accType := domain.AccountType(strings.ToUpper(strings.TrimSpace(req.Type)))
	visibility := domain.Visibility(strings.ToUpper(strings.TrimSpace(req.Visibility)))

	acc, err := domain.NewAccount(tenantID, ownerUserID, req.Name, accType, req.Currency, visibility)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	if req.InstitutionID != "" {
		acc.InstitutionID = strings.TrimSpace(req.InstitutionID)
	}
	if req.InstitutionName != "" {
		acc.InstitutionName = strings.TrimSpace(req.InstitutionName)
	}
	if req.AccountNumberMasked != "" {
		acc.AccountNumberMasked = strings.TrimSpace(req.AccountNumberMasked)
	}
	if req.ColorHex != "" {
		acc.ColorHex = strings.TrimSpace(req.ColorHex)
	}
	if req.IconName != "" {
		acc.IconName = strings.TrimSpace(req.IconName)
	}
	if req.InitialBalance != "" {
		bal, _ := decimal.NewFromString(strings.TrimSpace(req.InitialBalance))
		acc.Balance = bal
	}

	if err := h.service.Create(r.Context(), acc); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dto.ToAccountResponse(acc))
}

// ListAccounts handles GET /accounts.
func (h *AccountHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	query := r.URL.Query()
	filter := ports.AccountFilter{}

	if activeStr := query.Get("is_active"); activeStr != "" {
		val, err := strconv.ParseBool(activeStr)
		if err != nil {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'is_active': must be true or false."))
			return
		}
		filter.IsActive = &val
	}

	if typeStr := query.Get("type"); typeStr != "" {
		accType := domain.AccountType(strings.ToUpper(strings.TrimSpace(typeStr)))
		if !accType.IsValid() {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'type'."))
			return
		}
		filter.Type = &accType
	}

	if visStr := query.Get("visibility"); visStr != "" {
		vis := domain.Visibility(strings.ToUpper(strings.TrimSpace(visStr)))
		if !vis.IsValid() {
			problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid query parameter 'visibility'."))
			return
		}
		filter.Visibility = &vis
	}

	accounts, err := h.service.List(r.Context(), tenantID, filter)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToAccountListResponse(accounts))
}

// GetAccount handles GET /accounts/{id}.
func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil || id == uuid.Nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid account ID: must be a valid UUID."))
		return
	}

	acc, err := h.service.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Account not found."))
			return
		}
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}
	if acc == nil {
		problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Account not found."))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToAccountResponse(acc))
}

// UpdateAccount handles PATCH /accounts/{id}.
func (h *AccountHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil || id == uuid.Nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid account ID: must be a valid UUID."))
		return
	}

	var req dto.UpdateAccountRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}


	if invalidParams := req.Validate(); len(invalidParams) > 0 {
		prob := problem.NewValidationFailed(r.URL.Path, "Account update validation failed.", invalidParams)
		problem.Render(w, r, prob)
		return
	}

	acc, err := h.service.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Account not found."))
			return
		}
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}
	if acc == nil {
		problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Account not found."))
		return
	}

	if req.Name != nil {
		acc.Name = strings.TrimSpace(*req.Name)
	}
	if req.Visibility != nil {
		acc.Visibility = domain.Visibility(strings.ToUpper(strings.TrimSpace(*req.Visibility)))
	}
	if req.ColorHex != nil {
		acc.ColorHex = strings.TrimSpace(*req.ColorHex)
	}
	if req.IconName != nil {
		acc.IconName = strings.TrimSpace(*req.IconName)
	}
	if req.IsActive != nil {
		acc.IsActive = *req.IsActive
	}
	if req.InstitutionName != nil {
		acc.InstitutionName = strings.TrimSpace(*req.InstitutionName)
	}
	if req.AccountNumberMasked != nil {
		acc.AccountNumberMasked = strings.TrimSpace(*req.AccountNumberMasked)
	}

	if err := acc.Validate(); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	if err := h.service.Update(r.Context(), acc); err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToAccountResponse(acc))
}
