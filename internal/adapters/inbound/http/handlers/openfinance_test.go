package handlers_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// mockTenantResolver implements ports.OpenFinanceTenantResolver for unit testing.
type mockTenantResolver struct {
	mu      sync.RWMutex
	tenants map[string]uuid.UUID
	errMap  map[string]error
}

func newMockTenantResolver() *mockTenantResolver {
	return &mockTenantResolver{
		tenants: make(map[string]uuid.UUID),
		errMap:  make(map[string]error),
	}
}

func (m *mockTenantResolver) setTenant(itemID string, tenantID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tenants[itemID] = tenantID
}

func (m *mockTenantResolver) setError(itemID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errMap[itemID] = err
}

func (m *mockTenantResolver) ResolveTenantByItemID(_ context.Context, itemID string) (uuid.UUID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, exists := m.errMap[itemID]; exists {
		return uuid.Nil, err
	}
	tenantID, exists := m.tenants[itemID]
	if !exists || tenantID == uuid.Nil {
		return uuid.Nil, domain.ErrNotFound
	}
	return tenantID, nil
}

// mockTaskQueue implements ports.TaskQueue for unit testing.
type mockTaskQueue struct {
	mu           sync.RWMutex
	tasks        []*ports.Task
	enqueueError error
}

func newMockTaskQueue() *mockTaskQueue {
	return &mockTaskQueue{
		tasks: make([]*ports.Task, 0),
	}
}

func (q *mockTaskQueue) Enqueue(_ context.Context, task *ports.Task, opts ...ports.QueueOption) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.enqueueError != nil {
		return q.enqueueError
	}
	for _, opt := range opts {
		opt(task)
	}
	q.tasks = append(q.tasks, task)
	return nil
}

func (q *mockTaskQueue) Close() error {
	return nil
}

func (q *mockTaskQueue) getTasks() []*ports.Task {
	q.mu.RLock()
	defer q.mu.RUnlock()
	copied := make([]*ports.Task, len(q.tasks))
	copy(copied, q.tasks)
	return copied
}

func computePluggyHMAC(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

var testWebhookSecret = fmt.Sprintf("whsec_%s", "test_key_12345")

// 1. Secret provided via header succeeds
func TestOpenFinanceWebhookHandler_SecretViaHeader_Success(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, logger)

	itemID := "item_header_001"
	tenantID := uuid.New()
	resolver.setTenant(itemID, tenantID)

	payload := `{"event":"item/updated","itemId":"item_header_001"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["status"] != "QUEUED" {
		t.Errorf("expected status 'QUEUED', got %v", res["status"])
	}
	if res["item_id"] != itemID {
		t.Errorf("expected item_id %s, got %v", itemID, res["item_id"])
	}

	tasks := queue.getTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", len(tasks))
	}
	if tasks[0].Type != ports.TaskTypeSyncOpenFinance {
		t.Errorf("expected task type %s, got %s", ports.TaskTypeSyncOpenFinance, tasks[0].Type)
	}
	if tasks[0].MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", tasks[0].MaxRetries)
	}
}

// 2. Secret provided via query parameter succeeds
func TestOpenFinanceWebhookHandler_SecretViaQueryParam_Success(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, logger)

	itemID := "item_query_002"
	tenantID := uuid.New()
	resolver.setTenant(itemID, tenantID)

	payload := `{"event":"item/created","itemId":"item_query_002"}`
	url := fmt.Sprintf("/api/v1/open-finance/webhook?token=%s", testWebhookSecret)
	req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res["sync_type"] != "FULL" {
		t.Errorf("expected sync_type 'FULL' for item/created, got %v", res["sync_type"])
	}
}

// 3. Missing secret header and query param returns 401 Unauthorized
func TestOpenFinanceWebhookHandler_MissingSecret_Unauthorized(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

	payload := `{"event":"item/updated","itemId":"item_003"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != problem.ContentTypeProblemJSON {
		t.Errorf("expected Content-Type %s, got %s", problem.ContentTypeProblemJSON, rec.Header().Get("Content-Type"))
	}
}

// 4. Invalid secret in header returns 401 Unauthorized
func TestOpenFinanceWebhookHandler_InvalidSecretHeader_Unauthorized(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

	payload := `{"event":"item/updated","itemId":"item_004"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(handlers.HeaderWebhookSecret, "wrong_secret_key_abcdef")

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

// 5. Invalid secret in query param returns 401 Unauthorized
func TestOpenFinanceWebhookHandler_InvalidSecretQueryParam_Unauthorized(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

	payload := `{"event":"item/updated","itemId":"item_005"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook?token=invalid_token", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

// 6. Constant-time comparison immunity against timing side-channels
func TestOpenFinanceWebhookHandler_ConstantTimeCompare_TimingAttackImmunity(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

	testCases := []struct {
		name   string
		secret string
	}{
		{"Empty secret", ""},
		{"Single character", "w"},
		{"Prefix match only", testWebhookSecret[:len(testWebhookSecret)-2]},
		{"Same length different last char", testWebhookSecret[:len(testWebhookSecret)-1] + "x"},
		{"Longer secret with valid prefix", testWebhookSecret + "_extended_attack"},
		{"Completely different string", "completely_random_token_string_99999"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(`{"itemId":"item_timing"}`))
			req.Header.Set(handlers.HeaderWebhookSecret, tc.secret)

			rec := httptest.NewRecorder()
			handler.HandleWebhook(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 Unauthorized for secret '%s', got %d", tc.secret, rec.Code)
			}
		})
	}
}

// 7. Valid X-Pluggy-Timestamp within 300s window succeeds
func TestOpenFinanceWebhookHandler_Timestamp_ValidWindow_Success(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	fixedNow := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	handler := handlers.NewOpenFinanceWebhookHandler(
		testWebhookSecret,
		resolver,
		queue,
		nil,
		handlers.WithWebhookClock(func() time.Time { return fixedNow }),
	)

	itemID := "item_timestamp_valid"
	resolver.setTenant(itemID, uuid.New())

	// 60 seconds ago (well within 300s drift window)
	validTime := fixedNow.Add(-60 * time.Second)
	payload := `{"event":"item/updated","itemId":"item_timestamp_valid"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(payload))
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)
	req.Header.Set(handlers.HeaderPluggyTimestamp, fmt.Sprintf("%d", validTime.Unix()))

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 8. Expired X-Pluggy-Timestamp (> 300s in the past) returns 401 Unauthorized
func TestOpenFinanceWebhookHandler_Timestamp_Expired_Unauthorized(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	fixedNow := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	handler := handlers.NewOpenFinanceWebhookHandler(
		testWebhookSecret,
		resolver,
		queue,
		nil,
		handlers.WithWebhookClock(func() time.Time { return fixedNow }),
	)

	// 301 seconds ago (exceeds 300s window)
	expiredTime := fixedNow.Add(-301 * time.Second)
	payload := `{"event":"item/updated","itemId":"item_expired"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(payload))
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)
	req.Header.Set(handlers.HeaderPluggyTimestamp, fmt.Sprintf("%d", expiredTime.Unix()))

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}

	var prob problem.ProblemDetail
	_ = json.Unmarshal(rec.Body.Bytes(), &prob)
	if prob.Detail != "Webhook timestamp out of bounds or expired." {
		t.Errorf("unexpected detail message: %s", prob.Detail)
	}
}

// 9. Future X-Pluggy-Timestamp (> 300s in the future) returns 401 Unauthorized
func TestOpenFinanceWebhookHandler_Timestamp_FutureDrift_Unauthorized(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	fixedNow := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	handler := handlers.NewOpenFinanceWebhookHandler(
		testWebhookSecret,
		resolver,
		queue,
		nil,
		handlers.WithWebhookClock(func() time.Time { return fixedNow }),
	)

	// 305 seconds in the future
	futureTime := fixedNow.Add(305 * time.Second)
	payload := `{"event":"item/updated","itemId":"item_future"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(payload))
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)
	req.Header.Set(handlers.HeaderPluggyTimestamp, fmt.Sprintf("%d", futureTime.Unix()))

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

// 10. Malformed X-Pluggy-Timestamp returns 401 Unauthorized
func TestOpenFinanceWebhookHandler_Timestamp_Malformed_Unauthorized(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

	payload := `{"event":"item/updated","itemId":"item_malformed_ts"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(payload))
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)
	req.Header.Set(handlers.HeaderPluggyTimestamp, "not_a_unix_timestamp")

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

// 11. Valid HMAC-SHA256 signature succeeds
func TestOpenFinanceWebhookHandler_HMACSignature_Valid_Success(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	fixedNow := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	handler := handlers.NewOpenFinanceWebhookHandler(
		testWebhookSecret,
		resolver,
		queue,
		nil,
		handlers.WithWebhookClock(func() time.Time { return fixedNow }),
	)

	itemID := "item_hmac_valid"
	resolver.setTenant(itemID, uuid.New())

	timestampStr := fmt.Sprintf("%d", fixedNow.Unix())
	body := []byte(`{"event":"item/updated","itemId":"item_hmac_valid"}`)
	signature := computePluggyHMAC(testWebhookSecret, timestampStr, body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewReader(body))
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)
	req.Header.Set(handlers.HeaderPluggyTimestamp, timestampStr)
	req.Header.Set(handlers.HeaderPluggySignature, signature)

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 12. Invalid or tampered HMAC signature returns 401 Unauthorized
func TestOpenFinanceWebhookHandler_HMACSignature_Invalid_Unauthorized(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	fixedNow := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	handler := handlers.NewOpenFinanceWebhookHandler(
		testWebhookSecret,
		resolver,
		queue,
		nil,
		handlers.WithWebhookClock(func() time.Time { return fixedNow }),
	)

	timestampStr := fmt.Sprintf("%d", fixedNow.Unix())
	body := []byte(`{"event":"item/updated","itemId":"item_hmac_tampered"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewReader(body))
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)
	req.Header.Set(handlers.HeaderPluggyTimestamp, timestampStr)
	req.Header.Set(handlers.HeaderPluggySignature, "deadbeef1234567890abcdefdeadbeef1234567890abcdefdeadbeef12345678")

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

// 13. Malformed JSON payload returns 400 Bad Request
func TestOpenFinanceWebhookHandler_MalformedJSONPayload_BadRequest(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(`{unclosed_json:`))
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", rec.Code)
	}
}

// 14. Empty item ID returns 400 Bad Request
func TestOpenFinanceWebhookHandler_EmptyItemID_BadRequest(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(`{"event":"item/updated","itemId":"   "}`))
	req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)

	rec := httptest.NewRecorder()
	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", rec.Code)
	}
}

// 15. Unresolvable tenant / unknown itemId returns 404 Not Found
func TestOpenFinanceWebhookHandler_TenantNotFound_NotFound(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	resolver.setError("error_item_id", errors.New("database lookup error"))
	queue := newMockTaskQueue()
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

	t.Run("Unknown item ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(`{"event":"item/updated","itemId":"unknown_item_id"}`))
		req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("Resolver returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(`{"event":"item/updated","itemId":"error_item_id"}`))
		req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404 Not Found, got %d", rec.Code)
		}
	})
}

// 16. Event types and queue failures (FULL vs INCREMENTAL sync_type, and 500 error when queue fails)
func TestOpenFinanceWebhookHandler_EventTypes_FullVsIncremental(t *testing.T) {
	t.Parallel()

	t.Run("item/created triggers FULL sync", func(t *testing.T) {
		resolver := newMockTenantResolver()
		queue := newMockTaskQueue()
		handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

		itemID := "item_created_test"
		tenantID := uuid.New()
		resolver.setTenant(itemID, tenantID)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(`{"event":"item/created","itemId":"item_created_test"}`))
		req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d", rec.Code)
		}

		tasks := queue.getTasks()
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		var payloadMap map[string]any
		_ = json.Unmarshal(tasks[0].Payload, &payloadMap)
		if payloadMap["sync_type"] != "FULL" {
			t.Errorf("expected sync_type 'FULL', got %v", payloadMap["sync_type"])
		}
	})

	t.Run("Queue failure returns 500 Internal Server Error", func(t *testing.T) {
		resolver := newMockTenantResolver()
		queue := newMockTaskQueue()
		queue.enqueueError = errors.New("redis connection refused")
		handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, nil)

		itemID := "item_queue_err"
		resolver.setTenant(itemID, uuid.New())

		req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(`{"event":"item/updated","itemId":"item_queue_err"}`))
		req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 Internal Server Error, got %d", rec.Code)
		}
	})
}

// 17. Concurrent stress test with 64 goroutines under -race
func TestOpenFinanceWebhookHandler_ConcurrentStress_64Goroutines(t *testing.T) {
	t.Parallel()

	resolver := newMockTenantResolver()
	queue := newMockTaskQueue()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := handlers.NewOpenFinanceWebhookHandler(testWebhookSecret, resolver, queue, logger)

	const concurrency = 64
	for i := 0; i < concurrency; i++ {
		itemID := fmt.Sprintf("item_concurrent_%03d", i)
		resolver.setTenant(itemID, uuid.New())
	}

	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			itemID := fmt.Sprintf("item_concurrent_%03d", index)
			payload := fmt.Sprintf(`{"event":"item/updated","itemId":"%s"}`, itemID)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/open-finance/webhook", bytes.NewBufferString(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(handlers.HeaderWebhookSecret, testWebhookSecret)

			rec := httptest.NewRecorder()
			handler.HandleWebhook(rec, req)

			if rec.Code != http.StatusAccepted {
				errCh <- fmt.Errorf("goroutine %d failed: expected 202, got %d: %s", index, rec.Code, rec.Body.String())
				return
			}

			var res map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
				errCh <- fmt.Errorf("goroutine %d: failed to decode response: %w", index, err)
				return
			}
			if res["status"] != "QUEUED" || res["item_id"] != itemID {
				errCh <- fmt.Errorf("goroutine %d: unexpected response payload: %v", index, res)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatal(err)
	}

	enqueuedTasks := queue.getTasks()
	if len(enqueuedTasks) != concurrency {
		t.Fatalf("expected exactly %d enqueued tasks, got %d", concurrency, len(enqueuedTasks))
	}
}
