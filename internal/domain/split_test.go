package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

func TestNewSplit_Success(t *testing.T) {
	tenantID := uuid.New()
	txID := uuid.New()
	userID := uuid.New()
	amount := decimal.NewFromFloat(50.00)
	pct := decimal.NewFromFloat(50.00)

	split, err := domain.NewSplit(tenantID, txID, userID, amount, pct, "50% split for lunch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if split.ID == uuid.Nil {
		t.Error("expected valid split UUID")
	}
	if split.TenantID != tenantID || split.TransactionID != txID || split.UserID != userID {
		t.Error("expected IDs to match")
	}
	if !split.Amount.Equal(amount) {
		t.Errorf("expected amount %v, got %v", amount, split.Amount)
	}
	if !split.Percentage.Equal(pct) {
		t.Errorf("expected percentage %v, got %v", pct, split.Percentage)
	}
}

func TestNewSplit_ValidationErrors(t *testing.T) {
	tenantID := uuid.New()
	txID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name        string
		tenantID    uuid.UUID
		txID        uuid.UUID
		userID      uuid.UUID
		amount      decimal.Decimal
		pct         decimal.Decimal
		expectedErr error
	}{
		{
			name:        "Nil tenant ID",
			tenantID:    uuid.Nil,
			txID:        txID,
			userID:      userID,
			amount:      decimal.NewFromFloat(50.00),
			pct:         decimal.NewFromFloat(50.00),
			expectedErr: domain.ErrInvalidID,
		},
		{
			name:        "Nil transaction ID",
			tenantID:    tenantID,
			txID:        uuid.Nil,
			userID:      userID,
			amount:      decimal.NewFromFloat(50.00),
			pct:         decimal.NewFromFloat(50.00),
			expectedErr: domain.ErrInvalidID,
		},
		{
			name:        "Nil user ID",
			tenantID:    tenantID,
			txID:        txID,
			userID:      uuid.Nil,
			amount:      decimal.NewFromFloat(50.00),
			pct:         decimal.NewFromFloat(50.00),
			expectedErr: domain.ErrInvalidID,
		},
		{
			name:        "Zero amount",
			tenantID:    tenantID,
			txID:        txID,
			userID:      userID,
			amount:      decimal.Zero,
			pct:         decimal.NewFromFloat(50.00),
			expectedErr: domain.ErrInvalidAmount,
		},
		{
			name:        "Negative percentage",
			tenantID:    tenantID,
			txID:        txID,
			userID:      userID,
			amount:      decimal.NewFromFloat(50.00),
			pct:         decimal.NewFromFloat(-10.00),
			expectedErr: domain.ErrInvalidSplit,
		},
		{
			name:        "Percentage > 100",
			tenantID:    tenantID,
			txID:        txID,
			userID:      userID,
			amount:      decimal.NewFromFloat(50.00),
			pct:         decimal.NewFromFloat(105.00),
			expectedErr: domain.ErrInvalidSplit,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewSplit(tc.tenantID, tc.txID, tc.userID, tc.amount, tc.pct, "")
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestValidateSplits(t *testing.T) {
	tenantID := uuid.New()
	txID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()
	user3 := uuid.New()

	// 1. Valid 50/50 split of an expense (-100.00)
	txExpense := decimal.NewFromFloat(-100.00)
	split1, _ := domain.NewSplit(tenantID, txID, user1, decimal.NewFromFloat(-50.00), decimal.NewFromFloat(50.00), "")
	split2, _ := domain.NewSplit(tenantID, txID, user2, decimal.NewFromFloat(-50.00), decimal.NewFromFloat(50.00), "")

	if err := domain.ValidateSplits(txExpense, []domain.Split{*split1, *split2}); err != nil {
		t.Fatalf("unexpected validation error on valid 50/50 split: %v", err)
	}

	// 2. Valid 3-way split with fractional cents (33.33 + 33.33 + 33.34 = 100.00)
	sA, _ := domain.NewSplit(tenantID, txID, user1, decimal.NewFromFloat(33.33), decimal.NewFromFloat(33.33), "")
	sB, _ := domain.NewSplit(tenantID, txID, user2, decimal.NewFromFloat(33.33), decimal.NewFromFloat(33.33), "")
	sC, _ := domain.NewSplit(tenantID, txID, user3, decimal.NewFromFloat(33.34), decimal.NewFromFloat(33.34), "")

	if err := domain.ValidateSplits(decimal.NewFromFloat(100.00), []domain.Split{*sA, *sB, *sC}); err != nil {
		t.Fatalf("unexpected validation error on valid 3-way split: %v", err)
	}

	// 3. Error on empty splits
	if err := domain.ValidateSplits(txExpense, nil); !errors.Is(err, domain.ErrInvalidSplit) {
		t.Errorf("expected ErrInvalidSplit on empty splits, got %v", err)
	}

	// 4. Error on percentage sum != 100
	badPct1, _ := domain.NewSplit(tenantID, txID, user1, decimal.NewFromFloat(50.00), decimal.NewFromFloat(40.00), "")
	badPct2, _ := domain.NewSplit(tenantID, txID, user2, decimal.NewFromFloat(50.00), decimal.NewFromFloat(40.00), "")
	if err := domain.ValidateSplits(decimal.NewFromFloat(100.00), []domain.Split{*badPct1, *badPct2}); !errors.Is(err, domain.ErrInvalidSplit) {
		t.Errorf("expected ErrInvalidSplit on percentage mismatch, got %v", err)
	}

	// 5. Error on amount sum != transaction amount
	badAmt1, _ := domain.NewSplit(tenantID, txID, user1, decimal.NewFromFloat(40.00), decimal.NewFromFloat(50.00), "")
	badAmt2, _ := domain.NewSplit(tenantID, txID, user2, decimal.NewFromFloat(40.00), decimal.NewFromFloat(50.00), "")
	if err := domain.ValidateSplits(decimal.NewFromFloat(100.00), []domain.Split{*badAmt1, *badAmt2}); !errors.Is(err, domain.ErrInvalidSplit) {
		t.Errorf("expected ErrInvalidSplit on amount mismatch, got %v", err)
	}

	// 6. Error on invalid split in slice
	invalidSplit := domain.Split{
		ID:         uuid.Nil, // Invalid ID
		Amount:     decimal.NewFromFloat(50.00),
		Percentage: decimal.NewFromFloat(50.00),
	}
	if err := domain.ValidateSplits(decimal.NewFromFloat(100.00), []domain.Split{invalidSplit, *split2}); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID on invalid split element, got %v", err)
	}
}
