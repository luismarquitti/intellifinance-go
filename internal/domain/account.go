package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Account represents a financial ledger account owned by a user within a tenant.
type Account struct {
	ID                  uuid.UUID       `json:"id"`
	TenantID            uuid.UUID       `json:"tenant_id"`
	OwnerUserID         uuid.UUID       `json:"owner_user_id"`
	Name                string          `json:"name"`
	Type                AccountType     `json:"type"`
	Currency            string          `json:"currency"`
	Balance             decimal.Decimal `json:"balance"`
	Visibility          Visibility      `json:"visibility"`
	InstitutionID       string          `json:"institution_id,omitempty"`
	InstitutionName     string          `json:"institution_name,omitempty"`
	AccountNumberMasked string          `json:"account_number_masked,omitempty"`
	ColorHex            string          `json:"color_hex"`
	IconName            string          `json:"icon_name"`
	IsActive            bool            `json:"is_active"`
	DeletedAt           *time.Time      `json:"deleted_at,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// NewAccount constructs and validates a new financial Account.
func NewAccount(
	tenantID, ownerUserID uuid.UUID,
	name string,
	accType AccountType,
	currency string,
	visibility Visibility,
) (*Account, error) {
	if strings.TrimSpace(currency) == "" {
		currency = "BRL"
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))

	if strings.TrimSpace(string(visibility)) == "" {
		visibility = VisibilityShared
	}

	now := time.Now().UTC()
	acc := &Account{
		ID:          uuid.New(),
		TenantID:    tenantID,
		OwnerUserID: ownerUserID,
		Name:        strings.TrimSpace(name),
		Type:        accType,
		Currency:    currency,
		Balance:     decimal.Zero,
		Visibility:  visibility,
		ColorHex:    "#6366F1",
		IconName:    "account-balance",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := acc.Validate(); err != nil {
		return nil, err
	}
	return acc, nil
}

// Validate ensures account invariants are satisfied.
func (a *Account) Validate() error {
	if a.ID == uuid.Nil || a.TenantID == uuid.Nil || a.OwnerUserID == uuid.Nil {
		return ErrInvalidID
	}
	if strings.TrimSpace(a.Name) == "" {
		return ErrEmptyName
	}
	if !a.Type.IsValid() {
		return ErrInvalidAccountType
	}
	if !a.Visibility.IsValid() {
		return ErrInvalidVisibility
	}
	if len(a.Currency) != 3 || !isAllLetters(a.Currency) {
		return ErrInvalidCurrency
	}
	return nil
}

// ApplyTransaction adjusts the account balance if the transaction status affects ledger balance.
func (a *Account) ApplyTransaction(amount decimal.Decimal, status TransactionStatus) {
	if status.AffectsBalance() {
		a.Balance = a.Balance.Add(amount)
		a.UpdatedAt = time.Now().UTC()
	}
}

// RevertTransaction reverses a previously applied transaction amount from the account balance.
func (a *Account) RevertTransaction(amount decimal.Decimal, status TransactionStatus) {
	if status.AffectsBalance() {
		a.Balance = a.Balance.Sub(amount)
		a.UpdatedAt = time.Now().UTC()
	}
}

// SoftDelete marks an account as deleted without physical row destruction.
func (a *Account) SoftDelete() {
	now := time.Now().UTC()
	a.DeletedAt = &now
	a.IsActive = false
	a.UpdatedAt = now
}

// Restore re-activates a soft-deleted account.
func (a *Account) Restore() {
	a.DeletedAt = nil
	a.IsActive = true
	a.UpdatedAt = time.Now().UTC()
}

// IsDeleted checks if the account has been soft-deleted.
func (a *Account) IsDeleted() bool {
	return a.DeletedAt != nil
}
