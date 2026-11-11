package domain_test

import (
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestReviewTransferConfirmationSettledAndReversible(t *testing.T) {
	for _, scenario := range []string{"pending", "inflow_pending", "void", "standalone_transfer", "wrong_sign_type", "wrong_inflow_type", "one_sided_peer"} {
		t.Run(scenario, func(t *testing.T) {
			a, b, err := domain.NewInternalTransfer(uuid.New(), uuid.New(), uuid.New(), decimal.NewFromInt(10), time.Now(), "synthetic", nil)
			require.NoError(t, err)
			a.TransferPeerTransactionID = nil
			b.TransferPeerTransactionID = nil
			a.Type = domain.TransactionTypeExpense
			b.Type = domain.TransactionTypeIncome
			switch scenario {
			case "pending":
				a.Status = domain.TransactionStatusPending
			case "inflow_pending":
				b.Status = domain.TransactionStatusPending
			case "void":
				a.Status = domain.TransactionStatusVoid
			case "wrong_inflow_type":
				b.Type = domain.TransactionTypeExpense
			case "standalone_transfer":
				a.Type = domain.TransactionTypeTransfer
				b.Type = domain.TransactionTypeTransfer
			case "wrong_sign_type":
				a.Type = domain.TransactionTypeIncome
			case "one_sided_peer":
				a.TransferPeerTransactionID = &b.ID
			}
			require.Error(t, domain.ConfirmTransferPair(a, b))
		})
	}
}

func TestReviewTransferSettledConfirmationReplayAndUnlink(t *testing.T) {
	for _, status := range []domain.TransactionStatus{domain.TransactionStatusCompleted, domain.TransactionStatusReconciled} {
		a, b, err := domain.NewInternalTransfer(uuid.New(), uuid.New(), uuid.New(), decimal.NewFromInt(10), time.Now(), "Synthetic settled", nil)
		require.NoError(t, err)
		a.Type = domain.TransactionTypeExpense
		b.Type = domain.TransactionTypeIncome
		a.Status = status
		b.Status = status
		a.TransferPeerTransactionID = nil
		b.TransferPeerTransactionID = nil
		require.NoError(t, domain.ConfirmTransferPair(a, b))
		require.NoError(t, domain.ConfirmTransferPair(b, a))
		require.NoError(t, domain.UnlinkTransferPair(a, b))
		require.Equal(t, domain.TransactionTypeExpense, a.Type)
		require.Equal(t, domain.TransactionTypeIncome, b.Type)
		require.Equal(t, status, a.Status)
		require.Equal(t, status, b.Status)
	}
}

func TestReviewTransferAmountPrecisionAndLimits(t *testing.T) {
	tenant := uuid.New()
	src := uuid.New()
	dst := uuid.New()
	now := time.Now()

	// 1.001 rejected
	_, _, err := domain.NewInternalTransfer(tenant, src, dst, decimal.RequireFromString("1.001"), now, "SubCent", nil)
	require.ErrorIs(t, err, domain.ErrInvalidAmount)

	// 0.001 rejected
	_, _, err = domain.NewInternalTransfer(tenant, src, dst, decimal.RequireFromString("0.001"), now, "SubCent", nil)
	require.ErrorIs(t, err, domain.ErrInvalidAmount)

	// Exceeds max
	_, _, err = domain.NewInternalTransfer(tenant, src, dst, decimal.RequireFromString("10000000000000.00"), now, "Overflow", nil)
	require.ErrorIs(t, err, domain.ErrInvalidAmount)

	// 1.000 accepted
	out, in, err := domain.NewInternalTransfer(tenant, src, dst, decimal.RequireFromString("1.000"), now, "Trailing Zero", nil)
	require.NoError(t, err)
	require.True(t, out.Amount.Equal(decimal.RequireFromString("-1.00")))
	require.True(t, in.Amount.Equal(decimal.RequireFromString("1.00")))

	// Max value accepted
	outMax, inMax, err := domain.NewInternalTransfer(tenant, src, dst, domain.MaxMonetaryAmount, now, "Max", nil)
	require.NoError(t, err)
	require.True(t, outMax.Amount.Equal(domain.MaxMonetaryAmount.Neg()))
	require.True(t, inMax.Amount.Equal(domain.MaxMonetaryAmount))
}

