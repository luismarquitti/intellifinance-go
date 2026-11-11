package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

var _ ports.InvoiceService = (*InvoiceService)(nil)

// InvoiceService implements credit card invoice inspection and cashflow projections.
type InvoiceService struct {
	invRepo     ports.InvoiceRepository
	txRepo      ports.TransactionRepository
	accountRepo ports.AccountRepository
}

// NewInvoiceService constructs a new InvoiceService instance.
func NewInvoiceService(
	invRepo ports.InvoiceRepository,
	txRepo ports.TransactionRepository,
	accountRepo ports.AccountRepository,
) *InvoiceService {
	return &InvoiceService{
		invRepo:     invRepo,
		txRepo:      txRepo,
		accountRepo: accountRepo,
	}
}

// ListInvoices returns invoices for the tenant, optionally filtered by account.
func (s *InvoiceService) ListInvoices(ctx context.Context, tenantID uuid.UUID, accountID *uuid.UUID) ([]*domain.CreditCardInvoice, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	return s.invRepo.List(ctx, tenantID, accountID)
}

// GetInvoiceDetails retrieves a single invoice alongside its associated transactions.
func (s *InvoiceService) GetInvoiceDetails(ctx context.Context, tenantID, invoiceID uuid.UUID) (*domain.CreditCardInvoice, []*domain.Transaction, error) {
	if tenantID == uuid.Nil || invoiceID == uuid.Nil {
		return nil, nil, domain.ErrInvalidID
	}

	inv, err := s.invRepo.FindByID(ctx, tenantID, invoiceID)
	if err != nil {
		return nil, nil, err
	}

	txs, err := s.txRepo.ListByBillID(ctx, tenantID, invoiceID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed fetching invoice transactions: %w", err)
	}

	return inv, txs, nil
}

// GetCashflowProjection calculates liquid balances vs upcoming open/closed credit card bills.
func (s *InvoiceService) GetCashflowProjection(ctx context.Context, tenantID uuid.UUID) (*ports.CashflowProjection, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}

	// 1. Fetch all accounts for tenant
	accounts, err := s.accountRepo.List(ctx, tenantID, ports.AccountFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed fetching accounts for cashflow projection: %w", err)
	}

	availableBalance := decimal.Zero
	var liquidAccounts []*domain.Account

	for _, acc := range accounts {
		if !acc.IsActive || acc.IsDeleted() {
			continue
		}
		// Consider liquid cash reserves: Checking, Savings, Cash
		if acc.Type == domain.AccountTypeChecking ||
			acc.Type == domain.AccountTypeSavings ||
			acc.Type == domain.AccountTypeCash {
			liquidAccounts = append(liquidAccounts, acc)
			availableBalance = availableBalance.Add(acc.Balance)
		}
	}

	// 2. Fetch all invoices for tenant
	invoices, err := s.invRepo.List(ctx, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed fetching invoices for cashflow projection: %w", err)
	}

	totalPendingBills := decimal.Zero
	var upcomingBills []*domain.CreditCardInvoice

	for _, inv := range invoices {
		// Only consider bills that have not been settled yet (OPEN, CLOSED, OVERDUE)
		if inv.Status == domain.InvoiceStatusOpen ||
			inv.Status == domain.InvoiceStatusClosed ||
			inv.Status == domain.InvoiceStatusOverdue {
			upcomingBills = append(upcomingBills, inv)
			totalPendingBills = totalPendingBills.Add(inv.TotalAmount)
		}
	}

	projectedNet := availableBalance.Sub(totalPendingBills)

	return &ports.CashflowProjection{
		AvailableBalance:  availableBalance,
		TotalPendingBills: totalPendingBills,
		ProjectedNet:      projectedNet,
		UpcomingBills:     upcomingBills,
		LiquidAccounts:    liquidAccounts,
	}, nil
}
