package domain

import (
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ProviderTxInput holds raw external transaction attributes for pure domain normalization.
type ProviderTxInput struct {
	RawAmount    decimal.Decimal
	Description  string
	CategoryHint string
	Date         time.Time
}

// NormalizedProviderTx represents canonical ledger values derived from an external transaction.
type NormalizedProviderTx struct {
	Amount              decimal.Decimal
	Type                TransactionType
	Status              TransactionStatus
	IsInvoiceSettlement bool
	IsFutureScheduled   bool
}

// InvoiceCycleSpec identifies a monthly billing cycle for a credit card Invoice.
type InvoiceCycleSpec struct {
	CycleKey    string
	ClosingDate time.Time
	DueDate     time.Time
}

// CardPaymentObservation represents a provider-side credit card settlement transaction observation.
type CardPaymentObservation struct {
	ExternalBillID string
	Date           time.Time
	Amount         decimal.Decimal
}

// SettlementWindow configures the calendar tolerance when matching Invoice Settlements.
type SettlementWindow struct {
	DaysBeforeDue int
	DaysAfterDue  int
	ReferenceDate *time.Time
}

// Bounds returns the normalized calendar [start, end] window around dueDate, expanded by ReferenceDate when present.
func (w SettlementWindow) Bounds(dueDate time.Time) (time.Time, time.Time) {
	return settlementDateBounds(dueDate, w)
}

// InvoiceSettlementMatch describes a resolved Liquid Account Outflow Leg or card payment observation.
type InvoiceSettlementMatch struct {
	Matched         bool
	LiquidAccount   *Account
	LiquidOutflowTx *Transaction
	PaidAt          time.Time
	PaidAmount      decimal.Decimal
	AutoCreated     bool
}

// DefaultSettlementWindow returns the canonical [-10, +20] day Invoice Settlement tolerance.
func DefaultSettlementWindow(referenceDate *time.Time) SettlementWindow {
	return SettlementWindow{
		DaysBeforeDue: 10,
		DaysAfterDue:  20,
		ReferenceDate: referenceDate,
	}
}

// IsLiquidAccount reports whether an Account is an active, non-deleted Liquid Account
// (CHECKING, SAVINGS, or CASH).
func (a *Account) IsLiquidAccount() bool {
	if a == nil || !a.IsActive || a.IsDeleted() {
		return false
	}
	return a.Type == AccountTypeChecking ||
		a.Type == AccountTypeSavings ||
		a.Type == AccountTypeCash
}

// IsInvoiceSettlement reports whether a Transaction represents a credit card Invoice Settlement leg.
func (t *Transaction) IsInvoiceSettlement() bool {
	if t == nil {
		return false
	}
	if t.Type == TransactionTypeTransfer {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(t.Description), "Pagamento recebido") {
		return true
	}
	if t.Metadata != nil {
		if v, ok := t.Metadata["is_bill_payment"].(bool); ok && v {
			return true
		}
	}
	return false
}

// IsCheckingInvoicePaymentDescription checks whether a Liquid Account description carries
// common Brazilian credit card Invoice payment keywords.
func IsCheckingInvoicePaymentDescription(desc string) bool {
	upper := strings.ToUpper(desc)
	return strings.Contains(upper, "FATURA") ||
		strings.Contains(upper, "ITAU BLACK") ||
		strings.Contains(upper, "ITAUCARD") ||
		strings.Contains(upper, "UNICLASS")
}

// ExtractBrazilCalendarDay returns the 1-based calendar day of t anchored to Brazil time
// (preserving midnight-UTC pure calendar dates), returning fallbackDay if t is zero.
func ExtractBrazilCalendarDay(t time.Time, fallbackDay int) int {
	day, ok := calendarDayUTC(t)
	if !ok {
		return fallbackDay
	}
	return day.Day()
}

// NormalizeProviderTransaction translates external provider sign conventions and Brazil-timezone
// future dates into canonical Transaction values.
func NormalizeProviderTransaction(
	accountType AccountType,
	in ProviderTxInput,
	referenceNow time.Time,
) NormalizedProviderTx {
	if referenceNow.IsZero() {
		referenceNow = time.Now().UTC()
	}

	out := NormalizedProviderTx{
		Amount: in.RawAmount,
		Type:   TransactionTypeExpense,
		Status: TransactionStatusCompleted,
	}

	if accountType == AccountTypeCreditCard {
		if strings.EqualFold(strings.TrimSpace(in.Description), "Pagamento recebido") ||
			strings.EqualFold(strings.TrimSpace(in.CategoryHint), "Credit card payment") {
			out.Type = TransactionTypeTransfer
			out.Amount = in.RawAmount.Abs()
			out.IsInvoiceSettlement = true
		} else if in.RawAmount.IsPositive() {
			out.Type = TransactionTypeExpense
			out.Amount = in.RawAmount.Neg()
		} else {
			out.Type = TransactionTypeIncome
			out.Amount = in.RawAmount.Abs()
		}
		return out
	}

	if in.RawAmount.IsPositive() {
		out.Type = TransactionTypeIncome
	}
	nowDateStr := StandardizeDateToBrazil(referenceNow.In(BrazilLocation))
	if StandardizeDateToBrazil(in.Date) > nowDateStr {
		out.Status = TransactionStatusPending
		out.IsFutureScheduled = true
	}

	return out
}

// ResolveInvoiceCycle routes a credit card Transaction date (anchored to America/Sao_Paulo,
// preserving midnight-UTC pure calendar dates) into its monthly Invoice cycle.
func ResolveInvoiceCycle(txDate time.Time, closingDay, dueDay int) InvoiceCycleSpec {
	if closingDay <= 0 || closingDay > 31 {
		closingDay = 8
	}
	if dueDay <= 0 || dueDay > 31 {
		dueDay = 15
	}

	txDay, ok := calendarDayUTC(txDate)
	if !ok {
		txDay = time.Now().In(BrazilLocation)
	}
	targetMonth := time.Date(txDay.Year(), txDay.Month(), 1, 0, 0, 0, 0, time.UTC)
	if txDay.Day() >= closingDay {
		targetMonth = targetMonth.AddDate(0, 1, 0)
	}

	lastDayOfMonth := targetMonth.AddDate(0, 1, -1).Day()
	cDay := closingDay
	if cDay > lastDayOfMonth {
		cDay = lastDayOfMonth
	}
	dDay := dueDay
	if dDay > lastDayOfMonth {
		dDay = lastDayOfMonth
	}

	return InvoiceCycleSpec{
		CycleKey:    targetMonth.Format("2006-01"),
		ClosingDate: time.Date(targetMonth.Year(), targetMonth.Month(), cDay, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(targetMonth.Year(), targetMonth.Month(), dDay, 0, 0, 0, 0, time.UTC),
	}
}

// ResolveProviderInvoiceStatus maps provider invoice status strings and payment counts to
// InvoiceStatus while preserving an already settled PAID Invoice.
func ResolveProviderInvoiceStatus(
	rawStatus string,
	paymentsCount int,
	existing *CreditCardInvoice,
) InvoiceStatus {
	status := InvoiceStatusOpen
	switch strings.ToUpper(strings.TrimSpace(rawStatus)) {
	case "CLOSED":
		status = InvoiceStatusClosed
	case "PAID":
		status = InvoiceStatusPaid
	case "OVERDUE":
		status = InvoiceStatusOverdue
	}
	if paymentsCount > 0 || (existing != nil && existing.Status == InvoiceStatusPaid) {
		return InvoiceStatusPaid
	}
	return status
}

// MatchInvoiceSettlement finds a matching Liquid Account Outflow Leg or card payment observation
// that settles an Invoice within the configured calendar window.
func MatchInvoiceSettlement(
	inv *CreditCardInvoice,
	liquidAccounts []*Account,
	liquidTxsByAccount map[uuid.UUID][]*Transaction,
	cardPayments []CardPaymentObservation,
	window SettlementWindow,
) InvoiceSettlementMatch {
	if inv == nil || !inv.TotalAmount.IsPositive() {
		return InvoiceSettlementMatch{}
	}

	windowStart, windowEnd := window.Bounds(inv.DueDate)
	negTotal := inv.TotalAmount.Neg()

	var matchedAcc *Account
	var matchedTx *Transaction

	for _, acc := range liquidAccounts {
		if acc == nil {
			continue
		}
		for _, tx := range liquidTxsByAccount[acc.ID] {
			if tx.AccountID != acc.ID || tx.Status == TransactionStatusVoid || !tx.Amount.IsNegative() {
				continue
			}
			txDay, ok := calendarDayUTC(tx.Date)
			if !ok || txDay.Before(windowStart) || txDay.After(windowEnd) {
				continue
			}
			if tx.Amount.Equal(negTotal) {
				matchedAcc = acc
				matchedTx = tx
				if IsCheckingInvoicePaymentDescription(tx.Description) {
					break
				}
			}
		}
		if matchedTx != nil && IsCheckingInvoicePaymentDescription(matchedTx.Description) {
			break
		}
	}

	if matchedTx != nil && matchedAcc != nil {
		return InvoiceSettlementMatch{
			Matched:         true,
			LiquidAccount:   matchedAcc,
			LiquidOutflowTx: matchedTx,
			PaidAt:          matchedTx.Date,
			PaidAmount:      inv.TotalAmount,
		}
	}

	for _, cp := range cardPayments {
		if inv.ExternalID != nil && *inv.ExternalID != "" && cp.ExternalBillID == *inv.ExternalID {
			return InvoiceSettlementMatch{
				Matched:    true,
				PaidAt:     cp.Date,
				PaidAmount: inv.TotalAmount,
			}
		}
		cpDay, ok := calendarDayUTC(cp.Date)
		if !ok || cpDay.Before(windowStart) || cpDay.After(windowEnd) {
			continue
		}
		if cp.Amount.Abs().Equal(inv.TotalAmount) {
			return InvoiceSettlementMatch{
				Matched:    true,
				PaidAt:     cp.Date,
				PaidAmount: inv.TotalAmount,
			}
		}
	}

	return InvoiceSettlementMatch{}
}

// MatchInvoiceForSettlementLeg selects the best matching Invoice for a credit card settlement leg,
// preferring unpaid exact-amount matches, then unpaid in-window matches (closest DueDate), before
// considering already PAID invoices.
func MatchInvoiceForSettlementLeg(
	invoices []*CreditCardInvoice,
	cardAccountID uuid.UUID,
	amount decimal.Decimal,
	date time.Time,
	window SettlementWindow,
) (matched *CreditCardInvoice, hasInvoicesForCard bool) {
	absAmount := amount.Abs()
	txDay, hasDay := calendarDayUTC(date)

	bestRank := math.MaxInt
	bestDistDays := math.MaxFloat64

	for _, inv := range invoices {
		if inv == nil || inv.AccountID != cardAccountID {
			continue
		}
		hasInvoicesForCard = true

		exactAmount := inv.TotalAmount.Equal(absAmount)
		inWindow := false
		distDays := 0.0
		if hasDay {
			start, end := window.Bounds(inv.DueDate)
			inWindow = !txDay.Before(start) && !txDay.After(end)
			if dueDay, dueOK := calendarDayUTC(inv.DueDate); dueOK {
				distDays = math.Abs(txDay.Sub(dueDay).Hours()) / 24.0
			}
		}
		if !exactAmount && !inWindow {
			continue
		}

		rank := settlementLegMatchRank(inv.Status != InvoiceStatusPaid, exactAmount, inWindow)
		if rank < bestRank || (rank == bestRank && distDays < bestDistDays) {
			bestRank = rank
			bestDistDays = distDays
			matched = inv
		}
	}

	return matched, hasInvoicesForCard
}

func settlementLegMatchRank(unpaid, exactAmount, inWindow bool) int {
	switch {
	case unpaid && exactAmount && inWindow:
		return 0
	case unpaid && exactAmount:
		return 1
	case unpaid && inWindow:
		return 2
	case !unpaid && exactAmount && inWindow:
		return 3
	case !unpaid && exactAmount:
		return 4
	default:
		return 5
	}
}

func settlementDateBounds(dueDate time.Time, window SettlementWindow) (time.Time, time.Time) {
	baseDue, ok := calendarDayUTC(dueDate)
	if !ok {
		baseDue = time.Now().UTC()
	}
	start := baseDue.AddDate(0, 0, -window.DaysBeforeDue)
	end := baseDue.AddDate(0, 0, window.DaysAfterDue)

	if window.ReferenceDate != nil && !window.ReferenceDate.IsZero() {
		if refDay, refOK := calendarDayUTC(*window.ReferenceDate); refOK {
			refStart := refDay.AddDate(0, 0, -window.DaysBeforeDue)
			if refStart.Before(start) {
				start = refStart
			}
			refEnd := refDay.AddDate(0, 0, 10)
			if refEnd.After(end) {
				end = refEnd
			}
		}
	}
	return start, end
}

func calendarDayUTC(t time.Time) (time.Time, bool) {
	s := StandardizeDateToBrazil(t)
	if s == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// ApplySettlement stamps Invoice Settlement metadata onto the Invoice and marks it PAID.
func (i *CreditCardInvoice) ApplySettlement(match InvoiceSettlementMatch) {
	if i == nil {
		return
	}
	if i.Metadata == nil {
		i.Metadata = make(map[string]any)
	}
	if match.LiquidOutflowTx != nil {
		i.Metadata["payment_transaction_id"] = match.LiquidOutflowTx.ID.String()
		if match.AutoCreated {
			i.Metadata["auto_created_payment_tx_id"] = match.LiquidOutflowTx.ID.String()
		} else {
			delete(i.Metadata, "auto_created_payment_tx_id")
		}
	} else {
		delete(i.Metadata, "auto_created_payment_tx_id")
	}
	if match.LiquidAccount != nil {
		i.Metadata["payment_account_id"] = match.LiquidAccount.ID.String()
		i.Metadata["payment_account_name"] = match.LiquidAccount.Name
	}
	if !match.PaidAt.IsZero() {
		i.Metadata["paid_at"] = StandardizeDateToBrazil(match.PaidAt)
	}
	i.Metadata["paid_amount"] = i.TotalAmount.StringFixed(2)
	i.MarkAsPaid()
}

// ClearSettlement removes Invoice Settlement metadata, reopens the Invoice, and returns
// any auto-created payment transaction UUID that the caller should delete.
func (i *CreditCardInvoice) ClearSettlement() *uuid.UUID {
	if i == nil {
		return nil
	}
	var autoTxID *uuid.UUID
	if i.Metadata != nil {
		if rawID, ok := i.Metadata["auto_created_payment_tx_id"].(string); ok && rawID != "" {
			if parsed, err := uuid.Parse(rawID); err == nil {
				autoTxID = &parsed
			}
		}
		delete(i.Metadata, "payment_transaction_id")
		delete(i.Metadata, "auto_created_payment_tx_id")
		delete(i.Metadata, "payment_account_id")
		delete(i.Metadata, "payment_account_name")
		delete(i.Metadata, "paid_at")
		delete(i.Metadata, "paid_amount")
	} else {
		i.Metadata = make(map[string]any)
	}
	i.MarkAsOpen()
	return autoTxID
}

// ComputeLiquidAccountAnchorDelta calculates the initial balance adjustment required to anchor
// a newly synced Liquid Account's Ledger Balance to the Provider Balance Snapshot after offsetting
// post-today scheduled transactions.
func ComputeLiquidAccountAnchorDelta(
	ledgerBalance decimal.Decimal,
	providerBalance decimal.Decimal,
	txs []ProviderTxInput,
	referenceNow time.Time,
) decimal.Decimal {
	if referenceNow.IsZero() {
		referenceNow = time.Now().UTC()
	}
	nowDateStr := StandardizeDateToBrazil(referenceNow.In(BrazilLocation))
	futureDelta := decimal.Zero
	for _, tx := range txs {
		if StandardizeDateToBrazil(tx.Date) > nowDateStr {
			futureDelta = futureDelta.Add(tx.RawAmount)
		}
	}
	targetBal := providerBalance.Add(futureDelta).Round(2)
	return targetBal.Sub(ledgerBalance)
}

// ComputeCreditCardLiabilityDelta calculates the adjustment required so a credit card Account's
// Ledger Balance equals the negative sum of its unpaid Invoices.
func ComputeCreditCardLiabilityDelta(
	ledgerBalance decimal.Decimal,
	invoices []*CreditCardInvoice,
) decimal.Decimal {
	unpaidSum := decimal.Zero
	for _, inv := range invoices {
		if inv != nil && inv.Status != InvoiceStatusPaid {
			unpaidSum = unpaidSum.Add(inv.TotalAmount)
		}
	}
	targetCardBal := unpaidSum.Neg().Round(2)
	return targetCardBal.Sub(ledgerBalance)
}
