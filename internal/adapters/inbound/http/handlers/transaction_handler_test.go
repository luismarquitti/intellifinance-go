package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

type mockTransactionService struct {
	txs map[uuid.UUID]*domain.Transaction
}

func newMockTransactionService() *mockTransactionService {
	return &mockTransactionService{
		txs: make(map[uuid.UUID]*domain.Transaction),
	}
}

func (m *mockTransactionService) Create(ctx context.Context, tx *domain.Transaction) error {
	m.txs[tx.ID] = tx
	return nil
}

func (m *mockTransactionService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error) {
	tx, exists := m.txs[id]
	if !exists || tx.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return tx, nil
}

func (m *mockTransactionService) List(ctx context.Context, tenantID uuid.UUID, filter ports.TransactionFilter) ([]*domain.Transaction, int64, error) {
	var res []*domain.Transaction
	for _, tx := range m.txs {
		if tx.TenantID != tenantID {
			continue
		}
		if filter.AccountID != nil && tx.AccountID != *filter.AccountID {
			continue
		}
		if filter.CategoryID != nil {
			if tx.CategoryID == nil || *tx.CategoryID != *filter.CategoryID {
				continue
			}
		}
		if filter.Type != nil && tx.Type != *filter.Type {
			continue
		}
		if filter.Status != nil && tx.Status != *filter.Status {
			continue
		}
		if filter.Visibility != nil && tx.Visibility != *filter.Visibility {
			continue
		}
		res = append(res, tx)
	}

	total := int64(len(res))
	start := (filter.Page - 1) * filter.Limit
	if start > len(res) {
		return []*domain.Transaction{}, total, nil
	}
	end := start + filter.Limit
	if end > len(res) {
		end = len(res)
	}

	return res[start:end], total, nil
}

func (m *mockTransactionService) Update(ctx context.Context, tx *domain.Transaction) error {
	if _, exists := m.txs[tx.ID]; !exists {
		return domain.ErrNotFound
	}
	m.txs[tx.ID] = tx
	return nil
}

func (m *mockTransactionService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	tx, exists := m.txs[id]
	if !exists || tx.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(m.txs, id)
	return nil
}

func TestTransactionHandler_CreateTransaction(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	accountID := uuid.New()
	service := newMockTransactionService()
	handler := handlers.NewTransactionHandler(service)

	t.Run("Success 201 Expense with negative amount", func(t *testing.T) {
		payload := []byte(`{
			"account_id": "` + accountID.String() + `",
			"date": "2026-09-24",
			"amount": "-45.00",
			"description": "Padaria Santo Antônio",
			"type": "EXPENSE",
			"visibility": "SHARED",
			"notes": "Café da manhã"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.CreateTransaction(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp dto.TransactionResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Amount != "-45.00" || resp.Type != "EXPENSE" {
			t.Errorf("expected created expense transaction, got %+v", resp)
		}
		if resp.CleanDescription == "" {
			t.Error("expected non-empty clean description")
		}
	})

	t.Run("Validation failure 422 - Expense with positive amount", func(t *testing.T) {
		payload := []byte(`{
			"account_id": "` + accountID.String() + `",
			"date": "2026-09-24",
			"amount": "45.00",
			"description": "Padaria Santo Antônio",
			"type": "EXPENSE"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.CreateTransaction(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422, got %d: %s", rec.Code, rec.Body.String())
		}

		var prob problem.ProblemDetail
		_ = json.Unmarshal(rec.Body.Bytes(), &prob)
		if prob.Code != problem.CodeValidationFailed {
			t.Errorf("expected code %s, got %s", problem.CodeValidationFailed, prob.Code)
		}
	})

	t.Run("Validation failure 422 - Zero amount", func(t *testing.T) {
		payload := []byte(`{
			"account_id": "` + accountID.String() + `",
			"date": "2026-09-24",
			"amount": "0.00",
			"description": "Padaria",
			"type": "EXPENSE"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.CreateTransaction(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422, got %d", rec.Code)
		}
	})

	t.Run("Validation failure 422 - Missing account_id", func(t *testing.T) {
		payload := []byte(`{
			"date": "2026-09-24",
			"amount": "-45.00",
			"description": "Padaria",
			"type": "EXPENSE"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.CreateTransaction(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422, got %d", rec.Code)
		}
	})
}

func TestTransactionHandler_GetTransaction(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	accountID := uuid.New()
	service := newMockTransactionService()
	handler := handlers.NewTransactionHandler(service)

	tx, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   accountID,
		Date:        time.Now(),
		Amount:      decimal.NewFromInt(-100),
		Description: "Mercado",
		Type:        domain.TransactionTypeExpense,
	})
	_ = service.Create(context.Background(), tx)

	r := chi.NewRouter()
	r.Get("/transactions/{id}", handler.GetTransaction)

	t.Run("Success 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/transactions/"+tx.ID.String(), nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("Not Found 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/transactions/"+uuid.New().String(), nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}

func TestTransactionHandler_ListTransactions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	accountID := uuid.New()
	service := newMockTransactionService()
	handler := handlers.NewTransactionHandler(service)

	for i := 1; i <= 5; i++ {
		tx, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:      tenantID,
			AccountID:     accountID,
			Date:          time.Now(),
			Amount:        decimal.NewFromInt(int64(-i * 10)),
			Description:   "Store item",
			Type:          domain.TransactionTypeExpense,
			SequenceIndex: i,
		})
		_ = service.Create(context.Background(), tx)
	}

	t.Run("Success 200 with pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/transactions?page=1&limit=2", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.ListTransactions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp dto.TransactionListResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if len(resp.Items) != 2 {
			t.Errorf("expected 2 items, got %d", len(resp.Items))
		}
		if resp.Pagination.Total != 5 {
			t.Errorf("expected total 5, got %d", resp.Pagination.Total)
		}
		if resp.Pagination.TotalPages != 3 {
			t.Errorf("expected 3 total pages, got %d", resp.Pagination.TotalPages)
		}
		if !resp.Pagination.HasNext {
			t.Error("expected has_next to be true")
		}
		if resp.Pagination.HasPrev {
			t.Error("expected has_prev to be false for page 1")
		}
	})

	t.Run("Invalid limit returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/transactions?limit=500", nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		handler.ListTransactions(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestTransactionHandler_UpdateTransaction(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	service := newMockTransactionService()
	handler := handlers.NewTransactionHandler(service)

	tx, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   accountID,
		Date:        time.Now(),
		Amount:      decimal.NewFromInt(-100),
		Description: "Old Desc",
		Type:        domain.TransactionTypeExpense,
	})
	_ = service.Create(context.Background(), tx)

	r := chi.NewRouter()
	r.Patch("/transactions/{id}", handler.UpdateTransaction)

	t.Run("Success 200", func(t *testing.T) {
		payload := []byte(`{
			"category_id": "` + categoryID.String() + `",
			"description": "New Desc",
			"notes": "Reviewed"
		}`)

		req := httptest.NewRequest(http.MethodPatch, "/transactions/"+tx.ID.String(), bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp dto.TransactionResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if resp.Description != "New Desc" || resp.Notes != "Reviewed" {
			t.Errorf("expected updated fields, got %+v", resp)
		}
		if !resp.IsManuallyVerified {
			t.Error("expected IsManuallyVerified to be true after category update")
		}
	})

	t.Run("Not Found 404", func(t *testing.T) {
		payload := []byte(`{"description":"New"}`)
		req := httptest.NewRequest(http.MethodPatch, "/transactions/"+uuid.New().String(), bytes.NewReader(payload))
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}

func TestTransactionHandler_DeleteTransaction(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	accountID := uuid.New()
	service := newMockTransactionService()
	handler := handlers.NewTransactionHandler(service)

	tx, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   accountID,
		Date:        time.Now(),
		Amount:      decimal.NewFromInt(-50),
		Description: "Coffee",
		Type:        domain.TransactionTypeExpense,
	})
	_ = service.Create(context.Background(), tx)

	r := chi.NewRouter()
	r.Delete("/transactions/{id}", handler.DeleteTransaction)

	t.Run("Success 204 No Content", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/transactions/"+tx.ID.String(), nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", rec.Code)
		}

		// Ensure it's deleted
		_, err := service.GetByID(context.Background(), tenantID, tx.ID)
		if err != domain.ErrNotFound {
			t.Errorf("expected transaction to be deleted, got err: %v", err)
		}
	})

	t.Run("Not Found 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/transactions/"+uuid.New().String(), nil)
		req = req.WithContext(middleware.WithTenantID(req.Context(), tenantID))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}
