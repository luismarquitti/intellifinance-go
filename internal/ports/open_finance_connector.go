package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ExternalAccountType represents account classifications received from Open Finance providers.
type ExternalAccountType string

const (
	// ExternalAccountChecking represents a checking account.
	ExternalAccountChecking ExternalAccountType = "CHECKING"
	// ExternalAccountCreditCard represents a credit card account.
	ExternalAccountCreditCard ExternalAccountType = "CREDIT_CARD"
	// ExternalAccountSavings represents a savings account.
	ExternalAccountSavings ExternalAccountType = "SAVINGS"
	// ExternalAccountInvestment represents an investment or brokerage account.
	ExternalAccountInvestment ExternalAccountType = "INVESTMENT"
)

// ExternalAccount represents account information provided by an Open Finance aggregator.
type ExternalAccount struct {
	ID           string              `json:"id"`
	ItemID       string              `json:"item_id"`
	Type         ExternalAccountType `json:"type"`
	Name         string              `json:"name"`
	Number       string              `json:"number"`
	Balance      decimal.Decimal     `json:"balance"`
	CurrencyCode string              `json:"currency_code"`
	BankID       string              `json:"bank_id"`
	BankName     string              `json:"bank_name"`
}

// ExternalTransaction represents a transaction record returned from an Open Finance aggregator.
type ExternalTransaction struct {
	ID            string          `json:"id"`
	AccountID     string          `json:"account_id"`
	Date          time.Time       `json:"date"`
	Amount        decimal.Decimal `json:"amount"` // Signed: negative = expense, positive = income
	Description   string          `json:"description"`
	CategoryHint  string          `json:"category_hint,omitempty"`
	PaymentMethod string          `json:"payment_method,omitempty"`
	Status        string          `json:"status"` // COMPLETED, PENDING
}

// ItemStatus represents the synchronization health and lifecycle state of a bank connection.
type ItemStatus string

const (
	// ItemStatusConnecting indicates connection is being established.
	ItemStatusConnecting ItemStatus = "CONNECTING"
	// ItemStatusWaitingUserInput indicates the institution requires MFA or SMS token input.
	ItemStatusWaitingUserInput ItemStatus = "WAITING_USER_INPUT"
	// ItemStatusUpdating indicates crawler is actively retrieving balances and transactions.
	ItemStatusUpdating ItemStatus = "UPDATING"
	// ItemStatusUpdated indicates connection is healthy and reconciled.
	ItemStatusUpdated ItemStatus = "UPDATED"
	// ItemStatusLoginError indicates invalid credentials or blocked access.
	ItemStatusLoginError ItemStatus = "LOGIN_ERROR"
	// ItemStatusOutdated indicates the consent window has expired and requires renewal.
	ItemStatusOutdated ItemStatus = "OUTDATED"
	// ItemStatusError indicates transient institution or communication error.
	ItemStatusError ItemStatus = "ERROR"
)

// ItemSyncResult contains synchronization details and status from the aggregator.
type ItemSyncResult struct {
	ItemID           string         `json:"item_id"`
	Status           ItemStatus     `json:"status"`
	ErrorCode        string         `json:"error_code,omitempty"`
	ErrorMessage     string         `json:"error_message,omitempty"`
	ParameterPrompt  map[string]any `json:"parameter_prompt,omitempty"`
	AccountsFound    int            `json:"accounts_found"`
	TransactionsSync int            `json:"transactions_sync"`
}

// OpenFinanceConnector defines the outbound port for interacting with banking aggregators (e.g. Pluggy).
type OpenFinanceConnector interface {
	// CreateConnectToken generates a short-lived token to initialize the client widget.
	CreateConnectToken(ctx context.Context, tenantID uuid.UUID, itemID *string) (token string, expiresAt time.Time, err error)

	// GetItem retrieves the latest synchronization status of a bank connection item.
	GetItem(ctx context.Context, itemID string) (*ItemSyncResult, error)

	// FetchAccounts retrieves accounts linked to a connection item.
	FetchAccounts(ctx context.Context, itemID string) ([]ExternalAccount, error)

	// FetchTransactions retrieves transactions for an external account in a date interval.
	FetchTransactions(ctx context.Context, accountID string, from, to time.Time) ([]ExternalTransaction, error)

	// TriggerSync requests an on-demand synchronization cycle for an item.
	TriggerSync(ctx context.Context, itemID string) error

	// DeleteConnection revokes and deletes a bank item connection.
	DeleteConnection(ctx context.Context, itemID string) error
}
