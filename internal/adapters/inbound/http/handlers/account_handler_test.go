package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

type mockAccountService struct {
	accounts map[uuid.UUID]*domain.Account
}

func newMockAccountService() *mockAccountService {
	return &mockAccountService{
		accounts: make(map[uuid.UUID]*domain.Account),
	}
}

func (m *mockAccountService) Create(ctx context.Context, account *domain.Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Account, error) {
	acc, exists := m.accounts[id]
	if !exists || acc.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return acc, nil
}

func (m *mockAccountService) List(ctx context.Context, tenantID uuid.UUID, filter ports.AccountFilter) ([]*domain.Account, error) {
	var res []*domain.Account
	for _, acc := range m.accounts {
		if acc.TenantID != tenantID {
			continue
		}
		if filter.IsActive != nil && acc.IsActive != *filter.IsActive {
			continue
		}
		if filter.Type != nil && acc.Type != *filter.Type {
			continue
		}
		if filter.Visibility != nil && acc.Visibility != *filter.Visibility {
			continue
		}
		res = append(res, acc)
	}
	return res, nil
}

func (m *mockAccountService) Update(ctx context.Context, account *domain.Account) error {
	if _, exists := m.accounts[account.ID]; !exists {
		return domain.ErrNotFound
	}
	m.accounts[account.ID] = account
	return nil
}

func TestAccountHandler_CreateAccount(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	service := newMockAccountService()
	handler := handlers.NewAccountHandler(service)

	t.Run("Success 201", func(t *testing.T) {
		payload := []byte(`{
			"name": "Nubank Roxinho",
			"type": "CREDIT_CARD",
			"currency": "BRL",
			"initial_balance": "150.00",
			"visibility": "SHARED"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rec := httptest.NewRecorder()
		handler.CreateAccount(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp dto.AccountResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Name != "Nubank Roxinho" {
			t.Errorf("expected name 'Nubank Roxinho', got '%s'", resp.Name)
		}
		if resp.Balance != "150.00" {
			t.Errorf("expected balance '150.00', got '%s'", resp.Balance)
		}
	})

	t.Run("Validation failure 422 - Empty name and invalid type", func(t *testing.T) {
		payload := []byte(`{
			"name": "",
			"type": "UNKNOWN_TYPE"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))

		rec := httptest.NewRecorder()
		handler.CreateAccount(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422, got %d: %s", rec.Code, rec.Body.String())
		}

		var prob problem.ProblemDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
			t.Fatalf("failed to decode problem detail: %v", err)
		}
		if prob.Code != problem.CodeValidationFailed {
			t.Errorf("expected code %s, got %s", problem.CodeValidationFailed, prob.Code)
		}
		if len(prob.InvalidParams) < 2 {
			t.Errorf("expected at least 2 invalid params, got %d", len(prob.InvalidParams))
		}
	})

	t.Run("Missing tenant 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewReader([]byte(`{"name":"Test"}`)))
		rec := httptest.NewRecorder()
		handler.CreateAccount(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("Malformed JSON 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewReader([]byte(`{not-json}`)))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()
		handler.CreateAccount(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestAccountHandler_GetAccount(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	service := newMockAccountService()
	handler := handlers.NewAccountHandler(service)

	acc, _ := domain.NewAccount(tenantID, uuid.New(), "Itaú", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	_ = service.Create(context.Background(), acc)

	r := chi.NewRouter()
	r.Get("/accounts/{id}", handler.GetAccount)

	t.Run("Success 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/accounts/"+acc.ID.String(), nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Not Found 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/accounts/"+uuid.New().String(), nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
		var prob problem.ProblemDetail
		_ = json.Unmarshal(rec.Body.Bytes(), &prob)
		if prob.Code != problem.CodeNotFound {
			t.Errorf("expected code %s, got %s", problem.CodeNotFound, prob.Code)
		}
	})

	t.Run("Invalid UUID 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/accounts/not-a-uuid", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestAccountHandler_ListAccounts(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	service := newMockAccountService()
	handler := handlers.NewAccountHandler(service)

	acc1, _ := domain.NewAccount(tenantID, uuid.New(), "Acc 1", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	acc2, _ := domain.NewAccount(tenantID, uuid.New(), "Acc 2", domain.AccountTypeCreditCard, "BRL", domain.VisibilityPrivate)
	_ = service.Create(context.Background(), acc1)
	_ = service.Create(context.Background(), acc2)

	t.Run("List all", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.ListAccounts(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp dto.AccountListResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if len(resp.Items) != 2 {
			t.Errorf("expected 2 accounts, got %d", len(resp.Items))
		}
	})

	t.Run("Filter by type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/accounts?type=CHECKING", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.ListAccounts(rec, req)

		var resp dto.AccountListResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if len(resp.Items) != 1 || resp.Items[0].Type != "CHECKING" {
			t.Errorf("expected 1 CHECKING account, got %v", resp.Items)
		}
	})

	t.Run("Invalid filter returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/accounts?is_active=not-a-bool", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.ListAccounts(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestAccountHandler_UpdateAccount(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	service := newMockAccountService()
	handler := handlers.NewAccountHandler(service)

	acc, _ := domain.NewAccount(tenantID, uuid.New(), "Old Name", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	_ = service.Create(context.Background(), acc)

	r := chi.NewRouter()
	r.Patch("/accounts/{id}", handler.UpdateAccount)

	t.Run("Success 200", func(t *testing.T) {
		payload := []byte(`{"name":"New Name","color_hex":"#123456"}`)
		req := httptest.NewRequest(http.MethodPatch, "/accounts/"+acc.ID.String(), bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp dto.AccountResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Name != "New Name" || resp.ColorHex != "#123456" {
			t.Errorf("expected updated fields, got %+v", resp)
		}
	})

	t.Run("Not found 404", func(t *testing.T) {
		payload := []byte(`{"name":"New Name"}`)
		req := httptest.NewRequest(http.MethodPatch, "/accounts/"+uuid.New().String(), bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}
