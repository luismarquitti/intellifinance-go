package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockInvoiceService struct {
	invoices   []*domain.CreditCardInvoice
	txs        []*domain.Transaction
	projection *ports.CashflowProjection
	err        error
}

func (m *mockInvoiceService) ListInvoices(ctx context.Context, tenantID uuid.UUID, accountID *uuid.UUID) ([]*domain.CreditCardInvoice, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.invoices, nil
}

func (m *mockInvoiceService) GetInvoiceDetails(ctx context.Context, tenantID, invoiceID uuid.UUID) (*domain.CreditCardInvoice, []*domain.Transaction, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	for _, inv := range m.invoices {
		if inv.ID == invoiceID {
			return inv, m.txs, nil
		}
	}
	return nil, nil, domain.ErrNotFound
}

func (m *mockInvoiceService) GetCashflowProjection(ctx context.Context, tenantID uuid.UUID) (*ports.CashflowProjection, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.projection, nil
}

func TestInvoiceHandler_Endpoints(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	invoiceID := uuid.New()

	mockInv := &domain.CreditCardInvoice{
		ID:          invoiceID,
		TenantID:    tenantID,
		AccountID:   accountID,
		DueDate:     time.Now().AddDate(0, 0, 10),
		TotalAmount: decimal.NewFromInt(1250),
		Status:      domain.InvoiceStatusOpen,
	}

	mockSvc := &mockInvoiceService{
		invoices: []*domain.CreditCardInvoice{mockInv},
		txs:      []*domain.Transaction{},
		projection: &ports.CashflowProjection{
			AvailableBalance:  decimal.NewFromInt(3000),
			TotalPendingBills: decimal.NewFromInt(1250),
			ProjectedNet:      decimal.NewFromInt(1750),
		},
	}

	handler := handlers.NewInvoiceHandler(mockSvc)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := middleware.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/invoices", handler.ListInvoices)
	r.Get("/invoices/{id}", handler.GetInvoice)
	r.Get("/cashflow/projection", handler.GetCashflowProjection)

	// 1. Test ListInvoices
	t.Run("ListInvoices", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/invoices", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var list []*domain.CreditCardInvoice
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
		assert.Len(t, list, 1)
		assert.Equal(t, invoiceID, list[0].ID)
	})

	// 2. Test GetInvoice
	t.Run("GetInvoice_Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/invoices/"+invoiceID.String(), nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var res struct {
			Invoice      *domain.CreditCardInvoice `json:"invoice"`
			Transactions []*domain.Transaction     `json:"transactions"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		assert.Equal(t, invoiceID, res.Invoice.ID)
	})

	t.Run("GetInvoice_NotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/invoices/"+uuid.New().String(), nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	// 3. Test GetCashflowProjection
	t.Run("GetCashflowProjection", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cashflow/projection", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var proj ports.CashflowProjection
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &proj))
		assert.True(t, proj.AvailableBalance.Equal(decimal.NewFromInt(3000)))
		assert.True(t, proj.ProjectedNet.Equal(decimal.NewFromInt(1750)))
	})
}
