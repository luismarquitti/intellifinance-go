package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

func TestNewAccount_Success(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	acc, err := domain.NewAccount(
		tenantID,
		userID,
		"Nubank Conjunta",
		domain.AccountTypeChecking,
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if acc.ID == uuid.Nil {
		t.Error("expected valid account ID")
	}
	if acc.TenantID != tenantID || acc.OwnerUserID != userID {
		t.Error("expected matching tenant and owner user IDs")
	}
	if acc.Currency != "BRL" {
		t.Errorf("expected default currency BRL, got %s", acc.Currency)
	}
	if acc.Visibility != domain.VisibilityShared {
		t.Errorf("expected default visibility SHARED, got %s", acc.Visibility)
	}
	if !acc.Balance.Equal(decimal.Zero) {
		t.Errorf("expected initial zero balance, got %v", acc.Balance)
	}
	if !acc.IsActive {
		t.Error("expected account to be active")
	}
	if acc.IsDeleted() {
		t.Error("expected account not to be deleted")
	}
}

func TestNewAccount_ValidationErrors(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name        string
		tenantID    uuid.UUID
		ownerID     uuid.UUID
		accName     string
		accType     domain.AccountType
		currency    string
		visibility  domain.Visibility
		expectedErr error
	}{
		{
			name:        "Nil tenant ID",
			tenantID:    uuid.Nil,
			ownerID:     userID,
			accName:     "Test",
			accType:     domain.AccountTypeChecking,
			currency:    "BRL",
			visibility:  domain.VisibilityShared,
			expectedErr: domain.ErrInvalidID,
		},
		{
			name:        "Nil owner user ID",
			tenantID:    tenantID,
			ownerID:     uuid.Nil,
			accName:     "Test",
			accType:     domain.AccountTypeChecking,
			currency:    "BRL",
			visibility:  domain.VisibilityShared,
			expectedErr: domain.ErrInvalidID,
		},
		{
			name:        "Empty account name",
			tenantID:    tenantID,
			ownerID:     userID,
			accName:     "   ",
			accType:     domain.AccountTypeChecking,
			currency:    "BRL",
			visibility:  domain.VisibilityShared,
			expectedErr: domain.ErrEmptyName,
		},
		{
			name:        "Invalid account type",
			tenantID:    tenantID,
			ownerID:     userID,
			accName:     "Checking",
			accType:     domain.AccountType("WALLET"),
			currency:    "BRL",
			visibility:  domain.VisibilityShared,
			expectedErr: domain.ErrInvalidAccountType,
		},
		{
			name:        "Invalid visibility",
			tenantID:    tenantID,
			ownerID:     userID,
			accName:     "Checking",
			accType:     domain.AccountTypeChecking,
			currency:    "BRL",
			visibility:  domain.Visibility("SECRET"),
			expectedErr: domain.ErrInvalidVisibility,
		},
		{
			name:        "Invalid currency",
			tenantID:    tenantID,
			ownerID:     userID,
			accName:     "Checking",
			accType:     domain.AccountTypeChecking,
			currency:    "US",
			visibility:  domain.VisibilityShared,
			expectedErr: domain.ErrInvalidCurrency,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewAccount(
				tc.tenantID,
				tc.ownerID,
				tc.accName,
				tc.accType,
				tc.currency,
				tc.visibility,
			)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestAccount_BalanceAdjustments(t *testing.T) {
	acc, err := domain.NewAccount(
		uuid.New(),
		uuid.New(),
		"Itaú",
		domain.AccountTypeChecking,
		"BRL",
		domain.VisibilityShared,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Pending transaction does not affect balance
	depositPending := decimal.NewFromFloat(500.00)
	acc.ApplyTransaction(depositPending, domain.TransactionStatusPending)
	if !acc.Balance.Equal(decimal.Zero) {
		t.Errorf("expected balance to remain 0, got %v", acc.Balance)
	}

	// 2. Completed transaction increments balance
	depositCompleted := decimal.NewFromFloat(1500.50)
	acc.ApplyTransaction(depositCompleted, domain.TransactionStatusCompleted)
	if !acc.Balance.Equal(depositCompleted) {
		t.Errorf("expected balance %v, got %v", depositCompleted, acc.Balance)
	}

	// 3. Reconciled expense decrements balance
	expense := decimal.NewFromFloat(-300.25)
	acc.ApplyTransaction(expense, domain.TransactionStatusReconciled)
	expectedAfterExpense := depositCompleted.Add(expense) // 1500.50 + (-300.25) = 1200.25
	if !acc.Balance.Equal(expectedAfterExpense) {
		t.Errorf("expected balance %v, got %v", expectedAfterExpense, acc.Balance)
	}

	// 4. Void transaction does not affect balance
	acc.ApplyTransaction(decimal.NewFromFloat(100.00), domain.TransactionStatusVoid)
	if !acc.Balance.Equal(expectedAfterExpense) {
		t.Errorf("expected balance to remain %v, got %v", expectedAfterExpense, acc.Balance)
	}

	// 5. Revert completed transaction
	acc.RevertTransaction(depositCompleted, domain.TransactionStatusCompleted)
	if !acc.Balance.Equal(expense) {
		t.Errorf("expected balance %v, got %v", expense, acc.Balance)
	}

	// 6. Revert pending transaction does nothing
	acc.RevertTransaction(depositPending, domain.TransactionStatusPending)
	if !acc.Balance.Equal(expense) {
		t.Errorf("expected balance to remain %v, got %v", expense, acc.Balance)
	}
}

func TestAccount_SoftDeleteAndRestore(t *testing.T) {
	acc, err := domain.NewAccount(
		uuid.New(),
		uuid.New(),
		"Itaú",
		domain.AccountTypeChecking,
		"BRL",
		domain.VisibilityShared,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	acc.SoftDelete()
	if !acc.IsDeleted() {
		t.Error("expected account to be deleted")
	}
	if acc.IsActive {
		t.Error("expected deleted account to be inactive")
	}

	acc.Restore()
	if acc.IsDeleted() {
		t.Error("expected restored account not to be deleted")
	}
	if !acc.IsActive {
		t.Error("expected restored account to be active")
	}
}
