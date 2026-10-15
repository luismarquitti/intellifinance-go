package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Tenant represents an isolated multi-tenant boundary (e.g. Household, Personal, Business).
type Tenant struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	Type      TenantType     `json:"type"`
	Currency  string         `json:"currency"`
	Settings  map[string]any `json:"settings"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// NewTenant constructs and validates a new Tenant entity.
func NewTenant(name string, tenantType TenantType, currency string) (*Tenant, error) {
	if strings.TrimSpace(currency) == "" {
		currency = "BRL"
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))

	now := time.Now().UTC()
	t := &Tenant{
		ID:        uuid.New(),
		Name:      strings.TrimSpace(name),
		Type:      tenantType,
		Currency:  currency,
		Settings:  make(map[string]any),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t, nil
}

// Validate ensures tenant invariants are satisfied.
func (t *Tenant) Validate() error {
	if t.ID == uuid.Nil {
		return ErrInvalidID
	}
	if strings.TrimSpace(t.Name) == "" {
		return ErrEmptyName
	}
	if !t.Type.IsValid() {
		return ErrInvalidTenantType
	}
	if len(t.Currency) != 3 || !isAllLetters(t.Currency) {
		return ErrInvalidCurrency
	}
	return nil
}

func isAllLetters(s string) bool {
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}
