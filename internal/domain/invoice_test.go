package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCreditCardInvoice_ValidationAndLifecycle(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	dueDate := time.Now().AddDate(0, 0, 5)
	amount := decimal.NewFromFloat(1540.50)

	// Validation errors
	_, err := domain.NewCreditCardInvoice(uuid.Nil, accountID, dueDate, amount, domain.InvoiceStatusOpen)
	require.Error(t, err)

	_, err = domain.NewCreditCardInvoice(tenantID, uuid.Nil, dueDate, amount, domain.InvoiceStatusOpen)
	require.Error(t, err)

	_, err = domain.NewCreditCardInvoice(tenantID, accountID, time.Time{}, amount, domain.InvoiceStatusOpen)
	require.Error(t, err)

	// Valid creation
	inv, err := domain.NewCreditCardInvoice(tenantID, accountID, dueDate, amount, "")
	require.NoError(t, err)
	require.NotNil(t, inv)
	require.Equal(t, domain.InvoiceStatusOpen, inv.Status)
	require.True(t, amount.Equal(inv.TotalAmount))
	require.Equal(t, "BRL", inv.Currency)

	// Lifecycle
	inv.CloseInvoice()
	require.Equal(t, domain.InvoiceStatusClosed, inv.Status)

	inv.MarkAsPaid()
	require.Equal(t, domain.InvoiceStatusPaid, inv.Status)
}
