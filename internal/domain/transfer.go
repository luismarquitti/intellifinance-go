package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ValidateTransferPair checks whether two transactions satisfy business invariants
// to be confirmed as an internal transfer between own accounts.
func ValidateTransferPair(legA, legB *Transaction) error {
	if legA == nil || legB == nil {
		return ErrInvalidTransferPair
	}
	if legA.TenantID == uuid.Nil || legB.TenantID == uuid.Nil {
		return ErrInvalidID
	}
	if legA.TenantID != legB.TenantID {
		return ErrTransferCrossTenant
	}
	if legA.ID == legB.ID {
		return ErrInvalidTransferPair
	}
	if legA.AccountID == legB.AccountID {
		return ErrTransferSameAccount
	}
	if legA.Status == TransactionStatusVoid || legB.Status == TransactionStatusVoid {
		return ErrTransferVoidLeg
	}
	if (legA.TransferPeerTransactionID != nil && *legA.TransferPeerTransactionID != legB.ID) ||
		(legB.TransferPeerTransactionID != nil && *legB.TransferPeerTransactionID != legA.ID) {
		return ErrTransferAlreadyLinked
	}
	if legA.Amount.IsZero() || legB.Amount.IsZero() {
		return ErrInvalidAmount
	}
	if !IsRepresentableInCents(legA.Amount) || !IsRepresentableInCents(legB.Amount) {
		return ErrInvalidAmount
	}
	if legA.Amount.Abs().GreaterThan(MaxMonetaryAmount) || legB.Amount.Abs().GreaterThan(MaxMonetaryAmount) {
		return ErrInvalidAmount
	}
	// Strict decimal opposite parity: legA.Amount + legB.Amount must equal exactly 0.00
	if !legA.Amount.Add(legB.Amount).IsZero() {
		return ErrTransferAmountMismatch
	}
	// One leg must be negative and the other must be positive
	if legA.Amount.IsNegative() == legB.Amount.IsNegative() {
		return ErrTransferAmountMismatch
	}
	return nil
}

// ConfirmTransferPair explicitly links two verified journal legs into a confirmed internal transfer.
// Original transaction details, import provenance, notes and dates are preserved.
func ConfirmTransferPair(legA, legB *Transaction) error {
	if err := ValidateTransferPair(legA, legB); err != nil {
		return err
	}

	outflow := legA
	inflow := legB
	if legA.Amount.IsPositive() {
		outflow = legB
		inflow = legA
	}

	// Only settled income/expense legs can be reversibly confirmed. A coherent
	// already-linked transfer is an idempotent confirmation replay.
	settled := func(t *Transaction) bool {
		return t.Status == TransactionStatusCompleted || t.Status == TransactionStatusReconciled
	}
	if !settled(outflow) || !settled(inflow) {
		return ErrInvalidTransferPair
	}
	replay := outflow.Type == TransactionTypeTransfer && inflow.Type == TransactionTypeTransfer &&
		outflow.TransferPeerTransactionID != nil && *outflow.TransferPeerTransactionID == inflow.ID &&
		inflow.TransferPeerTransactionID != nil && *inflow.TransferPeerTransactionID == outflow.ID
	if !replay && (outflow.Type != TransactionTypeExpense || inflow.Type != TransactionTypeIncome || outflow.TransferPeerTransactionID != nil || inflow.TransferPeerTransactionID != nil) {
		return ErrInvalidTransferPair
	}
	outflow.TransferPeerTransactionID = &inflow.ID
	inflow.TransferPeerTransactionID = &outflow.ID

	outflow.Type = TransactionTypeTransfer
	inflow.Type = TransactionTypeTransfer

	now := time.Now().UTC()
	outflow.UpdatedAt = now
	inflow.UpdatedAt = now

	return nil
}

// UnlinkTransferPair breaks an established transfer pairing reversibly without deleting either leg.
func UnlinkTransferPair(legA, legB *Transaction) error {
	if legA == nil || legB == nil {
		return ErrInvalidTransferPair
	}
	if legA.TransferPeerTransactionID == nil || *legA.TransferPeerTransactionID != legB.ID ||
		legB.TransferPeerTransactionID == nil || *legB.TransferPeerTransactionID != legA.ID {
		return ErrTransferNotLinked
	}

	legA.TransferPeerTransactionID = nil
	legB.TransferPeerTransactionID = nil

	if legA.Amount.IsNegative() {
		legA.Type = TransactionTypeExpense
	} else {
		legA.Type = TransactionTypeIncome
	}

	if legB.Amount.IsNegative() {
		legB.Type = TransactionTypeExpense
	} else {
		legB.Type = TransactionTypeIncome
	}

	now := time.Now().UTC()
	legA.UpdatedAt = now
	legB.UpdatedAt = now

	return nil
}

// NewInternalTransfer generates a dual-leg internal transfer ready for atomic persistence.
func NewInternalTransfer(
	tenantID, sourceAccountID, destAccountID uuid.UUID,
	amount decimal.Decimal,
	date time.Time,
	description string,
	createdByUserID *uuid.UUID,
) (*Transaction, *Transaction, error) {
	if tenantID == uuid.Nil || sourceAccountID == uuid.Nil || destAccountID == uuid.Nil {
		return nil, nil, ErrInvalidID
	}
	if sourceAccountID == destAccountID {
		return nil, nil, ErrTransferSameAccount
	}
	if !amount.IsPositive() {
		return nil, nil, ErrInvalidAmount
	}
	if !IsRepresentableInCents(amount) || amount.GreaterThan(MaxMonetaryAmount) {
		return nil, nil, ErrInvalidAmount
	}

	desc := strings.TrimSpace(description)
	if desc == "" {
		desc = "Transferência entre contas próprias"
	}

	outflowID := uuid.New()
	inflowID := uuid.New()

	outflow, err := NewTransaction(NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       sourceAccountID,
		CreatedByUserID: createdByUserID,
		Date:            date,
		Amount:          amount.Neg(),
		Description:     desc,
		Type:            TransactionTypeTransfer,
		Status:          TransactionStatusCompleted,
		Visibility:      VisibilityShared,
		Source:          SourceTypeManual,
	})
	if err != nil {
		return nil, nil, err
	}
	outflow.ID = outflowID
	outflow.TransferPeerTransactionID = &inflowID

	inflow, err := NewTransaction(NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       destAccountID,
		CreatedByUserID: createdByUserID,
		Date:            date,
		Amount:          amount,
		Description:     desc,
		Type:            TransactionTypeTransfer,
		Status:          TransactionStatusCompleted,
		Visibility:      VisibilityShared,
		Source:          SourceTypeManual,
	})
	if err != nil {
		return nil, nil, err
	}
	inflow.ID = inflowID
	inflow.TransferPeerTransactionID = &outflowID

	return outflow, inflow, nil
}
