package domain_test

import (
	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

func TestSentinelErrors(t *testing.T) {
	errors := []error{
		domain.ErrNotFound,
		domain.ErrUnauthorized,
		domain.ErrDuplicateTransaction,
		domain.ErrInvalidAmount,
		domain.ErrEmptyDescription,
		domain.ErrEmptyName,
		domain.ErrInvalidEmail,
		domain.ErrInvalidDate,
		domain.ErrInvalidAccountType,
		domain.ErrInvalidTransactionType,
		domain.ErrInvalidVisibility,
		domain.ErrInvalidCurrency,
		domain.ErrInvalidSplit,
		domain.ErrInvalidStatus,
		domain.ErrInvalidSource,
		domain.ErrInvalidTenantType,
		domain.ErrInvalidMemberRole,
		domain.ErrInvalidID,
	}

	for _, err := range errors {
		if err == nil {
			t.Fatal("expected sentinel error not to be nil")
		}
		if err.Error() == "" {
			t.Errorf("expected non-empty error message for %v", err)
		}
	}
}
