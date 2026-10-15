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

type mockCategoryService struct {
	categories map[uuid.UUID]*domain.Category
}

func newMockCategoryService() *mockCategoryService {
	return &mockCategoryService{
		categories: make(map[uuid.UUID]*domain.Category),
	}
}

func (m *mockCategoryService) Create(ctx context.Context, category *domain.Category) error {
	m.categories[category.ID] = category
	return nil
}

func (m *mockCategoryService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Category, error) {
	cat, exists := m.categories[id]
	if !exists || cat.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return cat, nil
}

func (m *mockCategoryService) List(ctx context.Context, tenantID uuid.UUID, filter ports.CategoryFilter) ([]*domain.Category, error) {
	var res []*domain.Category
	for _, c := range m.categories {
		if c.TenantID != tenantID {
			continue
		}
		if filter.Type != nil && c.Type != *filter.Type {
			continue
		}
		if filter.IncludeSystem != nil && !*filter.IncludeSystem && c.IsSystem {
			continue
		}
		if filter.ParentID != nil {
			if c.ParentID == nil || *c.ParentID != *filter.ParentID {
				continue
			}
		}
		res = append(res, c)
	}
	return res, nil
}

func (m *mockCategoryService) Update(ctx context.Context, category *domain.Category) error {
	if _, exists := m.categories[category.ID]; !exists {
		return domain.ErrNotFound
	}
	m.categories[category.ID] = category
	return nil
}

func TestCategoryHandler_CreateCategory(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	service := newMockCategoryService()
	handler := handlers.NewCategoryHandler(service)

	t.Run("Success 201", func(t *testing.T) {
		payload := []byte(`{
			"name": "Alimentação & Mercado",
			"type": "EXPENSE",
			"color_hex": "#10B981",
			"icon_name": "shopping-cart"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/categories", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.CreateCategory(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp dto.CategoryResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Name != "Alimentação & Mercado" || resp.Type != "EXPENSE" {
			t.Errorf("expected created category, got %+v", resp)
		}
	})

	t.Run("Validation failure 422", func(t *testing.T) {
		payload := []byte(`{"name":"","type":"INVALID"}`)
		req := httptest.NewRequest(http.MethodPost, "/categories", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.CreateCategory(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422, got %d", rec.Code)
		}

		var prob problem.ProblemDetail
		_ = json.Unmarshal(rec.Body.Bytes(), &prob)
		if prob.Code != problem.CodeValidationFailed {
			t.Errorf("expected code %s, got %s", problem.CodeValidationFailed, prob.Code)
		}
	})
}

func TestCategoryHandler_ListCategories(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	service := newMockCategoryService()
	handler := handlers.NewCategoryHandler(service)

	parent, _ := domain.NewCategory(tenantID, nil, "Parent", domain.TransactionTypeExpense, true)
	sub, _ := domain.NewCategory(tenantID, &parent.ID, "Child", domain.TransactionTypeExpense, false)
	_ = service.Create(context.Background(), parent)
	_ = service.Create(context.Background(), sub)

	t.Run("Hierarchical list 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/categories", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.ListCategories(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp dto.CategoryListResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if len(resp.Items) != 1 {
			t.Fatalf("expected 1 root category, got %d", len(resp.Items))
		}
		if len(resp.Items[0].Subcategories) != 1 {
			t.Fatalf("expected 1 subcategory under parent, got %d", len(resp.Items[0].Subcategories))
		}
		if resp.Items[0].Subcategories[0].Name != "Child" {
			t.Errorf("expected subcategory 'Child', got '%s'", resp.Items[0].Subcategories[0].Name)
		}
	})
}

func TestCategoryHandler_UpdateCategory(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	service := newMockCategoryService()
	handler := handlers.NewCategoryHandler(service)

	cat, _ := domain.NewCategory(tenantID, nil, "Old Cat", domain.TransactionTypeExpense, false)
	_ = service.Create(context.Background(), cat)

	r := chi.NewRouter()
	r.Patch("/categories/{id}", handler.UpdateCategory)

	t.Run("Success 200", func(t *testing.T) {
		payload := []byte(`{"name":"New Cat","color_hex":"#FFFFFF"}`)
		req := httptest.NewRequest(http.MethodPatch, "/categories/"+cat.ID.String(), bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp dto.CategoryResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Name != "New Cat" || resp.ColorHex != "#FFFFFF" {
			t.Errorf("expected updated category, got %+v", resp)
		}
	})

	t.Run("Not Found 404", func(t *testing.T) {
		payload := []byte(`{"name":"New Cat"}`)
		req := httptest.NewRequest(http.MethodPatch, "/categories/"+uuid.New().String(), bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}
