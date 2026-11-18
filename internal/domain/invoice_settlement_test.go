package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccount_IsLiquidAccount(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	cases := []struct {
		name     string
		accType  domain.AccountType
		inactive bool
		deleted  bool
		want     bool
	}{
		{"active checking", domain.AccountTypeChecking, false, false, true},
		{"active savings", domain.AccountTypeSavings, false, false, true},
		{"active cash", domain.AccountTypeCash, false, false, true},
		{"credit card excluded", domain.AccountTypeCreditCard, false, false, false},
		{"investment excluded", domain.AccountTypeInvestment, false, false, false},
		{"inactive checking excluded", domain.AccountTypeChecking, true, false, false},
		{"soft-deleted checking excluded", domain.AccountTypeChecking, false, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acc, err := domain.NewAccount(tenantID, userID, tc.name, tc.accType, "BRL", domain.VisibilityShared)
			require.NoError(t, err)
			if tc.inactive {
				acc.IsActive = false
			}
			if tc.deleted {
				acc.SoftDelete()
			}
			assert.Equal(t, tc.want, acc.IsLiquidAccount())
		})
	}
	assert.False(t, (*domain.Account)(nil).IsLiquidAccount())
}

func TestNormalizeProviderTransaction(t *testing.T) {
	refNow := time.Date(2026, 11, 17, 15, 0, 0, 0, domain.BrazilLocation)

	cases := []struct {
		name           string
		accType        domain.AccountType
		in             domain.ProviderTxInput
		wantAmount     string
		wantType       domain.TransactionType
		wantStatus     domain.TransactionStatus
		wantSettlement bool
		wantFuture     bool
	}{
		{
			name:    "credit card positive purchase becomes negative expense",
			accType: domain.AccountTypeCreditCard,
			in: domain.ProviderTxInput{
				RawAmount:   decimal.RequireFromString("125.40"),
				Description: "Supermercado",
				Date:        time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
			},
			wantAmount: "-125.40",
			wantType:   domain.TransactionTypeExpense,
			wantStatus: domain.TransactionStatusCompleted,
		},
		{
			name:    "credit card negative refund becomes positive income",
			accType: domain.AccountTypeCreditCard,
			in: domain.ProviderTxInput{
				RawAmount:   decimal.RequireFromString("-42.10"),
				Description: "Estorno Loja",
				Date:        time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
			},
			wantAmount: "42.10",
			wantType:   domain.TransactionTypeIncome,
			wantStatus: domain.TransactionStatusCompleted,
		},
		{
			name:    "credit card bill payment becomes positive transfer settlement",
			accType: domain.AccountTypeCreditCard,
			in: domain.ProviderTxInput{
				RawAmount:    decimal.RequireFromString("-1500.00"),
				Description:  "Pagamento recebido",
				CategoryHint: "Credit card payment",
				Date:         time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC),
			},
			wantAmount:     "1500.00",
			wantType:       domain.TransactionTypeTransfer,
			wantStatus:     domain.TransactionStatusCompleted,
			wantSettlement: true,
		},
		{
			name:    "checking future transaction is pending in Brazil time",
			accType: domain.AccountTypeChecking,
			in: domain.ProviderTxInput{
				RawAmount:   decimal.RequireFromString("-200.00"),
				Description: "Agendamento Aluguel",
				Date:        time.Date(2026, 11, 18, 12, 0, 0, 0, domain.BrazilLocation),
			},
			wantAmount: "-200.00",
			wantType:   domain.TransactionTypeExpense,
			wantStatus: domain.TransactionStatusPending,
			wantFuture: true,
		},
		{
			name:    "checking same Brazil calendar day at late UTC is completed",
			accType: domain.AccountTypeChecking,
			in: domain.ProviderTxInput{
				RawAmount:   decimal.RequireFromString("5000.00"),
				Description: "Salario",
				Date:        time.Date(2026, 11, 18, 1, 30, 0, 0, time.UTC), // 2026-11-17 22:30 BRT
			},
			wantAmount: "5000.00",
			wantType:   domain.TransactionTypeIncome,
			wantStatus: domain.TransactionStatusCompleted,
			wantFuture: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.NormalizeProviderTransaction(tc.accType, tc.in, refNow)
			assert.Equal(t, tc.wantAmount, got.Amount.StringFixed(2))
			assert.Equal(t, tc.wantType, got.Type)
			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, tc.wantSettlement, got.IsInvoiceSettlement)
			assert.Equal(t, tc.wantFuture, got.IsFutureScheduled)
		})
	}
}

func TestResolveInvoiceCycle(t *testing.T) {
	cases := []struct {
		name        string
		txDate      time.Time
		closingDay  int
		dueDay      int
		wantKey     string
		wantDue     string
		wantClosing string
	}{
		{
			name:        "before closing stays in current month",
			txDate:      time.Date(2026, 11, 7, 14, 0, 0, 0, domain.BrazilLocation),
			closingDay:  8,
			dueDay:      15,
			wantKey:     "2026-11",
			wantClosing: "2026-11-08",
			wantDue:     "2026-11-15",
		},
		{
			name:        "on closing rolls to next month",
			txDate:      time.Date(2026, 11, 8, 10, 0, 0, 0, domain.BrazilLocation),
			closingDay:  8,
			dueDay:      15,
			wantKey:     "2026-12",
			wantClosing: "2026-12-08",
			wantDue:     "2026-12-15",
		},
		{
			name:        "midnight UTC on closing day preserves calendar day and rolls to next month",
			txDate:      time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC),
			closingDay:  8,
			dueDay:      15,
			wantKey:     "2026-12",
			wantClosing: "2026-12-08",
			wantDue:     "2026-12-15",
		},
		{
			name:        "clamps day 31 to February 28 in non-leap year",
			txDate:      time.Date(2026, 1, 31, 10, 0, 0, 0, domain.BrazilLocation),
			closingDay:  31,
			dueDay:      31,
			wantKey:     "2026-02",
			wantClosing: "2026-02-28",
			wantDue:     "2026-02-28",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := domain.ResolveInvoiceCycle(tc.txDate, tc.closingDay, tc.dueDay)
			assert.Equal(t, tc.wantKey, spec.CycleKey)
			assert.Equal(t, tc.wantClosing, spec.ClosingDate.Format("2006-01-02"))
			assert.Equal(t, tc.wantDue, spec.DueDate.Format("2006-01-02"))
		})
	}
}

func TestMatchInvoiceSettlement_AndMetadataLifecycle(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	chkAcc, err := domain.NewAccount(tenantID, userID, "Itaú Corrente", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	ccAcc, err := domain.NewAccount(tenantID, userID, "Itaú Black", domain.AccountTypeCreditCard, "BRL", domain.VisibilityShared)
	require.NoError(t, err)

	dueDate := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	extID := "bill-nov-2026"
	inv, err := domain.NewCreditCardInvoice(tenantID, ccAcc.ID, dueDate, decimal.RequireFromString("1200.00"), domain.InvoiceStatusClosed)
	require.NoError(t, err)
	inv.ExternalID = &extID

	genericDebit, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   chkAcc.ID,
		Date:        dueDate,
		Amount:      decimal.RequireFromString("-1200.00"),
		Description: "Transferencia generica",
		Type:        domain.TransactionTypeExpense,
	})
	require.NoError(t, err)

	preferredDebit, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   chkAcc.ID,
		Date:        dueDate.AddDate(0, 0, 2),
		Amount:      decimal.RequireFromString("-1200.00"),
		Description: "Pagamento Fatura Itaucard Black",
		Type:        domain.TransactionTypeExpense,
	})
	require.NoError(t, err)

	outOfWindowDebit, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   chkAcc.ID,
		Date:        dueDate.AddDate(0, 0, 25),
		Amount:      decimal.RequireFromString("-1200.00"),
		Description: "Pagamento Fatura Itaucard Black",
		Type:        domain.TransactionTypeExpense,
	})
	require.NoError(t, err)

	// Prefers keyword match over generic same-amount debit inside window
	match := domain.MatchInvoiceSettlement(
		inv,
		[]*domain.Account{chkAcc},
		map[uuid.UUID][]*domain.Transaction{chkAcc.ID: {genericDebit, preferredDebit}},
		nil,
		domain.DefaultSettlementWindow(nil),
	)
	require.True(t, match.Matched)
	require.NotNil(t, match.LiquidOutflowTx)
	assert.Equal(t, preferredDebit.ID, match.LiquidOutflowTx.ID)
	assert.Equal(t, chkAcc.ID, match.LiquidAccount.ID)

	inv.ApplySettlement(match)
	assert.Equal(t, domain.InvoiceStatusPaid, inv.Status)
	assert.Equal(t, preferredDebit.ID.String(), inv.Metadata["payment_transaction_id"])
	assert.Equal(t, "1200.00", inv.Metadata["paid_amount"])

	// Out-of-window debit alone does not match, falls back to card payment with matching ExternalBillID
	fallbackMatch := domain.MatchInvoiceSettlement(
		inv,
		[]*domain.Account{chkAcc},
		map[uuid.UUID][]*domain.Transaction{chkAcc.ID: {outOfWindowDebit}},
		[]domain.CardPaymentObservation{{
			ExternalBillID: extID,
			Date:           dueDate.AddDate(0, 0, 1),
			Amount:         decimal.RequireFromString("-1200.00"),
		}},
		domain.DefaultSettlementWindow(nil),
	)
	require.True(t, fallbackMatch.Matched)
	assert.Nil(t, fallbackMatch.LiquidOutflowTx)
	assert.Equal(t, "2026-11-16", fallbackMatch.PaidAt.Format("2006-01-02"))

	// ClearSettlement removes keys and restores open status
	autoTxID := uuid.New()
	inv.Metadata["auto_created_payment_tx_id"] = autoTxID.String()
	clearedAutoID := inv.ClearSettlement()
	require.NotNil(t, clearedAutoID)
	assert.Equal(t, autoTxID, *clearedAutoID)
	assert.Equal(t, domain.InvoiceStatusOpen, inv.Status)
	assert.NotContains(t, inv.Metadata, "payment_transaction_id")
	assert.NotContains(t, inv.Metadata, "auto_created_payment_tx_id")
}

func TestBalanceAnchorAndLiabilityDeltas(t *testing.T) {
	refNow := time.Date(2026, 11, 17, 12, 0, 0, 0, domain.BrazilLocation)
	rawTxs := []domain.ProviderTxInput{
		{RawAmount: decimal.RequireFromString("-100.00"), Date: time.Date(2026, 11, 15, 12, 0, 0, 0, domain.BrazilLocation)},
		{RawAmount: decimal.RequireFromString("-250.00"), Date: time.Date(2026, 11, 20, 12, 0, 0, 0, domain.BrazilLocation)}, // future
	}
	// Provider reports 750.00 (already reflecting future -250.00), while ledger currently holds -100.00 (completed only) -> target 500.00 -> delta +600.00
	delta := domain.ComputeLiquidAccountAnchorDelta(
		decimal.RequireFromString("-100.00"),
		decimal.RequireFromString("750.00"),
		rawTxs,
		refNow,
	)
	assert.Equal(t, "600.00", delta.StringFixed(2))

	tenantID := uuid.New()
	ccID := uuid.New()
	invOpen, _ := domain.NewCreditCardInvoice(tenantID, ccID, refNow, decimal.RequireFromString("300.00"), domain.InvoiceStatusOpen)
	invPaid, _ := domain.NewCreditCardInvoice(tenantID, ccID, refNow.AddDate(0, -1, 0), decimal.RequireFromString("900.00"), domain.InvoiceStatusPaid)
	liabDelta := domain.ComputeCreditCardLiabilityDelta(decimal.RequireFromString("-100.00"), []*domain.CreditCardInvoice{invOpen, invPaid})
	assert.Equal(t, "-200.00", liabDelta.StringFixed(2))
}

func TestResolveProviderInvoiceStatus_AndIsInvoiceSettlement(t *testing.T) {
	tenantID := uuid.New()
	accID := uuid.New()
	existingPaid, _ := domain.NewCreditCardInvoice(tenantID, accID, time.Now(), decimal.NewFromInt(100), domain.InvoiceStatusPaid)
	existingOpen, _ := domain.NewCreditCardInvoice(tenantID, accID, time.Now(), decimal.NewFromInt(100), domain.InvoiceStatusOpen)

	assert.Equal(t, domain.InvoiceStatusClosed, domain.ResolveProviderInvoiceStatus("CLOSED", 0, existingOpen))
	assert.Equal(t, domain.InvoiceStatusOverdue, domain.ResolveProviderInvoiceStatus("OVERDUE", 0, nil))
	assert.Equal(t, domain.InvoiceStatusPaid, domain.ResolveProviderInvoiceStatus("OPEN", 1, existingOpen))
	assert.Equal(t, domain.InvoiceStatusPaid, domain.ResolveProviderInvoiceStatus("OPEN", 0, existingPaid))

	transferTx, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: accID, Amount: decimal.NewFromInt(50), Description: "Anything", Type: domain.TransactionTypeTransfer,
	})
	assert.True(t, transferTx.IsInvoiceSettlement())

	metaTx, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: accID, Amount: decimal.NewFromInt(-50), Description: "Debito", Type: domain.TransactionTypeExpense,
		Metadata: map[string]any{"is_bill_payment": true},
	})
	assert.True(t, metaTx.IsInvoiceSettlement())

	purchaseTx, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID: tenantID, AccountID: accID, Amount: decimal.NewFromInt(-50), Description: "Padaria", Type: domain.TransactionTypeExpense,
	})
	assert.False(t, purchaseTx.IsInvoiceSettlement())
	assert.False(t, (*domain.Transaction)(nil).IsInvoiceSettlement())

	matchedInv, hasCard := domain.MatchInvoiceForSettlementLeg(
		[]*domain.CreditCardInvoice{existingPaid},
		accID,
		decimal.NewFromInt(100),
		time.Now(),
		domain.DefaultSettlementWindow(nil),
	)
	assert.True(t, hasCard)
	require.NotNil(t, matchedInv)
	assert.Equal(t, existingPaid.ID, matchedInv.ID)

	// Overlapping newer OPEN invoice (due Dec 1, R$ 300) must NOT preempt older unpaid exact-amount invoice (due Nov 15, R$ 1500)
	invDec, _ := domain.NewCreditCardInvoice(tenantID, accID, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), decimal.NewFromInt(300), domain.InvoiceStatusOpen)
	invNovUnpaid, _ := domain.NewCreditCardInvoice(tenantID, accID, time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), decimal.NewFromInt(1500), domain.InvoiceStatusClosed)
	invOctPaid, _ := domain.NewCreditCardInvoice(tenantID, accID, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), decimal.NewFromInt(1500), domain.InvoiceStatusPaid)

	best, hasAny := domain.MatchInvoiceForSettlementLeg(
		[]*domain.CreditCardInvoice{invDec, invOctPaid, invNovUnpaid},
		accID,
		decimal.NewFromInt(1500),
		time.Date(2026, 11, 22, 12, 0, 0, 0, domain.BrazilLocation),
		domain.DefaultSettlementWindow(nil),
	)
	assert.True(t, hasAny)
	require.NotNil(t, best)
	assert.Equal(t, invNovUnpaid.ID, best.ID)
	assert.Equal(t, 8, domain.ExtractBrazilCalendarDay(time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC), 1))
}
