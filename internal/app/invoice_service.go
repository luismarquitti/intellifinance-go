package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
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

func isBillSettlementTransaction(tx *domain.Transaction) bool {
	if tx == nil {
		return false
	}
	if tx.Type == domain.TransactionTypeTransfer {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(tx.Description), "Pagamento recebido") {
		return true
	}
	if tx.Metadata != nil {
		if v, ok := tx.Metadata["is_bill_payment"].(bool); ok && v {
			return true
		}
	}
	return false
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
		if isBillSettlementTransaction(tx) {
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

	var upcomingBills []*domain.CreditCardInvoice
	for _, inv := range invoices {
		// Only consider bills that have not been settled yet (OPEN, CLOSED, OVERDUE)
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

func isCheckingBillPaymentDescription(desc string) bool {
	upper := strings.ToUpper(desc)
	return strings.Contains(upper, "FATURA") ||
		strings.Contains(upper, "ITAU BLACK") ||
		strings.Contains(upper, "ITAUCARD") ||
		strings.Contains(upper, "UNICLASS")
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

	var checkingAcc *domain.Account
	if accountID != nil && *accountID != uuid.Nil {
		acc, getErr := s.accountRepo.GetByID(ctx, tenantID, *accountID)
		if getErr != nil {
			return nil, getErr
		}
		if acc.Type == domain.AccountTypeCreditCard {
			return nil, domain.ErrInvalidAccountType
		}
		checkingAcc = acc
	} else {
		accounts, listErr := s.accountRepo.List(ctx, tenantID, ports.AccountFilter{})
		if listErr != nil {
			return nil, fmt.Errorf("failed listing accounts: %w", listErr)
		}
		for _, acc := range accounts {
			if !acc.IsActive || acc.IsDeleted() {
				continue
			}
			if acc.Type == domain.AccountTypeChecking {
				checkingAcc = acc
				break
			}
		}
		if checkingAcc == nil {
			for _, acc := range accounts {
				if !acc.IsActive || acc.IsDeleted() {
					continue
				}
				if acc.Type == domain.AccountTypeSavings || acc.Type == domain.AccountTypeCash {
					checkingAcc = acc
					break
				}
			}
		}
		if checkingAcc == nil {
			return nil, domain.ErrNotFound
		}
	}

	payDate := inv.DueDate
	if paidAt != nil && !paidAt.IsZero() {
		payDate = *paidAt
	}

	// Search existing checking account transactions around due date / pay date to avoid duplicate debits
	windowStart := inv.DueDate.AddDate(0, 0, -10)
	if payDate.AddDate(0, 0, -10).Before(windowStart) {
		windowStart = payDate.AddDate(0, 0, -10)
	}
	windowEnd := inv.DueDate.AddDate(0, 0, 20)
	if payDate.AddDate(0, 0, 10).After(windowEnd) {
		windowEnd = payDate.AddDate(0, 0, 10)
	}

	candidates, _, listErr := s.txRepo.List(ctx, tenantID, ports.TransactionFilter{
		AccountID: &checkingAcc.ID,
		FromDate:  &windowStart,
		ToDate:    &windowEnd,
		Limit:     200,
	})

	var matchedTx *domain.Transaction
	if listErr == nil {
		negTotal := inv.TotalAmount.Neg()
		for _, tx := range candidates {
			if tx.AccountID != checkingAcc.ID || tx.Status == domain.TransactionStatusVoid || !tx.Amount.IsNegative() {
				continue
			}
			if tx.Date.Before(windowStart) || tx.Date.After(windowEnd) {
				continue
			}
			if tx.Amount.Equal(negTotal) {
				matchedTx = tx
				if isCheckingBillPaymentDescription(tx.Description) {
					break
				}
			}
		}
	}

	if inv.Metadata == nil {
		inv.Metadata = make(map[string]any)
	}

	if matchedTx != nil {
		inv.Metadata["payment_transaction_id"] = matchedTx.ID.String()
		delete(inv.Metadata, "auto_created_payment_tx_id")
		payDate = matchedTx.Date
	} else if inv.TotalAmount.IsPositive() {
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
			return nil, fmt.Errorf("failed creating invoice payment transaction: %w", txErr)
		}

		if err := s.txRepo.Create(ctx, paymentTx); err != nil {
			return nil, fmt.Errorf("failed persisting invoice payment transaction: %w", err)
		}

		inv.Metadata["payment_transaction_id"] = paymentTx.ID.String()
		inv.Metadata["auto_created_payment_tx_id"] = paymentTx.ID.String()
	}

	inv.Metadata["payment_account_id"] = checkingAcc.ID.String()
	inv.Metadata["payment_account_name"] = checkingAcc.Name
	inv.Metadata["paid_at"] = payDate.Format("2006-01-02")
	inv.Metadata["paid_amount"] = inv.TotalAmount.StringFixed(2)
	inv.MarkAsPaid()

	if err := s.invRepo.Save(ctx, inv); err != nil {
		return nil, fmt.Errorf("failed saving paid invoice: %w", err)
	}

	return inv, nil
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

	if inv.Metadata != nil {
		if autoTxIDStr, ok := inv.Metadata["auto_created_payment_tx_id"].(string); ok && autoTxIDStr != "" {
			if autoTxID, parseErr := uuid.Parse(autoTxIDStr); parseErr == nil {
				existingTx, getErr := s.txRepo.GetByID(ctx, tenantID, autoTxID)
				if getErr == nil && existingTx != nil && existingTx.Source != domain.SourceTypePluggy {
					if delErr := s.txRepo.Delete(ctx, tenantID, autoTxID); delErr != nil && !errors.Is(delErr, domain.ErrNotFound) {
						return nil, fmt.Errorf("failed deleting auto-created payment transaction: %w", delErr)
					}
				}
			}
		}

		delete(inv.Metadata, "payment_transaction_id")
		delete(inv.Metadata, "auto_created_payment_tx_id")
		delete(inv.Metadata, "payment_account_id")
		delete(inv.Metadata, "payment_account_name")
		delete(inv.Metadata, "paid_at")
		delete(inv.Metadata, "paid_amount")
	} else {
		inv.Metadata = make(map[string]any)
	}

	inv.MarkAsOpen()
	if err := s.invRepo.Save(ctx, inv); err != nil {
		return nil, fmt.Errorf("failed saving reopened invoice: %w", err)
	}

	return inv, nil
}
