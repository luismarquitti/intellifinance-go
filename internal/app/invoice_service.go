package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

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

// GetInvoiceDetails retrieves a single invoice alongside its associated card purchases and refunds.
func (s *InvoiceService) GetInvoiceDetails(ctx context.Context, tenantID, invoiceID uuid.UUID) (*domain.CreditCardInvoice, []*domain.Transaction, error) {
	if tenantID == uuid.Nil || invoiceID == uuid.Nil {
		return nil, nil, domain.ErrInvalidID
	}

	inv, err := s.invRepo.FindByID(ctx, tenantID, invoiceID)
	if err != nil {
		return nil, nil, err
	}

	rawTxs, err := s.txRepo.ListByBillID(ctx, tenantID, invoiceID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed fetching invoice transactions: %w", err)
	}

	txs := make([]*domain.Transaction, 0, len(rawTxs))
	for _, tx := range rawTxs {
		if tx.IsInvoiceSettlement() {
			continue
		}
		txs = append(txs, tx)
	}

	return inv, txs, nil
}

// GetCashflowProjection calculates liquid balances vs upcoming open/closed credit card bills.
func (s *InvoiceService) GetCashflowProjection(ctx context.Context, tenantID uuid.UUID) (*ports.CashflowProjection, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}

	accounts, err := s.accountRepo.List(ctx, tenantID, ports.AccountFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed fetching accounts for cashflow projection: %w", err)
	}

	availableBalance := decimal.Zero
	var liquidAccounts []*domain.Account
	for _, acc := range accounts {
		if acc.IsLiquidAccount() {
			liquidAccounts = append(liquidAccounts, acc)
			availableBalance = availableBalance.Add(acc.Balance)
		}
	}

	invoices, err := s.invRepo.List(ctx, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed fetching invoices for cashflow projection: %w", err)
	}

	var upcomingBills []*domain.CreditCardInvoice
	for _, inv := range invoices {
		if inv.Status == domain.InvoiceStatusOpen ||
			inv.Status == domain.InvoiceStatusClosed ||
			inv.Status == domain.InvoiceStatusOverdue {
			upcomingBills = append(upcomingBills, inv)
		}
	}

	sort.Slice(upcomingBills, func(i, j int) bool {
		return upcomingBills[i].DueDate.Before(upcomingBills[j].DueDate)
	})

	now := time.Now().UTC()
	cycleHorizon := now.AddDate(0, 1, 5)
	totalPendingBills := decimal.Zero
	includedAny := false

	for _, inv := range upcomingBills {
		if inv.Status == domain.InvoiceStatusClosed ||
			inv.Status == domain.InvoiceStatusOverdue ||
			inv.ClosingDate == nil ||
			!inv.ClosingDate.After(cycleHorizon) {
			totalPendingBills = totalPendingBills.Add(inv.TotalAmount)
			includedAny = true
		}
	}
	if !includedAny && len(upcomingBills) > 0 {
		totalPendingBills = upcomingBills[0].TotalAmount
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

// PayInvoice marks a credit card invoice as paid and ensures the payment is reflected on the target checking account.
func (s *InvoiceService) PayInvoice(
	ctx context.Context,
	tenantID, userID, invoiceID uuid.UUID,
	accountID *uuid.UUID,
	paidAt *time.Time,
) (*domain.CreditCardInvoice, error) {
	if tenantID == uuid.Nil || invoiceID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}

	inv, err := s.invRepo.FindByID(ctx, tenantID, invoiceID)
	if err != nil {
		return nil, err
	}

	checkingAcc, err := s.resolveSettlementLiquidAccount(ctx, tenantID, accountID)
	if err != nil {
		return nil, err
	}

	payDate := inv.DueDate
	if paidAt != nil && !paidAt.IsZero() {
		payDate = *paidAt
	}

	window := domain.DefaultSettlementWindow(&payDate)
	windowStart, windowEnd := window.Bounds(inv.DueDate)

	candidates, _, listErr := s.txRepo.List(ctx, tenantID, ports.TransactionFilter{
		AccountID: &checkingAcc.ID,
		FromDate:  &windowStart,
		ToDate:    &windowEnd,
		Limit:     200,
	})
	if listErr != nil {
		return nil, fmt.Errorf("failed listing candidate payment transactions: %w", listErr)
	}

	match := domain.MatchInvoiceSettlement(
		inv,
		[]*domain.Account{checkingAcc},
		map[uuid.UUID][]*domain.Transaction{checkingAcc.ID: candidates},
		nil,
		window,
	)

	if !match.Matched {
		match, err = s.synthesizeInvoicePayment(ctx, tenantID, userID, inv, checkingAcc, payDate)
		if err != nil {
			return nil, err
		}
	}

	inv.ApplySettlement(match)

	if err := s.invRepo.Save(ctx, inv); err != nil {
		return nil, fmt.Errorf("failed saving paid invoice: %w", err)
	}

	return inv, nil
}

func (s *InvoiceService) resolveSettlementLiquidAccount(
	ctx context.Context,
	tenantID uuid.UUID,
	accountID *uuid.UUID,
) (*domain.Account, error) {
	if accountID != nil && *accountID != uuid.Nil {
		acc, err := s.accountRepo.GetByID(ctx, tenantID, *accountID)
		if err != nil {
			return nil, err
		}
		if acc.Type == domain.AccountTypeCreditCard {
			return nil, domain.ErrInvalidAccountType
		}
		return acc, nil
	}

	accounts, err := s.accountRepo.List(ctx, tenantID, ports.AccountFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed listing accounts: %w", err)
	}
	var fallback *domain.Account
	for _, acc := range accounts {
		if !acc.IsLiquidAccount() {
			continue
		}
		if acc.Type == domain.AccountTypeChecking {
			return acc, nil
		}
		if fallback == nil {
			fallback = acc
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, domain.ErrNotFound
}

func (s *InvoiceService) synthesizeInvoicePayment(
	ctx context.Context,
	tenantID, userID uuid.UUID,
	inv *domain.CreditCardInvoice,
	checkingAcc *domain.Account,
	payDate time.Time,
) (domain.InvoiceSettlementMatch, error) {
	match := domain.InvoiceSettlementMatch{
		Matched:       true,
		LiquidAccount: checkingAcc,
		PaidAt:        payDate,
		PaidAmount:    inv.TotalAmount,
	}
	if !inv.TotalAmount.IsPositive() {
		return match, nil
	}

	cardName := "Cartão de Crédito"
	if cardAcc, cardErr := s.accountRepo.GetByID(ctx, tenantID, inv.AccountID); cardErr == nil && cardAcc != nil {
		cardName = cardAcc.Name
	}

	var createdBy *uuid.UUID
	if userID != uuid.Nil {
		createdBy = &userID
	}

	paymentTx, txErr := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       checkingAcc.ID,
		CreatedByUserID: createdBy,
		TransactedAt:    payDate,
		Date:            payDate,
		Amount:          inv.TotalAmount.Neg(),
		Description:     fmt.Sprintf("Pagamento Fatura %s (%s)", cardName, inv.DueDate.Format("01/2006")),
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      checkingAcc.Visibility,
		Source:          domain.SourceTypeManual,
		Metadata: map[string]any{
			"invoice_id":      inv.ID.String(),
			"is_bill_payment": true,
		},
	})
	if txErr != nil {
		return domain.InvoiceSettlementMatch{}, fmt.Errorf("failed creating invoice payment transaction: %w", txErr)
	}

	if err := s.txRepo.Create(ctx, paymentTx); err != nil {
		return domain.InvoiceSettlementMatch{}, fmt.Errorf("failed persisting invoice payment transaction: %w", err)
	}

	match.LiquidOutflowTx = paymentTx
	match.AutoCreated = true
	return match, nil
}

// UnpayInvoice reverts an invoice back to open status and removes any auto-created manual payment transaction.
func (s *InvoiceService) UnpayInvoice(ctx context.Context, tenantID, invoiceID uuid.UUID) (*domain.CreditCardInvoice, error) {
	if tenantID == uuid.Nil || invoiceID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}

	inv, err := s.invRepo.FindByID(ctx, tenantID, invoiceID)
	if err != nil {
		return nil, err
	}

	if autoTxID := inv.ClearSettlement(); autoTxID != nil {
		existingTx, getErr := s.txRepo.GetByID(ctx, tenantID, *autoTxID)
		if getErr == nil && existingTx != nil && existingTx.Source != domain.SourceTypePluggy {
			if delErr := s.txRepo.Delete(ctx, tenantID, *autoTxID); delErr != nil && !errors.Is(delErr, domain.ErrNotFound) {
				return nil, fmt.Errorf("failed deleting auto-created payment transaction: %w", delErr)
			}
		}
	}

	if err := s.invRepo.Save(ctx, inv); err != nil {
		return nil, fmt.Errorf("failed saving reopened invoice: %w", err)
	}

	return inv, nil
}
