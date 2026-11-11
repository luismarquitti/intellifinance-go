package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// InvoiceStatus represents the lifecycle state of a credit card bill.
type InvoiceStatus string

// Invoice status definitions.
const (
	// InvoiceStatusOpen indicates the invoice is accepting transactions for the current period.
	InvoiceStatusOpen InvoiceStatus = "OPEN"
	// InvoiceStatusClosed indicates the invoice billing cycle has closed and payment is due.
	InvoiceStatusClosed InvoiceStatus = "CLOSED"
	// InvoiceStatusPaid indicates the invoice has been fully settled.
	InvoiceStatusPaid InvoiceStatus = "PAID"
	// InvoiceStatusOverdue indicates the invoice passed its due date without full settlement.
	InvoiceStatusOverdue InvoiceStatus = "OVERDUE"
)

// CreditCardInvoice represents a monthly or cycle bill for a credit card account.
type CreditCardInvoice struct {
	ID                 uuid.UUID       `json:"id"`
	TenantID           uuid.UUID       `json:"tenant_id"`
	AccountID          uuid.UUID       `json:"account_id"`
	ExternalID         *string         `json:"external_id,omitempty"`
	DueDate            time.Time       `json:"due_date"`
	ClosingDate        *time.Time      `json:"closing_date,omitempty"`
	TotalAmount        decimal.Decimal `json:"total_amount"`
	MinimumPayment     decimal.Decimal `json:"minimum_payment"`
	Currency           string          `json:"currency"`
	Status             InvoiceStatus   `json:"status"`
	AllowsInstallments bool            `json:"allows_installments"`
	Metadata           map[string]any  `json:"metadata"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// NewCreditCardInvoice instantiates and validates a new CreditCardInvoice.
func NewCreditCardInvoice(
	tenantID, accountID uuid.UUID,
	dueDate time.Time,
	totalAmount decimal.Decimal,
	status InvoiceStatus,
) (*CreditCardInvoice, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("tenant_id is required")
	}
	if accountID == uuid.Nil {
		return nil, errors.New("account_id is required")
	}
	if dueDate.IsZero() {
		return nil, errors.New("due_date is required")
	}
	if strings.TrimSpace(string(status)) == "" {
		status = InvoiceStatusOpen
	}

	now := time.Now().UTC()
	return &CreditCardInvoice{
		ID:             uuid.New(),
		TenantID:       tenantID,
		AccountID:      accountID,
		DueDate:        dueDate,
		TotalAmount:    totalAmount.Round(2),
		MinimumPayment: decimal.Zero,
		Currency:       "BRL",
		Status:         status,
		Metadata:       make(map[string]any),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// MarkAsPaid marks the invoice as paid and updates the timestamp.
func (i *CreditCardInvoice) MarkAsPaid() {
	i.Status = InvoiceStatusPaid
	i.UpdatedAt = time.Now().UTC()
}

// CloseInvoice marks an open invoice as closed.
func (i *CreditCardInvoice) CloseInvoice() {
	if i.Status == InvoiceStatusOpen {
		i.Status = InvoiceStatusClosed
		i.UpdatedAt = time.Now().UTC()
	}
}
