# IntelliFinance Technical Specification: Open Finance Connector & Pluggy Integration

- **Document ID**: SPEC-004-OPENFINANCE
- **Status**: Authoritative / Production-Ready
- **Hexagonal Role**: Outbound Adapter (`internal/adapters/outbound/openfinance`) & Inbound Webhook Handler
- **Open Finance Provider**: Pluggy.ai (Brazil Financial Aggregator)
- **Execution Modes**: Dual-Mode (Perpetual Free Sandbox for Dev/Portfolio vs Configurable Live for Production)

---

## 1. Overview & Architectural Role

The Open Finance subsystem enables automated, real-time banking synchronization across Brazilian financial institutions (including Itaú, Nubank, Banco do Brasil, Bradesco, Santander, Inter, C6 Bank, XP, and BTG Pactual).

The architecture maintains strict decoupling between the domain layer and Pluggy APIs:
1. **Zero Vendor Lock-In**: The domain interacts strictly through the `ports.OpenFinanceConnector` interface.
2. **Dual-Mode Operation**:
   - **Sandbox Mode ($0 Cost)**: Provides simulated institutions, deterministic credential scenarios, MFA testing, and infinite mock accounts for local development, CI/CD automated tests, and the public portfolio showcase on GitHub.
   - **Live Production Mode**: Connects to real Brazilian banking APIs with tenant-isolated credential encryption, automatic daily balance sync, and webhook-driven incremental transaction streaming.
3. **Defense-in-Depth Security**: Bank credentials are never ingested or stored by IntelliFinance servers. Authentication occurs within Pluggy's certified Connect Widget iframe/modal. Server API keys and client access tokens are encrypted with **AES-256-GCM** using keys supplied by GCP Secret Manager.

---

## 2. Authentication Flow & Token Lifecycle

Pluggy enforces a hierarchical two-tier token model:

```
+---------------------------------------------------------------------------------------------------+
|                                     AUTHENTICATION TOPOLOGY                                       |
|                                                                                                   |
|  [ Backend API ]                                                                                  |
|        |                                                                                          |
|        | 1. POST https://api.pluggy.ai/auth { clientId, clientSecret }                            |
|        v                                                                                          |
|  [ Pluggy Server Auth ] ---> Returns Server API Key (TTL: 2 Hours)                                |
|        |                                                                                          |
|        | (Cached in Memory / Redis for 1h 50m)                                                    |
|        |                                                                                          |
|        | 2. POST https://api.pluggy.ai/connect_token                                               |
|        |    Headers: { X-API-KEY: <server_api_key> }                                              |
|        |    Body: { clientUserId: "<tenant_id>", webhookUrl: "..." }                              |
|        v                                                                                          |
|  [ Pluggy Token Service ] ---> Returns Connect Token (TTL: 30 Minutes)                            |
|        |                                                                                          |
|        | 3. Connect Token passed to Frontend                                                      |
|        v                                                                                          |
|  [ Frontend / Mobile App ]                                                                        |
|        |                                                                                          |
|        | 4. Launches Pluggy Connect Widget (Iframe / SDK) with Connect Token                      |
|        v                                                                                          |
|  [ Bank Authentication & MFA ] ---> User Authenticates with Institution                           |
|        |                                                                                          |
|        v                                                                                          |
|  [ Pluggy Event Callback ] ---> Dispatches `item/created` or `item/updated` Webhook to Backend    |
+---------------------------------------------------------------------------------------------------+
```

### 2.1 Backend Server-to-Server Authentication (`/auth`)
- **Endpoint**: `POST https://api.pluggy.ai/auth`
- **Payload**:
```json
{
  "clientId": "${PLUGGY_CLIENT_ID}",
  "clientSecret": "${PLUGGY_CLIENT_SECRET}"
}
```
- **Response `200 OK`**:
```json
{
  "apiKey": "pk_live_a1b2c3d4e5f6..."
}
```
- **Caching Invariant**: The Go client caches `apiKey` in memory with an expiration buffer of 10 minutes (TTL = 110 minutes), refreshing lazily upon expiration or upon receiving an HTTP 401 response.

### 2.2 Client-Scoped Connect Token (`/connect_token`)
- **Endpoint**: `POST https://api.pluggy.ai/connect_token`
- **Headers**: `X-API-KEY: pk_live_...`, `Content-Type: application/json`
- **Payload**:
```json
{
  "clientUserId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "itemId": "8a6e87f1-7391-49b4-b4a1-094982635412",
  "options": {
    "webhookUrl": "https://api.intellifinance.app/api/v1/open-finance/webhook"
  }
}
```
- **Response `200 OK`**:
```json
{
  "accessToken": "ct_eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

---

## 3. Go Interface Contracts & Connector Implementation

### 3.1 Hexagonal Port Definition (`internal/ports/openfinance.go`)

```go
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ExternalAccountType string

const (
	ExternalAccountChecking   ExternalAccountType = "CHECKING"
	ExternalAccountCreditCard ExternalAccountType = "CREDIT_CARD"
	ExternalAccountSavings    ExternalAccountType = "SAVINGS"
	ExternalAccountInvestment ExternalAccountType = "INVESTMENT"
)

type ExternalAccount struct {
	ID                    string              `json:"id"`
	ItemID                string              `json:"item_id"`
	Type                  ExternalAccountType `json:"type"`
	Name                  string              `json:"name"`
	Number                string              `json:"number"`
	Balance               decimal.Decimal     `json:"balance"`
	CurrencyCode          string              `json:"currency_code"`
	BankID                string              `json:"bank_id"`
	BankName              string              `json:"bank_name"`
}

type ExternalTransaction struct {
	ID               string          `json:"id"`
	AccountID        string          `json:"account_id"`
	Date             time.Time       `json:"date"`
	Amount           decimal.Decimal `json:"amount"` // Signed: negative = expense, positive = income
	Description      string          `json:"description"`
	CategoryHint     string          `json:"category_hint"`
	PaymentMethod    string          `json:"payment_method"`
	Status           string          `json:"status"` // POSTED, PENDING
}

type ItemStatus string

const (
	ItemStatusConnecting       ItemStatus = "CONNECTING"
	ItemStatusWaitingUserInput ItemStatus = "WAITING_USER_INPUT"
	ItemStatusUpdating         ItemStatus = "UPDATING"
	ItemStatusUpdated          ItemStatus = "UPDATED"
	ItemStatusLoginError       ItemStatus = "LOGIN_ERROR"
	ItemStatusOutdated         ItemStatus = "OUTDATED"
	ItemStatusError            ItemStatus = "ERROR"
)

type ItemSyncResult struct {
	ItemID           string
	Status           ItemStatus
	ErrorCode        string
	ErrorMessage     string
	ParameterPrompt  map[string]any
	AccountsFound    int
	TransactionsSync int
}

type OpenFinanceConnector interface {
	CreateConnectToken(ctx context.Context, tenantID uuid.UUID, itemID *string) (token string, expiresAt time.Time, err error)
	GetItem(ctx context.Context, itemID string) (*ItemSyncResult, error)
	FetchAccounts(ctx context.Context, itemID string) ([]ExternalAccount, error)
	FetchTransactions(ctx context.Context, accountID string, from, to time.Time) ([]ExternalTransaction, error)
	TriggerSync(ctx context.Context, itemID string) error
	DeleteConnection(ctx context.Context, itemID string) error
}
```

### 3.2 Concrete Client Implementation (`internal/adapters/outbound/openfinance/pluggy_client.go`)

```go
package openfinance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"intellifinance/internal/ports"
)

type PluggyClient struct {
	httpClient *http.Client
	baseURL    string
	clientID   string
	secret     string
	webhookURL string

	mu        sync.RWMutex
	apiKey    string
	expiresAt time.Time
}

func NewPluggyClient(clientID, secret, webhookURL, environment string) *PluggyClient {
	baseURL := "https://api.pluggy.ai"
	return &PluggyClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		clientID:   clientID,
		secret:     secret,
		webhookURL: webhookURL,
	}
}

func (c *PluggyClient) authenticate(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.apiKey != "" && time.Now().Before(c.expiresAt) {
		token := c.apiKey
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double check lock
	if c.apiKey != "" && time.Now().Before(c.expiresAt) {
		return c.apiKey, nil
	}

	payload, _ := json.Marshal(map[string]string{
		"clientId":     c.clientID,
		"clientSecret": c.secret,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("pluggy auth call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("pluggy auth returned %d: %s", resp.StatusCode, string(body))
	}

	var authResp struct {
		ApiKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	c.apiKey = authResp.ApiKey
	c.expiresAt = time.Now().Add(110 * time.Minute) // 1h 50m cache
	return c.apiKey, nil
}

func (c *PluggyClient) CreateConnectToken(ctx context.Context, tenantID uuid.UUID, itemID *string) (string, time.Time, error) {
	apiKey, err := c.authenticate(ctx)
	if err != nil {
		return "", time.Time{}, err
	}

	// Sign webhook URL with tenant context and HMAC token to ensure delivery authentication
	// even when aggregator proxies omit custom headers.
	webhookToken := c.generateWebhookSignature(tenantID)
	signedWebhookURL := fmt.Sprintf("%s?tenant_id=%s&token=%s", c.webhookURL, tenantID.String(), webhookToken)

	bodyData := map[string]any{
		"clientUserId": tenantID.String(),
		"options": map[string]string{
			"webhookUrl": signedWebhookURL,
		},
	}
	if itemID != nil && *itemID != "" {
		bodyData["itemId"] = *itemID
	}

	payload, _ := json.Marshal(bodyData)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/connect_token", bytes.NewReader(payload))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("failed connect_token (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, err
	}

	return tokenResp.AccessToken, time.Now().Add(30 * time.Minute), nil
}

func (c *PluggyClient) FetchTransactions(ctx context.Context, accountID string, from, to time.Time) ([]ports.ExternalTransaction, error) {
	apiKey, err := c.authenticate(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/transactions?accountId=%s&from=%s&to=%s&pageSize=500",
		c.baseURL, accountID, from.Format("2006-01-02"), to.Format("2006-01-02"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pluggy transactions failed (%d): %s", resp.StatusCode, string(body))
	}

	var listResp struct {
		Results []struct {
			ID          string          `json:"id"`
			AccountId   string          `json:"accountId"`
			Date        string          `json:"date"`
			Amount      decimal.Decimal `json:"amount"`
			Description string          `json:"description"`
			Category    string          `json:"category"`
			PaymentData struct {
				PaymentMethod string `json:"paymentMethod"`
			} `json:"paymentData"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	txs := make([]ports.ExternalTransaction, len(listResp.Results))
	for i, r := range listResp.Results {
		parsedDate, _ := time.Parse(time.RFC3339, r.Date)
		if parsedDate.IsZero() {
			parsedDate, _ = time.Parse("2006-01-02", r.Date)
		}

		txs[i] = ports.ExternalTransaction{
			ID:            r.ID,
			AccountID:     r.AccountId,
			Date:          parsedDate,
			Amount:        r.Amount, // Pluggy provides signed amount
			Description:   r.Description,
			CategoryHint:  r.Category,
			PaymentMethod: r.PaymentData.PaymentMethod,
			Status:        "COMPLETED",
		}
	}

	return txs, nil
}
```

---

## 4. Sandbox Mode Test Scenarios & Showcase Configuration

When running in sandbox mode (`PLUGGY_ENVIRONMENT=sandbox`), the system consumes zero financial credits while providing deterministic bank behaviors:

### 4.1 Mock Credentials Matrix

| Institution | Connector ID | Username / CPF | Password | MFA Code | Expected Result |
|---|---|---|---|---|---|
| **Itaú Sandbox** | 201 | `user-ok` / `000.000.000-00` | `password-ok` | `123456` | Immediate success (`UPDATED`), 3 accounts, 50 mock transactions. |
| **Nubank Sandbox** | 202 | `user-ok` | `password-ok` | `123456` | Immediate success (`UPDATED`), checking & credit card accounts. |
| **Bradesco Error** | 2 | `user-error` | `password-invalid`| N/A | Triggers `LOGIN_ERROR` (code: `INVALID_CREDENTIALS`). |
| **Inter MFA Flow** | 4 | `user-mfa` | `password-ok` | `123456` | Enters `WAITING_USER_INPUT` requesting SMS token. Submitting `123456` transitions to `UPDATED`. |
| **Santander Outdated** | 1 | `user-outdated`| `password-outdated`| N/A | Triggers `OUTDATED` credentials warning requiring user reconnection. |

---

## 5. Webhook Security & Event Ingestion Pipeline

### 5.1 Verification via Secret, Replay Protection & HMAC Signatures
Pluggy allows configuring custom headers or signed callback endpoints when registering webhooks. IntelliFinance secures webhook endpoints with defense-in-depth:
1. **Pre-Shared Secret / Token Validation**: Checks `X-IntelliFinance-Webhook-Secret` or query parameter token.
2. **Replay Protection**: Verifies `X-Pluggy-Timestamp` with a strict 300-second maximum drift window ($|T_{\text{now}} - T_{\text{webhook}}| \le 300\text{s}$).
3. **Payload Signature**: Verifies constant-time HMAC-SHA256 signature when `X-Pluggy-Signature` is provided.

```go
package http

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func VerifyPluggyWebhook(next http.Handler) http.Handler {
	expectedSecret := []byte(os.Getenv("PLUGGY_WEBHOOK_SECRET"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Secret verification (header or signed URL parameter fallback)
		receivedSecret := r.Header.Get("X-IntelliFinance-Webhook-Secret")
		if receivedSecret == "" {
			receivedSecret = r.URL.Query().Get("token")
		}
		if receivedSecret == "" || subtle.ConstantTimeCompare([]byte(receivedSecret), expectedSecret) != 1 {
			http.Error(w, `{"error":"unauthorized webhook sender"}`, http.StatusUnauthorized)
			return
		}

		// 2. Replay Protection: Timestamp Header Validation (max 5 minutes drift)
		timestampStr := r.Header.Get("X-Pluggy-Timestamp")
		if timestampStr != "" {
			ts, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil || time.Since(time.Unix(ts, 0)).Abs() > 5*time.Minute {
				http.Error(w, `{"error":"webhook timestamp out of bounds or expired"}`, http.StatusUnauthorized)
				return
			}

			// 3. HMAC-SHA256 Signature Validation over Timestamp + Payload
			sigHeader := r.Header.Get("X-Pluggy-Signature")
			if sigHeader != "" {
				bodyBytes, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				mac := hmac.New(sha256.New, expectedSecret)
				mac.Write([]byte(timestampStr + "."))
				mac.Write(bodyBytes)
				expectedSig := hex.EncodeToString(mac.Sum(nil))

				if subtle.ConstantTimeCompare([]byte(sigHeader), []byte(expectedSig)) != 1 {
					http.Error(w, `{"error":"invalid webhook signature"}`, http.StatusUnauthorized)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
```

### 5.2 Supported Webhook Event Types & Actions

| Event Name | Payload Summary | Backend Action |
|---|---|---|
| `item/created` | `{ "itemId": "...", "event": "item/created" }` | Registers `open_finance_items` row; enqueues `task:sync_open_finance` with `sync_type: FULL`. |
| `item/updated` | `{ "itemId": "...", "event": "item/updated" }` | Enqueues `task:sync_open_finance` with `sync_type: INCREMENTAL` for last 7 days. |
| `item/error` | `{ "itemId": "...", "error": { "code": "...", "message": "..." } }` | Updates item status to `LOGIN_ERROR` or `ERROR`; creates audit log; notifies user. |
| `item/waiting_user_input` | `{ "itemId": "...", "parameter": { ... } }` | Updates item status to `WAITING_USER_INPUT`; saves prompt schema to notify frontend. |
| `transactions/deleted`| `{ "itemId": "...", "transactionIds": [...] }` | Voids or soft-deletes matching transactions; recalculates account balances. |

### 5.3 Webhook Tenant Resolution & Session Scoping (Resolving RLS Deadlock)

When Pluggy dispatches an asynchronous event, the payload contains an external `itemId` without user credentials or tenant session context:

```json
{
  "event": "item/updated",
  "itemId": "8a6e87f1-7391-49b4-b4a1-094982635412"
}
```

Because `open_finance_items` enforces strict Row-Level Security, a standard query without tenant context returns 0 rows. To resolve this cleanly without exposing other tenant records or granting broad table access, the webhook receiver calls the PostgreSQL `SECURITY DEFINER` function `resolve_open_finance_tenant(p_item_id VARCHAR)`:

```go
func (h *WebhookHandler) HandlePluggyEvent(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Event  string `json:"event"`
		ItemID string `json:"itemId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	// Safely lookup tenant via SECURITY DEFINER function with strict sanitization
	var tenantID uuid.UUID
	err := h.db.QueryRow(r.Context(), "SELECT resolve_open_finance_tenant($1)", payload.ItemID).Scan(&tenantID)
	if err != nil || tenantID == uuid.Nil {
		h.logger.WarnContext(r.Context(), "Webhook received for unknown or unregistered item", "itemId", payload.ItemID)
		http.Error(w, `{"error":"item not found"}`, http.StatusNotFound)
		return
	}

	// Dispatch background worker task with resolved tenant context
	err = h.queue.Enqueue(r.Context(), &ports.Task{
		Type:     "task:sync_open_finance",
		TenantID: tenantID,
		Payload: map[string]any{
			"item_id":   payload.ItemID,
			"event":     payload.Event,
			"sync_type": "INCREMENTAL",
		},
	})
	if err != nil {
		http.Error(w, `{"error":"failed to enqueue sync job"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"enqueued"}`))
}
```

---

## 6. Sync State Machine & Error Recovery

```
                  +-----------------------+
                  |     DISCONNECTED      |
                  +-----------------------+
                              |
                     (Connect Widget)
                              v
                  +-----------------------+
                  |      CONNECTING       |
                  +-----------------------+
                              |
              +---------------+---------------+
              |                               |
    (Requires MFA Challenge)         (Valid Direct Auth)
              v                               v
+---------------------------+     +-----------------------+
|    WAITING_USER_INPUT     |     |       UPDATING        |
+---------------------------+     +-----------------------+
              |                               |
    (User Submits Token)             (Sync Completes)
              v                               v
      [ Validate MFA ]            +-----------------------+
              |                   |        UPDATED        |
              +------------------>|  (Normal Sync State) |
                                  +-----------------------+
                                              |
                          +-------------------+-------------------+
                          |                                       |
                   (Bad Password)                         (Session Expired)
                          v                                       v
              +-----------------------+               +-----------------------+
              |      LOGIN_ERROR      |               |       OUTDATED        |
              |  (Prompt New Password)|               | (Re-authenticate Item)|
              +-----------------------+               +-----------------------+
```

### 6.1 State Definitions & Actions
1. **`CONNECTING`**: Connection initialized in Connect Widget. Waiting for bank handshake.
2. **`WAITING_USER_INPUT`**: Institution has issued an out-of-band challenge (SMS OTP, token generator, captcha). The client fetches the required parameter schema from `GET /api/v1/open-finance/items` and renders an interactive prompt.
3. **`UPDATING`**: Pluggy crawler is querying account balances and transaction pages.
4. **`UPDATED`**: Sync successfully completed. Accounts and transactions are reconciled.
5. **`LOGIN_ERROR`**: Invalid bank password or blocked access card. Requires user to update credentials in widget.
6. **`OUTDATED`**: Bank consent window (typically 365 days under Brazilian Open Finance regulations) has expired. Requires re-authorization.
7. **`ERROR`**: Transient upstream bank downtime. The queue scheduler applies exponential backoff and retries in 2 hours.
