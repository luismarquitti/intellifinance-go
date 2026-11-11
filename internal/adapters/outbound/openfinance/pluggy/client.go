package pluggy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

var _ ports.OpenFinanceConnector = (*Client)(nil)

// Config encapsulates configuration for the Pluggy Open Finance API client.
type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	Timeout      time.Duration
}

// Client implements ports.OpenFinanceConnector communicating with Pluggy REST API.
type Client struct {
	httpClient      *http.Client
	baseURL         string
	clientID        string
	clientSecret    string
	mu              sync.RWMutex
	apiKey          string
	apiKeyExpiresAt time.Time
}

// NewClient constructs a new Pluggy API client.
func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.pluggy.ai"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		httpClient:   &http.Client{Timeout: timeout},
		baseURL:      baseURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	}
}

// getAuthToken ensures an active apiKey is cached or authenticates via POST /auth.
func (c *Client) getAuthToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.apiKey != "" && time.Now().Before(c.apiKeyExpiresAt) {
		token := c.apiKey
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double check inside write lock
	if c.apiKey != "" && time.Now().Before(c.apiKeyExpiresAt) {
		return c.apiKey, nil
	}

	authReqBody := map[string]string{
		"clientId":     c.clientID,
		"clientSecret": c.clientSecret,
	}
	payloadBytes, err := json.Marshal(authReqBody)
	if err != nil {
		return "", fmt.Errorf("failed marshaling pluggy auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed creating auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed executing pluggy auth request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("pluggy auth failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("failed parsing pluggy auth response: %w", err)
	}

	if authResp.APIKey == "" {
		return "", errors.New("pluggy auth response did not contain apiKey")
	}

	c.apiKey = authResp.APIKey
	// Pluggy API keys typically expire in 2 hours; cache for 1h50m
	c.apiKeyExpiresAt = time.Now().Add(110 * time.Minute)

	return c.apiKey, nil
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, body any) (*http.Response, error) {
	token, err := c.getAuthToken(ctx)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed marshaling request body: %w", marshalErr)
		}
		bodyReader = bytes.NewReader(data)
	}

	reqURL := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed building request: %w", err)
	}

	req.Header.Set("X-API-KEY", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pluggy request failed (%s %s): %w", method, reqURL, err)
	}

	return resp, nil
}

// CreateConnectToken generates a short-lived token to initialize the client widget.
func (c *Client) CreateConnectToken(ctx context.Context, tenantID uuid.UUID, itemID *string) (string, time.Time, error) {
	reqBody := map[string]any{
		"options": map[string]any{
			"clientUserId": tenantID.String(),
		},
	}
	if itemID != nil && *itemID != "" {
		reqBody["itemId"] = *itemID
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/connect_token", reqBody)
	if err != nil {
		return "", time.Time{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("failed creating connect token (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", time.Time{}, fmt.Errorf("failed decoding connect token response: %w", err)
	}

	return res.AccessToken, time.Now().Add(30 * time.Minute), nil
}

// GetItem retrieves the latest synchronization status of a bank connection item.
func (c *Client) GetItem(ctx context.Context, itemID string) (*ports.ItemSyncResult, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/items/"+url.PathEscape(itemID), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed fetching item %s (status %d): %s", itemID, resp.StatusCode, string(body))
	}

	var itemResp struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		ExecutionStat struct {
			Status string `json:"status"`
			Error  *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"executionStatus"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&itemResp); err != nil {
		return nil, fmt.Errorf("failed decoding item response: %w", err)
	}

	result := &ports.ItemSyncResult{
		ItemID: itemResp.ID,
		Status: ports.ItemStatus(itemResp.Status),
	}
	if itemResp.Error != nil {
		result.ErrorCode = itemResp.Error.Code
		result.ErrorMessage = itemResp.Error.Message
	} else if itemResp.ExecutionStat.Error != nil {
		result.ErrorCode = itemResp.ExecutionStat.Error.Code
		result.ErrorMessage = itemResp.ExecutionStat.Error.Message
	}

	return result, nil
}

// FetchAccounts retrieves accounts linked to a connection item.
func (c *Client) FetchAccounts(ctx context.Context, itemID string) ([]ports.ExternalAccount, error) {
	endpoint := fmt.Sprintf("/accounts?itemId=%s", url.QueryEscape(itemID))
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed fetching accounts for item %s (status %d): %s", itemID, resp.StatusCode, string(body))
	}

	var res struct {
		Results []struct {
			ID           string  `json:"id"`
			Type         string  `json:"type"`
			Subtype      string  `json:"subtype"`
			Name         string  `json:"name"`
			Balance      float64 `json:"balance"`
			CurrencyCode string  `json:"currencyCode"`
			Number       string  `json:"number"`
			BankData     *struct {
				TransferNumber string `json:"transferNumber"`
			} `json:"bankData"`
			CreditData *struct {
				AvailableCreditLimit float64 `json:"availableCreditLimit"`
			} `json:"creditData"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed decoding accounts response: %w", err)
	}

	var accounts []ports.ExternalAccount
	for _, a := range res.Results {
		accType := ports.ExternalAccountChecking
		switch strings.ToUpper(a.Type) {
		case "CREDIT":
			accType = ports.ExternalAccountCreditCard
		case "SAVINGS":
			accType = ports.ExternalAccountSavings
		case "INVESTMENT":
			accType = ports.ExternalAccountInvestment
		}

		accounts = append(accounts, ports.ExternalAccount{
			ID:           a.ID,
			ItemID:       itemID,
			Type:         accType,
			Name:         a.Name,
			Number:       a.Number,
			Balance:      decimal.NewFromFloat(a.Balance).Round(2),
			CurrencyCode: a.CurrencyCode,
		})
	}

	return accounts, nil
}

// FetchTransactions retrieves transactions for an external account in a date interval.
func (c *Client) FetchTransactions(ctx context.Context, accountID string, from, to time.Time) ([]ports.ExternalTransaction, error) {
	params := url.Values{}
	params.Set("accountId", accountID)
	params.Set("pageSize", "500")
	if !from.IsZero() {
		params.Set("from", from.Format("2006-01-02"))
	}
	if !to.IsZero() {
		params.Set("to", to.Format("2006-01-02"))
	}

	endpoint := "/transactions?" + params.Encode()
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed fetching transactions for account %s (status %d): %s", accountID, resp.StatusCode, string(body))
	}

	var res struct {
		Results []struct {
			ID                 string    `json:"id"`
			Description        string    `json:"description"`
			Amount             float64   `json:"amount"`
			Date               time.Time `json:"date"`
			Status             string    `json:"status"`
			Category           string    `json:"category"`
			PaymentType        string    `json:"paymentType"`
			BillID             *string   `json:"billId"`
			CreditCardMetadata *struct {
				InstallmentNumber int `json:"installmentNumber"`
				TotalInstallments int `json:"totalInstallments"`
			} `json:"creditCardMetadata"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed decoding transactions response: %w", err)
	}

	var txs []ports.ExternalTransaction
	for _, t := range res.Results {
		extTx := ports.ExternalTransaction{
			ID:            t.ID,
			AccountID:     accountID,
			Date:          t.Date,
			Amount:        decimal.NewFromFloat(t.Amount).Round(2),
			Description:   t.Description,
			CategoryHint:  t.Category,
			PaymentMethod: t.PaymentType,
			Status:        t.Status,
		}
		if t.BillID != nil {
			extTx.BillID = *t.BillID
		}
		if t.CreditCardMetadata != nil {
			extTx.InstallmentNumber = t.CreditCardMetadata.InstallmentNumber
			extTx.TotalInstallments = t.CreditCardMetadata.TotalInstallments
		}
		txs = append(txs, extTx)
	}

	return txs, nil
}

// FetchBills retrieves credit card invoices/bills for an external account.
func (c *Client) FetchBills(ctx context.Context, accountID string) ([]ports.ExternalBill, error) {
	endpoint := fmt.Sprintf("/bills?accountId=%s", url.QueryEscape(accountID))
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed fetching bills for account %s (status %d): %s", accountID, resp.StatusCode, string(body))
	}

	var res struct {
		Results []struct {
			ID                   string     `json:"id"`
			DueDate              time.Time  `json:"dueDate"`
			BillClosingDate      *time.Time `json:"billClosingDate"`
			TotalAmount          float64    `json:"totalAmount"`
			MinimumPaymentAmount *float64   `json:"minimumPaymentAmount"`
			Status               string     `json:"status"`
			AllowsInstallments   bool       `json:"allowsInstallments"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed decoding bills response: %w", err)
	}

	var bills []ports.ExternalBill
	for _, b := range res.Results {
		bill := ports.ExternalBill{
			ID:                 b.ID,
			DueDate:            b.DueDate,
			ClosingDate:        b.BillClosingDate,
			TotalAmount:        decimal.NewFromFloat(b.TotalAmount).Round(2),
			Status:             strings.ToUpper(b.Status),
			AllowsInstallments: b.AllowsInstallments,
		}
		if b.MinimumPaymentAmount != nil {
			minDec := decimal.NewFromFloat(*b.MinimumPaymentAmount).Round(2)
			bill.MinimumPaymentAmount = &minDec
		}
		bills = append(bills, bill)
	}

	return bills, nil
}

// TriggerSync requests an on-demand synchronization cycle for an item.
func (c *Client) TriggerSync(ctx context.Context, itemID string) error {
	resp, err := c.doRequest(ctx, http.MethodPatch, "/items/"+url.PathEscape(itemID), nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed triggering sync for item %s (status %d): %s", itemID, resp.StatusCode, string(body))
	}

	return nil
}

// DeleteConnection revokes and deletes a bank item connection.
func (c *Client) DeleteConnection(ctx context.Context, itemID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/items/"+url.PathEscape(itemID), nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed deleting connection item %s (status %d): %s", itemID, resp.StatusCode, string(body))
	}

	return nil
}
