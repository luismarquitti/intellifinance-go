package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

func TestNewTenant_Success(t *testing.T) {
	tests := []struct {
		name       string
		tenantName string
		tt         domain.TenantType
		currency   string
		expectedC  string
	}{
		{
			name:       "Household with default currency",
			tenantName: "Família Marquitti",
			tt:         domain.TenantTypeHousehold,
			currency:   "",
			expectedC:  "BRL",
		},
		{
			name:       "Personal with USD",
			tenantName: "Luis Personal",
			tt:         domain.TenantTypePersonal,
			currency:   "usd",
			expectedC:  "USD",
		},
		{
			name:       "Business with EUR",
			tenantName: "Claw Tech LTDA",
			tt:         domain.TenantTypeBusiness,
			currency:   "EUR",
			expectedC:  "EUR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tenant, err := domain.NewTenant(tc.tenantName, tc.tt, tc.currency)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tenant.ID == uuid.Nil {
				t.Error("expected non-nil tenant ID")
			}
			if tenant.Name != tc.tenantName {
				t.Errorf("expected name %q, got %q", tc.tenantName, tenant.Name)
			}
			if tenant.Type != tc.tt {
				t.Errorf("expected type %q, got %q", tc.tt, tenant.Type)
			}
			if tenant.Currency != tc.expectedC {
				t.Errorf("expected currency %q, got %q", tc.expectedC, tenant.Currency)
			}
			if tenant.CreatedAt.IsZero() || tenant.UpdatedAt.IsZero() {
				t.Error("expected non-zero timestamps")
			}
		})
	}
}

func TestNewTenant_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		tenantName  string
		tt          domain.TenantType
		currency    string
		expectedErr error
	}{
		{
			name:        "Empty name",
			tenantName:  "   ",
			tt:          domain.TenantTypeHousehold,
			currency:    "BRL",
			expectedErr: domain.ErrEmptyName,
		},
		{
			name:        "Invalid tenant type",
			tenantName:  "Invalid Org",
			tt:          domain.TenantType("INVALID"),
			currency:    "BRL",
			expectedErr: domain.ErrInvalidTenantType,
		},
		{
			name:        "Invalid currency length",
			tenantName:  "My Home",
			tt:          domain.TenantTypeHousehold,
			currency:    "REAL",
			expectedErr: domain.ErrInvalidCurrency,
		},
		{
			name:        "Invalid currency characters",
			tenantName:  "My Home",
			tt:          domain.TenantTypeHousehold,
			currency:    "123",
			expectedErr: domain.ErrInvalidCurrency,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewTenant(tc.tenantName, tc.tt, tc.currency)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestTenant_Validate_NilID(t *testing.T) {
	tenant := &domain.Tenant{
		ID:       uuid.Nil,
		Name:     "Test",
		Type:     domain.TenantTypeHousehold,
		Currency: "BRL",
	}
	if err := tenant.Validate(); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}
