package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestProviderBalanceSnapshot_Validation(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	bal := decimal.NewFromFloat(5420.15)
	now := time.Now().UTC()

	// Missing tenant
	_, err := domain.NewProviderBalanceSnapshot(uuid.Nil, accountID, "PLUGGY", bal, "BRL", now)
	require.Error(t, err)

	// Missing account
	_, err = domain.NewProviderBalanceSnapshot(tenantID, uuid.Nil, "PLUGGY", bal, "BRL", now)
	require.Error(t, err)

	// Missing provider
	_, err = domain.NewProviderBalanceSnapshot(tenantID, accountID, "", bal, "BRL", now)
	require.Error(t, err)

	// Success
	snap, err := domain.NewProviderBalanceSnapshot(tenantID, accountID, "pluggy", bal, "", now)
	require.NoError(t, err)
	require.Equal(t, "PLUGGY", snap.Provider)
	require.Equal(t, "BRL", snap.Currency)
	require.Equal(t, bal, snap.Balance)
}
