package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// InvoiceRepository manages persistence for credit card invoices.
type InvoiceRepository interface {
	Save(ctx context.Context, invoice *domain.CreditCardInvoice) error
	FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.CreditCardInvoice, error)
	FindByExternalID(ctx context.Context, tenantID uuid.UUID, externalID string) (*domain.CreditCardInvoice, error)
	List(ctx context.Context, tenantID uuid.UUID, accountID *uuid.UUID) ([]*domain.CreditCardInvoice, error)
	ListByAccount(ctx context.Context, tenantID, accountID uuid.UUID) ([]*domain.CreditCardInvoice, error)
}
