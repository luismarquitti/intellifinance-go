package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

//nolint:misspell // Brazilian Portuguese bank description keyword: "Debito Automatico"
func TestCleanDescription(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "COMPRA CARTAO - PÃO DE AÇÚCAR 12/03",
			expected: "PÃO DE AÇÚCAR",
		},
		{
			input:    "Pix enviado - 25/12/2025: Restaurante Silva",
			expected: "RESTAURANTE SILVA",
		},
		{
			input:    "PAGAMENTO BOLETO: ENEL SP #12345",
			expected: "ENEL SP 12345",
		},
		{
			input:    "Debito Automatico NETFLIX COM BR",
			expected: "NETFLIX COM BR",
		},
		{
			input:    "TED RECEBIDA - CLIENTE CONSULTORIA",
			expected: "CLIENTE CONSULTORIA",
		},
		{
			input:    "UBER *TRIP 14/05/26 SP",
			expected: "UBER TRIP SP",
		},
		{
			input:    "   MERCADO   LIVRE   ",
			expected: "MERCADO LIVRE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := domain.CleanDescription(tc.input)
			if got != tc.expected {
				t.Errorf("CleanDescription(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestComputeFingerprint(t *testing.T) {
	tenantID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	accountID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	date := time.Date(2026, 9, 24, 15, 30, 0, 0, time.UTC)
	amount := decimal.NewFromFloat(-154.30)
	cleanDesc := "PAO DE ACUCAR"
	seq := 0

	fp1 := domain.ComputeFingerprint(tenantID, accountID, date, amount, cleanDesc, seq)
	fp2 := domain.ComputeFingerprint(tenantID, accountID, date, amount, cleanDesc, seq)

	// Deterministic
	if fp1 != fp2 {
		t.Errorf("expected deterministic fingerprints, got %s and %s", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Errorf("expected 64-character sha256 hex string, got len %d (%s)", len(fp1), fp1)
	}

	// Changing tenantID alters fingerprint
	otherTenantID := uuid.MustParse("99999999-bbbb-cccc-dddd-eeeeeeeeeeee")
	fpDiffTenant := domain.ComputeFingerprint(otherTenantID, accountID, date, amount, cleanDesc, seq)
	if fp1 == fpDiffTenant {
		t.Error("fingerprint should change when tenantID changes")
	}

	// Changing amount alters fingerprint
	diffAmount := decimal.NewFromFloat(-154.31)
	fpDiffAmount := domain.ComputeFingerprint(tenantID, accountID, date, diffAmount, cleanDesc, seq)
	if fp1 == fpDiffAmount {
		t.Error("fingerprint should change when amount changes")
	}

	// Changing sequence index alters fingerprint (disambiguation for repeat same-day purchases)
	fpDiffSeq := domain.ComputeFingerprint(tenantID, accountID, date, amount, cleanDesc, 1)
	if fp1 == fpDiffSeq {
		t.Error("fingerprint should change when sequence index changes")
	}
}

func TestGenerateTransactionFingerprint_DeterminismAndSensitivity(t *testing.T) {
	tenantID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	accountID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	date := time.Date(2026, 9, 24, 15, 30, 0, 0, time.UTC)
	amount := decimal.NewFromFloat(-154.30)
	cleanDesc := "PAO DE ACUCAR"
	extID := "FITID-987654"
	seq := 0

	fp1 := domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, cleanDesc, extID, seq)
	fp2 := domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, cleanDesc, extID, seq)

	if fp1 != fp2 {
		t.Errorf("expected deterministic fingerprints, got %s and %s", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Errorf("expected 64-character sha256 hex string, got len %d (%s)", len(fp1), fp1)
	}

	// Sensitivity checks
	otherTenant := uuid.MustParse("99999999-bbbb-cccc-dddd-eeeeeeeeeeee")
	if fp1 == domain.GenerateTransactionFingerprint(otherTenant, accountID, date, amount, cleanDesc, extID, seq) {
		t.Error("fingerprint should change when tenant changes")
	}

	otherAccount := uuid.MustParse("22222222-2222-3333-4444-555555555555")
	if fp1 == domain.GenerateTransactionFingerprint(tenantID, otherAccount, date, amount, cleanDesc, extID, seq) {
		t.Error("fingerprint should change when account changes")
	}

	diffDate := time.Date(2026, 9, 25, 15, 30, 0, 0, time.UTC)
	if fp1 == domain.GenerateTransactionFingerprint(tenantID, accountID, diffDate, amount, cleanDesc, extID, seq) {
		t.Error("fingerprint should change when date changes")
	}

	diffAmount := decimal.NewFromFloat(-154.31)
	if fp1 == domain.GenerateTransactionFingerprint(tenantID, accountID, date, diffAmount, cleanDesc, extID, seq) {
		t.Error("fingerprint should change when amount changes")
	}

	diffDesc := "EXTRA SUPERMERCADOS"
	if fp1 == domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, diffDesc, extID, seq) {
		t.Error("fingerprint should change when description changes")
	}

	diffExtID := "FITID-000000"
	if fp1 == domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, cleanDesc, diffExtID, seq) {
		t.Error("fingerprint should change when externalID changes")
	}

	if fp1 == domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, cleanDesc, "", seq) {
		t.Error("fingerprint should change when externalID is omitted")
	}

	if fp1 == domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, cleanDesc, extID, 1) {
		t.Error("fingerprint should change when sequence index changes")
	}
}

func TestGenerateTransactionFingerprint_TimezoneImmunity(t *testing.T) {
	tenantID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	accountID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	amount := decimal.NewFromFloat(-50.00)
	cleanDesc := "UBER TRIP"
	extID := "UBER-123"
	seq := 0

	// 1. Same physical instant (22:30 in Brazil == 01:30 UTC next day)
	// Both must evaluate to 2026-10-14 in Brazil official time
	tBRT := time.Date(2026, 10, 14, 22, 30, 0, 0, domain.BrazilLocation)
	tUTC := tBRT.UTC() // 2026-10-15 01:30:00 UTC

	fpBRT := domain.GenerateTransactionFingerprint(tenantID, accountID, tBRT, amount, cleanDesc, extID, seq)
	fpUTC := domain.GenerateTransactionFingerprint(tenantID, accountID, tUTC, amount, cleanDesc, extID, seq)

	if fpBRT != fpUTC {
		t.Errorf("timezone immunity failure: fpBRT (%s) != fpUTC (%s) for same instant", fpBRT, fpUTC)
	}

	// 2. Pure calendar date (midnight UTC vs midnight Brazil time)
	dateOnlyUTC := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	dateOnlyBRT := time.Date(2026, 10, 14, 0, 0, 0, 0, domain.BrazilLocation)

	fpDateUTC := domain.GenerateTransactionFingerprint(tenantID, accountID, dateOnlyUTC, amount, cleanDesc, extID, seq)
	fpDateBRT := domain.GenerateTransactionFingerprint(tenantID, accountID, dateOnlyBRT, amount, cleanDesc, extID, seq)

	if fpDateUTC != fpDateBRT {
		t.Errorf("calendar date timezone immunity failure: fpDateUTC (%s) != fpDateBRT (%s)", fpDateUTC, fpDateBRT)
	}

	// 3. Foreign timezone (e.g., Tokyo UTC+9)
	tokyoLoc := time.FixedZone("JST", 9*3600)
	tTokyo := tBRT.In(tokyoLoc)
	fpTokyo := domain.GenerateTransactionFingerprint(tenantID, accountID, tTokyo, amount, cleanDesc, extID, seq)

	if fpTokyo != fpBRT {
		t.Errorf("Tokyo timezone immunity failure: fpTokyo (%s) != fpBRT (%s)", fpTokyo, fpBRT)
	}
}

func TestGenerateTransactionFingerprint_WhitespaceImmunity(t *testing.T) {
	tenantID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	accountID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	date := time.Date(2026, 9, 24, 12, 0, 0, 0, domain.BrazilLocation)
	amount := decimal.NewFromFloat(-100.00)
	seq := 0

	// Description with different whitespace
	fp1 := domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, "POSTO IPIRANGA", "FITID-1", seq)
	fp2 := domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, "  POSTO    IPIRANGA  ", "FITID-1", seq)
	fp3 := domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, "posto ipiranga", "FITID-1", seq)

	if fp1 != fp2 {
		t.Errorf("whitespace immunity failure: fp1 (%s) != fp2 (%s)", fp1, fp2)
	}
	if fp1 != fp3 {
		t.Errorf("case immunity failure: fp1 (%s) != fp3 (%s)", fp1, fp3)
	}

	// External ID whitespace trimming
	fpExt1 := domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, "POSTO IPIRANGA", "FITID-1", seq)
	fpExt2 := domain.GenerateTransactionFingerprint(tenantID, accountID, date, amount, "POSTO IPIRANGA", "  FITID-1  ", seq)

	if fpExt1 != fpExt2 {
		t.Errorf("external ID whitespace immunity failure: fpExt1 (%s) != fpExt2 (%s)", fpExt1, fpExt2)
	}
}

func TestNewTransaction_Success(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()
	catID := uuid.New()

	now := time.Now().UTC()
	expenseAmount := decimal.NewFromFloat(-45.50)

	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       accountID,
		CategoryID:      &catID,
		CreatedByUserID: &userID,
		TransactedAt:    now,
		Date:            now,
		Amount:          expenseAmount,
		Description:     "COMPRA CARTAO - Padaria Santo Antonio",
		Type:            domain.TransactionTypeExpense,
		Status:          "", // Defaults to COMPLETED
		Visibility:      "", // Defaults to SHARED
		Source:          "", // Defaults to MANUAL
		ExternalID:      "FITID-12345",
		SequenceIndex:   0,
		Notes:           "Café da manhã",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tx.ID == uuid.Nil {
		t.Error("expected valid transaction UUID")
	}
	if tx.CleanDescription != "PADARIA SANTO ANTONIO" {
		t.Errorf("expected clean description %q, got %q", "PADARIA SANTO ANTONIO", tx.CleanDescription)
	}
	if tx.Fingerprint == "" {
		t.Error("expected computed fingerprint")
	}
	if tx.Status != domain.TransactionStatusCompleted {
		t.Errorf("expected default status COMPLETED, got %s", tx.Status)
	}
	if tx.Visibility != domain.VisibilityShared {
		t.Errorf("expected default visibility SHARED, got %s", tx.Visibility)
	}
	if tx.Source != domain.SourceTypeManual {
		t.Errorf("expected default source MANUAL, got %s", tx.Source)
	}
	if !tx.IsSettled() {
		t.Error("expected completed transaction to be settled")
	}

	// Test Income
	incomeAmount := decimal.NewFromFloat(5000.00)
	txIncome, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   accountID,
		Amount:      incomeAmount,
		Description: "Salário Multinacional",
		Type:        domain.TransactionTypeIncome,
		Status:      domain.TransactionStatusCompleted,
	})
	if err != nil {
		t.Fatalf("unexpected error on income transaction: %v", err)
	}
	if !txIncome.Amount.Equal(incomeAmount) {
		t.Errorf("expected amount %v, got %v", incomeAmount, txIncome.Amount)
	}
}

func TestNewTransaction_ValidationErrors(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	now := time.Now().UTC()

	tests := []struct {
		name        string
		params      domain.NewTransactionParams
		expectedErr error
	}{
		{
			name: "Nil tenant ID",
			params: domain.NewTransactionParams{
				TenantID:    uuid.Nil,
				AccountID:   accountID,
				Amount:      decimal.NewFromFloat(-50.00),
				Description: "Lunch",
				Type:        domain.TransactionTypeExpense,
			},
			expectedErr: domain.ErrInvalidID,
		},
		{
			name: "Nil account ID",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   uuid.Nil,
				Amount:      decimal.NewFromFloat(-50.00),
				Description: "Lunch",
				Type:        domain.TransactionTypeExpense,
			},
			expectedErr: domain.ErrInvalidID,
		},
		{
			name: "Empty description",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   accountID,
				Amount:      decimal.NewFromFloat(-50.00),
				Description: "   ",
				Type:        domain.TransactionTypeExpense,
			},
			expectedErr: domain.ErrEmptyDescription,
		},
		{
			name: "Zero amount",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   accountID,
				Amount:      decimal.Zero,
				Description: "Lunch",
				Type:        domain.TransactionTypeExpense,
			},
			expectedErr: domain.ErrInvalidAmount,
		},
		{
			name: "Expense with positive amount",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   accountID,
				Amount:      decimal.NewFromFloat(50.00), // Positive!
				Description: "Lunch",
				Type:        domain.TransactionTypeExpense,
			},
			expectedErr: domain.ErrInvalidAmount,
		},
		{
			name: "Income with negative amount",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   accountID,
				Amount:      decimal.NewFromFloat(-50.00), // Negative!
				Description: "Salary",
				Type:        domain.TransactionTypeIncome,
			},
			expectedErr: domain.ErrInvalidAmount,
		},
		{
			name: "Invalid transaction type",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   accountID,
				Amount:      decimal.NewFromFloat(-50.00),
				Description: "Lunch",
				Type:        domain.TransactionType("REFUND"),
			},
			expectedErr: domain.ErrInvalidTransactionType,
		},
		{
			name: "Invalid status",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   accountID,
				Amount:      decimal.NewFromFloat(-50.00),
				Description: "Lunch",
				Type:        domain.TransactionTypeExpense,
				Status:      domain.TransactionStatus("EXPIRED"),
			},
			expectedErr: domain.ErrInvalidStatus,
		},
		{
			name: "Invalid visibility",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   accountID,
				Amount:      decimal.NewFromFloat(-50.00),
				Description: "Lunch",
				Type:        domain.TransactionTypeExpense,
				Visibility:  domain.Visibility("CONFIDENTIAL"),
			},
			expectedErr: domain.ErrInvalidVisibility,
		},
		{
			name: "Invalid source",
			params: domain.NewTransactionParams{
				TenantID:    tenantID,
				AccountID:   accountID,
				Amount:      decimal.NewFromFloat(-50.00),
				Description: "Lunch",
				Type:        domain.TransactionTypeExpense,
				Source:      domain.SourceType("TELEGRAM"),
			},
			expectedErr: domain.ErrInvalidSource,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.params.TransactedAt.IsZero() {
				tc.params.TransactedAt = now
			}
			_, err := domain.NewTransaction(tc.params)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestTransaction_Validate_EdgeCases(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	now := time.Now().UTC()

	// 1. Valid transfer transaction
	transferTx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   accountID,
		Amount:      decimal.NewFromFloat(250.00),
		Description: "Transfer to Savings",
		Type:        domain.TransactionTypeTransfer,
		Status:      domain.TransactionStatusCompleted,
	})
	if err != nil {
		t.Fatalf("unexpected error on valid transfer: %v", err)
	}
	if transferTx.Type != domain.TransactionTypeTransfer {
		t.Errorf("expected TRANSFER type, got %s", transferTx.Type)
	}

	// 2. Transfer with zero amount should fail
	transferZero := &domain.Transaction{
		ID:          uuid.New(),
		TenantID:    tenantID,
		AccountID:   accountID,
		Description: "Transfer zero",
		Date:        now,
		Amount:      decimal.Zero,
		Type:        domain.TransactionTypeTransfer,
		Status:      domain.TransactionStatusCompleted,
		Visibility:  domain.VisibilityShared,
		Source:      domain.SourceTypeManual,
	}
	if err := transferZero.Validate(); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount for zero transfer, got %v", err)
	}

	// 3. Transaction with invalid AI confidence in Validate()
	invalidConfidence := decimal.NewFromFloat(1.20)
	txInvalidConf := &domain.Transaction{
		ID:           uuid.New(),
		TenantID:     tenantID,
		AccountID:    accountID,
		Description:  "Coffee",
		Date:         now,
		Amount:       decimal.NewFromFloat(-10.00),
		Type:         domain.TransactionTypeExpense,
		Status:       domain.TransactionStatusCompleted,
		Visibility:   domain.VisibilityShared,
		Source:       domain.SourceTypeManual,
		AIConfidence: &invalidConfidence,
	}
	if err := txInvalidConf.Validate(); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount for confidence > 1.0, got %v", err)
	}

	negativeConfidence := decimal.NewFromFloat(-0.10)
	txInvalidConf.AIConfidence = &negativeConfidence
	if err := txInvalidConf.Validate(); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount for confidence < 0, got %v", err)
	}
}

func TestTransaction_Validate_ZeroDate(t *testing.T) {
	tx := &domain.Transaction{
		ID:          uuid.New(),
		TenantID:    uuid.New(),
		AccountID:   uuid.New(),
		Description: "Coffee",
		Date:        time.Time{}, // Zero date
		Amount:      decimal.NewFromFloat(-10.00),
		Type:        domain.TransactionTypeExpense,
		Status:      domain.TransactionStatusCompleted,
		Visibility:  domain.VisibilityShared,
		Source:      domain.SourceTypeManual,
	}

	if err := tx.Validate(); !errors.Is(err, domain.ErrInvalidDate) {
		t.Errorf("expected ErrInvalidDate, got %v", err)
	}
}

func TestTransaction_CategoryManagementAndLifecycle(t *testing.T) {
	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    uuid.New(),
		AccountID:   uuid.New(),
		Amount:      decimal.NewFromFloat(-30.00),
		Description: "Uber Trip",
		Type:        domain.TransactionTypeExpense,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	catID := uuid.New()
	confidence := decimal.NewFromFloat(0.95)

	// 1. Assign AI category
	err = tx.AssignCategory(catID, &confidence, "gemini-2.0-flash", true)
	if err != nil {
		t.Fatalf("unexpected error on AssignCategory: %v", err)
	}
	if *tx.CategoryID != catID || !tx.IsAICategorized || !tx.AIConfidence.Equal(confidence) {
		t.Error("expected AI category metadata set correctly")
	}

	// 2. AssignCategory validation errors
	err = tx.AssignCategory(uuid.Nil, &confidence, "gemini", true)
	if !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID on nil category, got %v", err)
	}
	invalidConf := decimal.NewFromFloat(1.50) // > 1.0
	err = tx.AssignCategory(catID, &invalidConf, "gemini", true)
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount on invalid confidence, got %v", err)
	}

	// 3. Verify category manually
	newCatID := uuid.New()
	err = tx.VerifyCategory(newCatID)
	if err != nil {
		t.Fatalf("unexpected error on VerifyCategory: %v", err)
	}
	if *tx.CategoryID != newCatID || !tx.IsManuallyVerified {
		t.Error("expected manually verified category set")
	}
	err = tx.VerifyCategory(uuid.Nil)
	if !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID on VerifyCategory with nil ID, got %v", err)
	}

	// 4. Void transaction
	tx.Void()
	if tx.Status != domain.TransactionStatusVoid {
		t.Errorf("expected status VOID, got %s", tx.Status)
	}
	if tx.IsSettled() {
		t.Error("expected void transaction not to be settled")
	}
}
