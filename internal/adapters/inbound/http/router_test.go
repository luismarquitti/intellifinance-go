package http_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	inboundHttp "github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

type dummyAccountService struct{}

func (d *dummyAccountService) Create(ctx context.Context, account *domain.Account) error {
	return nil
}
func (d *dummyAccountService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Account, error) {
	return nil, domain.ErrNotFound
}
func (d *dummyAccountService) List(ctx context.Context, tenantID uuid.UUID, filter ports.AccountFilter) ([]*domain.Account, error) {
	return []*domain.Account{}, nil
}
func (d *dummyAccountService) Update(ctx context.Context, account *domain.Account) error {
	return nil
}

type dummyCategoryService struct{}

func (d *dummyCategoryService) Create(ctx context.Context, category *domain.Category) error {
	return nil
}
func (d *dummyCategoryService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Category, error) {
	return nil, domain.ErrNotFound
}
func (d *dummyCategoryService) List(ctx context.Context, tenantID uuid.UUID, filter ports.CategoryFilter) ([]*domain.Category, error) {
	return []*domain.Category{}, nil
}
func (d *dummyCategoryService) Update(ctx context.Context, category *domain.Category) error {
	return nil
}

type dummyTransactionService struct{}

func (d *dummyTransactionService) Create(ctx context.Context, tx *domain.Transaction) error {
	return nil
}
func (d *dummyTransactionService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error) {
	return nil, domain.ErrNotFound
}
func (d *dummyTransactionService) List(ctx context.Context, tenantID uuid.UUID, filter ports.TransactionFilter) ([]*domain.Transaction, int64, error) {
	return []*domain.Transaction{}, 0, nil
}
func (d *dummyTransactionService) Update(ctx context.Context, tx *domain.Transaction) error {
	return nil
}
func (d *dummyTransactionService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return nil
}

type dummyTenantResolver struct {
	tenantID uuid.UUID
}

func (r *dummyTenantResolver) ResolveTenantByItemID(_ context.Context, itemID string) (uuid.UUID, error) {
	if itemID == "valid_item" {
		return r.tenantID, nil
	}
	return uuid.Nil, domain.ErrNotFound
}

type dummyTaskQueue struct{}

func (q *dummyTaskQueue) Enqueue(_ context.Context, _ *ports.Task, _ ...ports.QueueOption) error {
	return nil
}

func (q *dummyTaskQueue) Close() error {
	return nil
}

func TestRouter_E2E(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tenantID := uuid.New()
	webhookSecret := "test-router-" + uuid.NewString()
	signWebhook := func(req *http.Request, body string) {
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		mac := hmac.New(sha256.New, []byte(webhookSecret))
		_, _ = mac.Write([]byte(timestamp + "."))
		_, _ = mac.Write([]byte(body))
		req.Header.Set("X-Pluggy-Timestamp", timestamp)
		req.Header.Set("X-Pluggy-Signature", hex.EncodeToString(mac.Sum(nil)))
	}

	router := inboundHttp.NewRouter(inboundHttp.RouterConfig{
		AccountService:            &dummyAccountService{},
		CategoryService:           &dummyCategoryService{},
		TransactionService:        &dummyTransactionService{},
		OpenFinanceTenantResolver: &dummyTenantResolver{tenantID: tenantID},
		TaskQueue:                 &dummyTaskQueue{},
		OpenFinanceWebhookSecret:  webhookSecret,
		Logger:                    logger,
	})

	t.Run("Health checks are public", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("UP")) {
			t.Errorf("expected UP status in body, got %s", rec.Body.String())
		}

		recReady := httptest.NewRecorder()
		reqReady := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		router.ServeHTTP(recReady, reqReady)

		if recReady.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recReady.Code)
		}
	})

	t.Run("Protected route requires X-Tenant-ID", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}

		if rec.Header().Get("Content-Type") != problem.ContentTypeProblemJSON {
			t.Errorf("expected Content-Type %s, got %s", problem.ContentTypeProblemJSON, rec.Header().Get("Content-Type"))
		}

		var prob problem.ProblemDetail
		_ = json.Unmarshal(rec.Body.Bytes(), &prob)
		if prob.Code != problem.CodeUnauthorized {
			t.Errorf("expected code %s, got %s", problem.CodeUnauthorized, prob.Code)
		}
	})

	t.Run("Protected route with X-Tenant-ID succeeds and returns X-Request-ID", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
		req.Header.Set(middleware.HeaderTenantID, tenantID.String())
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		if rec.Header().Get(middleware.HeaderRequestID) == "" {
			t.Error("expected X-Request-ID header to be present on response")
		}
	})

	t.Run("Direct root route also functions", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
		req.Header.Set(middleware.HeaderTenantID, tenantID.String())
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CORS preflight request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/transactions", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Fatalf("expected preflight 200 or 204, got %d", rec.Code)
		}
	})

	t.Run("Open Finance webhook at /api/v1 is exempt from TenantContextMiddleware", func(t *testing.T) {
		rec := httptest.NewRecorder()
		body := `{"event":"item/updated","itemId":"valid_item"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-IntelliFinance-Webhook-Secret", webhookSecret)
		signWebhook(req, body)
		// Explicitly DO NOT set X-Tenant-ID header
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected status 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Open Finance webhook at root /open-finance/webhook is exempt from TenantContextMiddleware", func(t *testing.T) {
		rec := httptest.NewRecorder()
		body := `{"event":"item/created","itemId":"valid_item"}`
		req := httptest.NewRequest(http.MethodPost, "/open-finance/webhook", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-IntelliFinance-Webhook-Secret", webhookSecret)
		signWebhook(req, body)
		// Explicitly DO NOT set X-Tenant-ID header
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected status 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Open Finance webhook without secret returns 401 Unauthorized", func(t *testing.T) {
		rec := httptest.NewRecorder()
		body := `{"event":"item/updated","itemId":"valid_item"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
