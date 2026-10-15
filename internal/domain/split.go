package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Split represents a co-budgeting attribution of a transaction to a specific user.
type Split struct {
	ID            uuid.UUID       `json:"id"`
	TenantID      uuid.UUID       `json:"tenant_id"`
	TransactionID uuid.UUID       `json:"transaction_id"`
	UserID        uuid.UUID       `json:"user_id"`
	Amount        decimal.Decimal `json:"amount"`
	Percentage    decimal.Decimal `json:"percentage"`
	Notes         string          `json:"notes,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// NewSplit constructs and validates an individual transaction split.
func NewSplit(
	tenantID, transactionID, userID uuid.UUID,
	amount, percentage decimal.Decimal,
	notes string,
) (*Split, error) {
	now := time.Now().UTC()
	s := &Split{
		ID:            uuid.New(),
		TenantID:      tenantID,
		TransactionID: transactionID,
		UserID:        userID,
		Amount:        amount,
		Percentage:    percentage,
		Notes:         strings.TrimSpace(notes),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Validate ensures split properties satisfy financial and business constraints.
func (s *Split) Validate() error {
	if s.ID == uuid.Nil || s.TenantID == uuid.Nil || s.TransactionID == uuid.Nil || s.UserID == uuid.Nil {
		return ErrInvalidID
	}
	if s.Amount.IsZero() {
		return ErrInvalidAmount
	}
	if s.Percentage.IsNegative() || s.Percentage.GreaterThan(decimal.NewFromInt(100)) {
		return ErrInvalidSplit
	}
	return nil
}

// ValidateSplits verifies that a group of splits accounts completely for a transaction.
// The sum of split percentages must equal 100.00% (within 0.01 tolerance for rounding),
// and the absolute sum of split amounts must equal the absolute transaction amount.
func ValidateSplits(txAmount decimal.Decimal, splits []Split) error {
	if len(splits) == 0 {
		return ErrInvalidSplit
	}

	target100 := decimal.NewFromInt(100)
	roundingTolerance := decimal.NewFromFloat(0.01)

	sumPct := decimal.Zero
	sumAmount := decimal.Zero
	targetAmount := txAmount.Abs()

	for i := range splits {
		if err := splits[i].Validate(); err != nil {
			return err
		}
		sumPct = sumPct.Add(splits[i].Percentage)
		sumAmount = sumAmount.Add(splits[i].Amount.Abs())
	}

	// Verify total percentage equals 100%
	if sumPct.Sub(target100).Abs().GreaterThan(roundingTolerance) {
		return ErrInvalidSplit
	}

	// Verify total split amounts equal transaction total
	if sumAmount.Sub(targetAmount).Abs().GreaterThan(roundingTolerance) {
		return ErrInvalidSplit
	}

	return nil
}
