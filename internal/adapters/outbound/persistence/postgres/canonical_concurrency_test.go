package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestCanonicalImportConcurrentReplaysInsertExactlyOnce(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	account, err := domain.NewAccount(tenant, user, "Synthetic concurrent source", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accounts.Create(ctx, account))
	req := ports.CanonicalImportRequest{TenantID: tenant, UserID: user, AccountID: account.ID, SourceName: "synthetic.csv", Content: []byte("ID,Data,Descrição,Valor,Tipo\nSYN-A,01/02/2026,Repeated synthetic,1.00,Despesa\nSYN-B,01/02/2026,Repeated synthetic,1.00,Despesa\n")}
	const workers = 8
	type outcome struct {
		result *ports.CanonicalImportResult
		err    error
	}
	results := make(chan outcome, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, parseErr := parsers.NewCanonicalCSVParser().ParseCanonical(ctx, req)
			if parseErr != nil {
				results <- outcome{err: parseErr}
				return
			}
			result, importErr := postgres.NewTransactionRepository(pool).ImportCanonical(ctx, rows)
			results <- outcome{result, importErr}
		}()
	}
	wg.Wait()
	close(results)
	inserted, duplicates := 0, 0
	for result := range results {
		require.NoError(t, result.err)
		inserted += result.result.Inserted
		duplicates += result.result.Duplicates
	}
	require.Equal(t, 2, inserted)
	require.Equal(t, workers*2-2, duplicates)
	actual, err := accounts.GetByID(ctx, tenant, account.ID)
	require.NoError(t, err)
	require.Equal(t, "-2.00", actual.Balance.StringFixed(2))
}
