package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTransfer_ValidateAndConfirmTransferPair(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	sourceAccount := uuid.New()
	destAccount := uuid.New()
	now := time.Now().UTC()

	amount := decimal.NewFromFloat(150.75)

	outflow, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       sourceAccount,
		CreatedByUserID: &userID,
		Date:            now,
		Amount:          amount.Neg(),
		Description:     "Transferência enviada para Poupança",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Source:          domain.SourceTypeCSV,
		ExternalID:      "SRC-123",
		Metadata: map[string]any{
			"provenance": map[string]any{
				"file_sha256": "abcdef123456",
				"row_sha256":  "123456abcdef",
			},
		},
	})
	require.NoError(t, err)

	inflow, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       destAccount,
		CreatedByUserID: &userID,
		Date:            now,
		Amount:          amount,
		Description:     "Transferência recebida da CC",
		Type:            domain.TransactionTypeIncome,
		Status:          domain.TransactionStatusCompleted,
		Source:          domain.SourceTypeCSV,
		ExternalID:      "DST-456",
		Metadata: map[string]any{
			"provenance": map[string]any{
				"file_sha256": "fedcba654321",
				"row_sha256":  "654321fedcba",
			},
		},
	})
	require.NoError(t, err)

	// Valid pair confirmation
	err = domain.ConfirmTransferPair(outflow, inflow)
	require.NoError(t, err)
	require.Equal(t, &inflow.ID, outflow.TransferPeerTransactionID)
	require.Equal(t, &outflow.ID, inflow.TransferPeerTransactionID)
	require.Equal(t, domain.TransactionTypeTransfer, outflow.Type)
	require.Equal(t, domain.TransactionTypeTransfer, inflow.Type)

	// Original provenance and fields preserved
	require.Equal(t, "SRC-123", outflow.ExternalID)
	require.Equal(t, "DST-456", inflow.ExternalID)
	require.NotNil(t, outflow.Metadata["provenance"])
	require.NotNil(t, inflow.Metadata["provenance"])
}

func TestTransfer_RejectsInvalidPairs(t *testing.T) {
	tenantID := uuid.New()
	sourceAccount := uuid.New()
	destAccount := uuid.New()
	now := time.Now().UTC()

	t.Run("same account rejected", func(t *testing.T) {
		t1, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   sourceAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(-100.00),
			Description: "Outflow",
			Type:        domain.TransactionTypeExpense,
		})
		t2, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   sourceAccount, // same account!
			Date:        now,
			Amount:      decimal.NewFromFloat(100.00),
			Description: "Inflow",
			Type:        domain.TransactionTypeIncome,
		})

		err := domain.ConfirmTransferPair(t1, t2)
		require.ErrorIs(t, err, domain.ErrTransferSameAccount)
	})

	t.Run("amount mismatch rejected without inventing adjustments", func(t *testing.T) {
		t1, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   sourceAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(-100.00),
			Description: "Outflow",
			Type:        domain.TransactionTypeExpense,
		})
		t2, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   destAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(100.01), // 1 cent difference
			Description: "Inflow",
			Type:        domain.TransactionTypeIncome,
		})

		err := domain.ConfirmTransferPair(t1, t2)
		require.ErrorIs(t, err, domain.ErrTransferAmountMismatch)
	})

	t.Run("same sign rejected", func(t *testing.T) {
		t1, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   sourceAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(-100.00),
			Description: "Outflow 1",
			Type:        domain.TransactionTypeExpense,
		})
		t2, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   destAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(-100.00), // both negative!
			Description: "Outflow 2",
			Type:        domain.TransactionTypeExpense,
		})

		err := domain.ConfirmTransferPair(t1, t2)
		require.ErrorIs(t, err, domain.ErrTransferAmountMismatch)
	})

	t.Run("cross tenant rejected", func(t *testing.T) {
		otherTenantID := uuid.New()
		t1, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   sourceAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(-100.00),
			Description: "Outflow",
			Type:        domain.TransactionTypeExpense,
		})
		t2, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    otherTenantID, // different tenant
			AccountID:   destAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(100.00),
			Description: "Inflow",
			Type:        domain.TransactionTypeIncome,
		})

		err := domain.ConfirmTransferPair(t1, t2)
		require.ErrorIs(t, err, domain.ErrTransferCrossTenant)
	})

	t.Run("void leg rejected", func(t *testing.T) {
		t1, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   sourceAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(-100.00),
			Description: "Outflow",
			Type:        domain.TransactionTypeExpense,
			Status:      domain.TransactionStatusVoid,
		})
		t2, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   destAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(100.00),
			Description: "Inflow",
			Type:        domain.TransactionTypeIncome,
		})

		err := domain.ConfirmTransferPair(t1, t2)
		require.ErrorIs(t, err, domain.ErrTransferVoidLeg)
	})

	t.Run("already linked transaction rejected", func(t *testing.T) {
		existingPeer := uuid.New()
		t1, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   sourceAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(-100.00),
			Description: "Outflow",
			Type:        domain.TransactionTypeExpense,
		})
		t1.TransferPeerTransactionID = &existingPeer

		t2, _ := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   destAccount,
			Date:        now,
			Amount:      decimal.NewFromFloat(100.00),
			Description: "Inflow",
			Type:        domain.TransactionTypeIncome,
		})

		err := domain.ConfirmTransferPair(t1, t2)
		require.ErrorIs(t, err, domain.ErrTransferAlreadyLinked)
	})
}

func TestTransfer_UnlinkPair(t *testing.T) {
	tenantID := uuid.New()
	sourceAccount := uuid.New()
	destAccount := uuid.New()
	now := time.Now().UTC()

	t1, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   sourceAccount,
		Date:        now,
		Amount:      decimal.NewFromFloat(-50.00),
		Description: "Outflow",
		Type:        domain.TransactionTypeExpense,
	})
	t2, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenantID,
		AccountID:   destAccount,
		Date:        now,
		Amount:      decimal.NewFromFloat(50.00),
		Description: "Inflow",
		Type:        domain.TransactionTypeIncome,
	})

	require.NoError(t, domain.ConfirmTransferPair(t1, t2))

	// Unlink
	require.NoError(t, domain.UnlinkTransferPair(t1, t2))
	require.Nil(t, t1.TransferPeerTransactionID)
	require.Nil(t, t2.TransferPeerTransactionID)
	require.Equal(t, domain.TransactionTypeExpense, t1.Type)
	require.Equal(t, domain.TransactionTypeIncome, t2.Type)
}

func TestTransfer_NewInternalTransfer(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	sourceAccount := uuid.New()
	destAccount := uuid.New()
	now := time.Now().UTC()
	amt := decimal.NewFromFloat(200.50)

	outflow, inflow, err := domain.NewInternalTransfer(
		tenantID,
		sourceAccount,
		destAccount,
		amt,
		now,
		"Aplicação CDB",
		&userID,
	)
	require.NoError(t, err)
	require.Equal(t, amt.Neg(), outflow.Amount)
	require.Equal(t, amt, inflow.Amount)
	require.Equal(t, sourceAccount, outflow.AccountID)
	require.Equal(t, destAccount, inflow.AccountID)
	require.Equal(t, &inflow.ID, outflow.TransferPeerTransactionID)
	require.Equal(t, &outflow.ID, inflow.TransferPeerTransactionID)
	require.Equal(t, domain.TransactionTypeTransfer, outflow.Type)
	require.Equal(t, domain.TransactionTypeTransfer, inflow.Type)
}
