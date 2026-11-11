package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// CashflowProjection summarizes liquid cash reserves against upcoming credit card liabilities.
type CashflowProjection struct {
	AvailableBalance  decimal.Decimal             `json:"available_balance"`
	TotalPendingBills decimal.Decimal             `json:"total_pending_bills"`
	ProjectedNet      decimal.Decimal             `json:"projected_net"`
	UpcomingBills     []*domain.CreditCardInvoice `json:"upcoming_bills"`
	LiquidAccounts    []*domain.Account           `json:"liquid_accounts"`
}

// InvoiceService defines business use cases for credit card invoice management and cash flow forecasting.
type InvoiceService interface {
	ListInvoices(ctx context.Context, tenantID uuid.UUID, accountID *uuid.UUID) ([]*domain.CreditCardInvoice, error)
	GetInvoiceDetails(ctx context.Context, tenantID, invoiceID uuid.UUID) (*domain.CreditCardInvoice, []*domain.Transaction, error)
	GetCashflowProjection(ctx context.Context, tenantID uuid.UUID) (*CashflowProjection, error)
}
