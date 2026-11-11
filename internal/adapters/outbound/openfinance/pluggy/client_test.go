package pluggy_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/openfinance/pluggy"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func setupMockPluggyServer(t *testing.T) *httptest.Server {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/auth":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["clientId"] == "invalid" {
				http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "mock-api-key-12345"})

		case "/connect_token":
			if r.Header.Get("X-API-KEY") != "mock-api-key-12345" {
				http.Error(w, `{"message":"missing or invalid api key"}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": "mock-connect-token-xyz"})

		case "/items/item-123":
			switch r.Method {
			case http.MethodGet:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":     "item-123",
					"status": "UPDATED",
					"executionStatus": "SUCCESS",
				})
			case http.MethodPatch:
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "UPDATING"})
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			}

		case "/accounts":
			require.Equal(t, "item-123", r.URL.Query().Get("itemId"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{
						"id":           "acc-checking-1",
						"type":         "BANK",
						"name":         "Conta Corrente Nubank",
						"balance":      1450.25,
						"currencyCode": "BRL",
						"number":       "123456-7",
					},
					{
						"id":           "acc-credit-2",
						"type":         "CREDIT",
						"name":         "Cartão Ultravioleta",
						"balance":      -850.50,
						"currencyCode": "BRL",
						"number":       "9988",
					},
				},
			})

		case "/v2/transactions":
			require.Equal(t, "acc-credit-2", r.URL.Query().Get("accountId"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{
						"id":          "tx-pluggy-1",
						"description": "Supermercado Pão de Açúcar",
						"amount":      -120.50,
						"date":        "2026-11-05T12:00:00Z",
						"status":      "POSTED",
						"category":    "Groceries",
						"billId":      "bill-nov-2026",
						"creditCardMetadata": map[string]any{
							"installmentNumber": 1,
							"totalInstallments": 3,
						},
					},
				},
			})

		case "/bills":
			require.Equal(t, "acc-credit-2", r.URL.Query().Get("accountId"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{
						"id":                   "bill-nov-2026",
						"dueDate":              "2026-11-20T00:00:00Z",
						"billClosingDate":      "2026-11-13T00:00:00Z",
						"totalAmount":          850.50,
						"minimumPaymentAmount": 150.00,
						"status":               "OPEN",
						"allowsInstallments":   true,
					},
				},
			})

		default:
			http.NotFound(w, r)
		}
	})

	return httptest.NewServer(handler)
}

func TestPluggyClient_FullLifecycle(t *testing.T) {
	server := setupMockPluggyServer(t)
	defer server.Close()

	ctx := context.Background()
	client := pluggy.NewClient(pluggy.Config{
		BaseURL:      server.URL,
		ClientID:     "client-test-id",
		ClientSecret: "client-test-secret",
		Timeout:      5 * time.Second,
	})

	// 1. Connect token
	tenantID := uuid.New()
	token, expiresAt, err := client.CreateConnectToken(ctx, tenantID, nil)
	require.NoError(t, err)
	require.Equal(t, "mock-connect-token-xyz", token)
	require.True(t, expiresAt.After(time.Now()))

	// 2. Get item status
	item, err := client.GetItem(ctx, "item-123")
	require.NoError(t, err)
	require.Equal(t, "item-123", item.ItemID)
	require.Equal(t, ports.ItemStatusUpdated, item.Status)

	// 3. Fetch accounts
	accounts, err := client.FetchAccounts(ctx, "item-123")
	require.NoError(t, err)
	require.Len(t, accounts, 2)
	require.Equal(t, ports.ExternalAccountChecking, accounts[0].Type)
	require.True(t, decimal.NewFromFloat(1450.25).Equal(accounts[0].Balance))
	require.Equal(t, ports.ExternalAccountCreditCard, accounts[1].Type)

	// 4. Fetch transactions
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)
	txs, err := client.FetchTransactions(ctx, "acc-credit-2", from, to)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, "tx-pluggy-1", txs[0].ID)
	require.Equal(t, "bill-nov-2026", txs[0].BillID)
	require.Equal(t, 1, txs[0].InstallmentNumber)
	require.Equal(t, 3, txs[0].TotalInstallments)

	// 5. Fetch bills
	bills, err := client.FetchBills(ctx, "acc-credit-2")
	require.NoError(t, err)
	require.Len(t, bills, 1)
	require.Equal(t, "bill-nov-2026", bills[0].ID)
	require.Equal(t, "OPEN", bills[0].Status)
	require.True(t, decimal.NewFromFloat(850.50).Equal(bills[0].TotalAmount))
	require.True(t, bills[0].AllowsInstallments)

	// 6. Trigger sync
	require.NoError(t, client.TriggerSync(ctx, "item-123"))

	// 7. Delete connection
	require.NoError(t, client.DeleteConnection(ctx, "item-123"))
}

func TestPluggyClient_AuthFailure(t *testing.T) {
	server := setupMockPluggyServer(t)
	defer server.Close()

	ctx := context.Background()
	client := pluggy.NewClient(pluggy.Config{
		BaseURL:      server.URL,
		ClientID:     "invalid",
		ClientSecret: "wrong",
	})

	_, err := client.GetItem(ctx, "item-123")
	require.Error(t, err)
	require.Contains(t, err.Error(), "pluggy auth failed with status 401")
}
