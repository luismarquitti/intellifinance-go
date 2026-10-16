package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// CreateAccountRequest defines the payload for creating a new financial account.
type CreateAccountRequest struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	Currency            string `json:"currency,omitempty"`
	InitialBalance      string `json:"initial_balance,omitempty"`
	Visibility          string `json:"visibility,omitempty"`
	InstitutionID       string `json:"institution_id,omitempty"`
	InstitutionName     string `json:"institution_name,omitempty"`
	AccountNumberMasked string `json:"account_number_masked,omitempty"`
	ColorHex            string `json:"color_hex,omitempty"`
	IconName            string `json:"icon_name,omitempty"`
}

// Validate validates input fields of CreateAccountRequest.
func (r *CreateAccountRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam
	if strings.TrimSpace(r.Name) == "" {
		errs = append(errs, problem.InvalidParam{Name: "name", Reason: "cannot be empty"})
	}
	accType := domain.AccountType(strings.ToUpper(strings.TrimSpace(r.Type)))
	if !accType.IsValid() {
		errs = append(errs, problem.InvalidParam{Name: "type", Reason: "must be one of CHECKING, CREDIT_CARD, SAVINGS, INVESTMENT, CASH"})
	}
	if r.Visibility != "" {
		vis := domain.Visibility(strings.ToUpper(strings.TrimSpace(r.Visibility)))
		if !vis.IsValid() {
			errs = append(errs, problem.InvalidParam{Name: "visibility", Reason: "must be one of SHARED, PRIVATE, RESTRICTED"})
		}
	}
	if r.InitialBalance != "" {
		if _, err := decimal.NewFromString(strings.TrimSpace(r.InitialBalance)); err != nil {
			errs = append(errs, problem.InvalidParam{Name: "initial_balance", Reason: "must be a valid decimal number"})
		}
	}
	if r.Currency != "" {
		curr := strings.TrimSpace(r.Currency)
		if len(curr) != 3 {
			errs = append(errs, problem.InvalidParam{Name: "currency", Reason: "must be a 3-letter ISO code"})
		}
	}
	return errs
}

// UpdateAccountRequest defines the payload for updating an account.
type UpdateAccountRequest struct {
	Name                *string `json:"name,omitempty"`
	Visibility          *string `json:"visibility,omitempty"`
	ColorHex            *string `json:"color_hex,omitempty"`
	IconName            *string `json:"icon_name,omitempty"`
	IsActive            *bool   `json:"is_active,omitempty"`
	InstitutionName     *string `json:"institution_name,omitempty"`
	AccountNumberMasked *string `json:"account_number_masked,omitempty"`
}

// Validate validates input fields of UpdateAccountRequest.
func (r *UpdateAccountRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		errs = append(errs, problem.InvalidParam{Name: "name", Reason: "cannot be empty"})
	}
	if r.Visibility != nil {
		vis := domain.Visibility(strings.ToUpper(strings.TrimSpace(*r.Visibility)))
		if !vis.IsValid() {
			errs = append(errs, problem.InvalidParam{Name: "visibility", Reason: "must be one of SHARED, PRIVATE, RESTRICTED"})
		}
	}
	return errs
}

// AccountResponse defines the serializable response for an account.
type AccountResponse struct {
	ID                  uuid.UUID `json:"id"`
	TenantID            uuid.UUID `json:"tenant_id"`
	OwnerUserID         uuid.UUID `json:"owner_user_id"`
	Name                string    `json:"name"`
	Type                string    `json:"type"`
	Currency            string    `json:"currency"`
	Balance             string    `json:"balance"`
	Visibility          string    `json:"visibility"`
	InstitutionID       string    `json:"institution_id,omitempty"`
	InstitutionName     string    `json:"institution_name,omitempty"`
	AccountNumberMasked string    `json:"account_number_masked,omitempty"`
	ColorHex            string    `json:"color_hex"`
	IconName            string    `json:"icon_name"`
	IsActive            bool      `json:"is_active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// AccountListResponse defines the collection envelope for accounts.
type AccountListResponse struct {
	Items []AccountResponse `json:"items"`
}

// ToAccountResponse transforms a domain.Account into AccountResponse.
func ToAccountResponse(acc *domain.Account) AccountResponse {
	return AccountResponse{
		ID:                  acc.ID,
		TenantID:            acc.TenantID,
		OwnerUserID:         acc.OwnerUserID,
		Name:                acc.Name,
		Type:                string(acc.Type),
		Currency:            acc.Currency,
		Balance:             acc.Balance.StringFixed(2),
		Visibility:          string(acc.Visibility),
		InstitutionID:       acc.InstitutionID,
		InstitutionName:     acc.InstitutionName,
		AccountNumberMasked: acc.AccountNumberMasked,
		ColorHex:            acc.ColorHex,
		IconName:            acc.IconName,
		IsActive:            acc.IsActive,
		CreatedAt:           acc.CreatedAt,
		UpdatedAt:           acc.UpdatedAt,
	}
}

// ToAccountListResponse transforms a slice of domain.Account into AccountListResponse.
func ToAccountListResponse(accounts []*domain.Account) AccountListResponse {
	items := make([]AccountResponse, 0, len(accounts))
	for _, a := range accounts {
		items = append(items, ToAccountResponse(a))
	}
	return AccountListResponse{Items: items}
}
