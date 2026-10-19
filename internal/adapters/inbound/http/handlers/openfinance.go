package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

const (
	// HeaderWebhookSecret defines the custom secret header for webhook authentication.
	HeaderWebhookSecret = "X-IntelliFinance-Webhook-Secret"
	// HeaderPluggyTimestamp defines the timestamp header sent by Pluggy for replay protection.
	HeaderPluggyTimestamp = "X-Pluggy-Timestamp"
	// HeaderPluggySignature defines the HMAC-SHA256 signature header sent by Pluggy.
	HeaderPluggySignature = "X-Pluggy-Signature"

	// MaxTimestampDrift defines the maximum allowable clock skew / timestamp drift (5 minutes).
	MaxTimestampDrift = 300 * time.Second
)

// PluggyWebhookPayload represents the incoming webhook notification payload from Open Finance / Pluggy.
type PluggyWebhookPayload struct {
	Event  string `json:"event"`
	ItemID string `json:"itemId"`
	Error  any    `json:"error,omitempty"`
}

// OpenFinanceWebhookHandler handles inbound Open Finance webhook deliveries.
type OpenFinanceWebhookHandler struct {
	secret         string
	tenantResolver ports.OpenFinanceTenantResolver
	queue          ports.TaskQueue
	logger         *slog.Logger
	nowFunc        func() time.Time
}

// WebhookHandlerOption defines functional options for configuring OpenFinanceWebhookHandler.
type WebhookHandlerOption func(*OpenFinanceWebhookHandler)

// WithWebhookClock overrides the default time provider (useful for deterministic tests).
func WithWebhookClock(clock func() time.Time) WebhookHandlerOption {
	return func(h *OpenFinanceWebhookHandler) {
		if clock != nil {
			h.nowFunc = clock
		}
	}
}

// NewOpenFinanceWebhookHandler creates an OpenFinanceWebhookHandler with the required dependencies.
func NewOpenFinanceWebhookHandler(
	secret string,
	tenantResolver ports.OpenFinanceTenantResolver,
	queue ports.TaskQueue,
	logger *slog.Logger,
	opts ...WebhookHandlerOption,
) *OpenFinanceWebhookHandler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &OpenFinanceWebhookHandler{
		secret:         secret,
		tenantResolver: tenantResolver,
		queue:          queue,
		logger:         logger,
		nowFunc:        time.Now,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// SetClock sets a custom clock provider (useful for testing timestamp drift).
func (h *OpenFinanceWebhookHandler) SetClock(clock func() time.Time) {
	if clock != nil {
		h.nowFunc = clock
	}
}

// HandleWebhook processes inbound webhook requests from Open Finance (Pluggy).
func (h *OpenFinanceWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate via Pre-Shared Secret using constant-time comparison
	receivedSecret := r.Header.Get(HeaderWebhookSecret)
	if receivedSecret == "" {
		receivedSecret = r.URL.Query().Get("token")
	}

	if !h.validateSecretConstantTime(receivedSecret) {
		h.logger.WarnContext(r.Context(), "Webhook unauthorized: secret mismatch or missing",
			"remote_addr", r.RemoteAddr,
			"path", r.URL.Path,
		)
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Unauthorized webhook sender."))
		return
	}

	// 2. Replay Protection: Validate X-Pluggy-Timestamp header (maximum 300s / 5m drift)
	timestampStr := r.Header.Get(HeaderPluggyTimestamp)
	if timestampStr == "" {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing webhook timestamp."))
		return
	}
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		h.logger.WarnContext(r.Context(), "Webhook unauthorized: malformed timestamp header",
			"timestamp", timestampStr,
		)
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Malformed webhook timestamp."))
		return
	}

	now := h.nowFunc()
	webhookTime := time.Unix(ts, 0)
	drift := now.Sub(webhookTime)
	if drift < 0 {
		drift = -drift
	}

	if drift > MaxTimestampDrift {
		h.logger.WarnContext(r.Context(), "Webhook unauthorized: timestamp drift exceeds threshold",
			"drift_seconds", drift.Seconds(),
			"max_allowed", MaxTimestampDrift.Seconds(),
		)
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Webhook timestamp out of bounds or expired."))
		return
	}

	// 3. HMAC-SHA256 Signature Validation binds the timestamp and body before processing.
	sigHeader := r.Header.Get(HeaderPluggySignature)
	if sigHeader == "" {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing webhook signature."))
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Failed to read request body."))
		return
	}
	// Reset request body so downstream JSON decoder can read it
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if !h.validateHMACConstantTime(timestampStr, bodyBytes, sigHeader) {
		h.logger.WarnContext(r.Context(), "Webhook unauthorized: HMAC signature mismatch")
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Invalid webhook signature."))
		return
	}

	// 4. Decode JSON payload
	var payload PluggyWebhookPayload
	if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
		h.logger.WarnContext(r.Context(), "Webhook bad request: malformed JSON payload", "err", decodeErr)
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid JSON payload in request body."))
		return
	}

	payload.ItemID = strings.TrimSpace(payload.ItemID)
	if payload.ItemID == "" {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Missing required field 'itemId'."))
		return
	}

	// 5. Resolve target tenant securely via OpenFinanceTenantResolver
	tenantID, err := h.tenantResolver.ResolveTenantByItemID(r.Context(), payload.ItemID)
	if err != nil || tenantID == uuid.Nil {
		h.logger.WarnContext(r.Context(), "Webhook target item unresolvable or unknown",
			"item_id", payload.ItemID,
			"err", err,
		)
		problem.Render(w, r, problem.NewNotFound(r.URL.Path, "Open Finance item not found or tenant unresolvable."))
		return
	}

	// 6. Determine sync type: "FULL" for item/created, "INCREMENTAL" for item/updated and others
	syncType := "INCREMENTAL"
	if payload.Event == "item/created" {
		syncType = "FULL"
	}

	taskPayloadMap := map[string]any{
		"item_id":   payload.ItemID,
		"event":     payload.Event,
		"tenant_id": tenantID.String(),
		"sync_type": syncType,
	}
	if payload.Error != nil {
		taskPayloadMap["error"] = payload.Error
	}

	taskPayloadBytes, err := json.Marshal(taskPayloadMap)
	if err != nil {
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed to serialize task payload."))
		return
	}

	task := &ports.Task{
		ID:         uuid.New().String(),
		Type:       ports.TaskTypeSyncOpenFinance,
		Payload:    taskPayloadBytes,
		MaxRetries: 3,
		Headers: map[string]string{
			"X-Tenant-ID": tenantID.String(),
		},
	}

	if err := h.queue.Enqueue(r.Context(), task, ports.WithMaxRetries(3)); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to enqueue Open Finance sync task",
			"item_id", payload.ItemID,
			"tenant_id", tenantID.String(),
			"err", err,
		)
		problem.Render(w, r, problem.NewInternalError(r.URL.Path, "Failed to enqueue Open Finance sync task."))
		return
	}

	h.logger.InfoContext(r.Context(), "Open Finance webhook processed and sync task dispatched",
		"item_id", payload.ItemID,
		"tenant_id", tenantID.String(),
		"event", payload.Event,
		"sync_type", syncType,
	)

	// 7. Immediate HTTP 202 Accepted response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "QUEUED",
		"item_id":   payload.ItemID,
		"event":     payload.Event,
		"sync_type": syncType,
	})
}

// validateSecretConstantTime compares the incoming secret against the configured expected secret in constant time.
func (h *OpenFinanceWebhookHandler) validateSecretConstantTime(received string) bool {
	if received == "" || h.secret == "" {
		return false
	}

	// Double-defense: hash both strings with SHA-256 to equalize slice lengths to exactly 32 bytes,
	// preventing length-based timing side-channels, and also compare raw bytes in constant time.
	expectedHash := sha256.Sum256([]byte(h.secret))
	receivedHash := sha256.Sum256([]byte(received))

	hashMatch := subtle.ConstantTimeCompare(expectedHash[:], receivedHash[:]) == 1
	byteMatch := subtle.ConstantTimeCompare([]byte(received), []byte(h.secret)) == 1

	return hashMatch && byteMatch
}

// validateHMACConstantTime computes the HMAC-SHA256 of timestamp + "." + body and compares it in constant time.
func (h *OpenFinanceWebhookHandler) validateHMACConstantTime(timestamp string, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	expectedSigLower := strings.ToLower(expectedSig)
	signatureLower := strings.ToLower(signature)

	expectedHash := sha256.Sum256([]byte(expectedSigLower))
	receivedHash := sha256.Sum256([]byte(signatureLower))

	hashMatch := subtle.ConstantTimeCompare(expectedHash[:], receivedHash[:]) == 1
	byteMatch := subtle.ConstantTimeCompare([]byte(signatureLower), []byte(expectedSigLower)) == 1

	return hashMatch && byteMatch
}
