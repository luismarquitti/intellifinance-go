package postgres_test

import (
	"context"

	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestCanonicalImportRealLedgerAndManualClassification(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	account, err := domain.NewAccount(tenant, user, "Synthetic canonical", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accounts.Create(ctx, account))
	req := ports.CanonicalImportRequest{TenantID: tenant, UserID: user, AccountID: account.ID, SourceName: "synthetic.csv", Content: []byte("ID,Data,Descrição,Valor,Tipo\nSYN-1,01/02/2026,Repeated synthetic,10.50,Despesa\nSYN-2,01/02/2026,Repeated synthetic,10.50,Despesa\nSYN-3,02/02/2026,Synthetic income,30.00,Receita\n")}
	parser := parsers.NewCanonicalCSVParser()
	rows, err := parser.ParseCanonical(ctx, req)
	require.NoError(t, err)
	repo := postgres.NewTransactionRepository(pool)
	result, err := repo.ImportCanonical(ctx, rows)
	require.NoError(t, err)
	require.Equal(t, 3, result.Inserted)
	actual, err := accounts.GetByID(ctx, tenant, account.ID)
	require.NoError(t, err)
	require.Equal(t, "9.00", actual.Balance.StringFixed(2))
	require.NotEqual(t, rows[0].Fingerprint, rows[1].Fingerprint)
	for _, row := range rows {
		require.Equal(t, domain.ComputeFingerprint(row.TenantID, row.AccountID, row.Date, row.Amount, row.CleanDescription, row.SequenceIndex), row.Fingerprint, "canonical fingerprint must not depend on external ID")
	}
	cat, err := domain.NewCategory(tenant, nil, "Synthetic manually classified", domain.TransactionTypeExpense, false)
	require.NoError(t, err)
	require.NoError(t, postgres.NewCategoryRepository(pool).Create(ctx, cat))
	stored, err := repo.GetByID(ctx, tenant, rows[0].ID)
	require.NoError(t, err)
	stored.CategoryID = &cat.ID
	stored.Notes = "Manual synthetic note"
	stored.Fingerprint = domain.GenerateTransactionFingerprint(stored.TenantID, stored.AccountID, stored.Date, stored.Amount, stored.CleanDescription, stored.ExternalID, stored.SequenceIndex)
	require.NoError(t, repo.Update(ctx, stored))
	canonicalStored, getErr := repo.GetByID(ctx, tenant, rows[0].ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ComputeFingerprint(stored.TenantID, stored.AccountID, stored.Date, stored.Amount, stored.CleanDescription, stored.SequenceIndex), canonicalStored.Fingerprint, "editing canonical rows must retain canonical formula even through legacy handlers")
	req.SourceName = "different-representation.csv"
	again, err := parser.ParseCanonical(ctx, req)
	require.NoError(t, err)
	result, err = repo.ImportCanonical(ctx, again)
	require.NoError(t, err)
	require.Zero(t, result.Inserted)
	require.Equal(t, 3, result.Duplicates)
	stored, err = repo.GetByID(ctx, tenant, rows[0].ID)
	require.NoError(t, err)
	require.Equal(t, cat.ID, *stored.CategoryID)
	require.Equal(t, "Manual synthetic note", stored.Notes)
	// A source-ID conflict after a fresh row must roll back the fresh row and balance.
	req.Content = []byte("ID,Data,Descrição,Valor,Tipo\nSYN-NEW,03/02/2026,Must rollback,5.00,Despesa\nSYN-1,01/02/2026,Changed original,99.00,Despesa\n")
	changed, err := parser.ParseCanonical(ctx, req)
	require.NoError(t, err)
	_, err = repo.ImportCanonical(ctx, changed)
	require.Error(t, err)
	found, count, err := repo.List(ctx, tenant, ports.TransactionFilter{AccountID: &account.ID, Limit: 100, Page: 1})
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
	require.Len(t, found, 3)
	actual, err = accounts.GetByID(ctx, tenant, account.ID)
	require.NoError(t, err)
	require.Equal(t, "9.00", actual.Balance.StringFixed(2))
	// A new original ID with an identical legitimate movement is not a duplicate.
	req.Content = []byte("ID,Data,Descrição,Valor,Tipo\nSYN-4,01/02/2026,Repeated synthetic,10.50,Despesa\n")
	extra, err := parser.ParseCanonical(ctx, req)
	require.NoError(t, err)
	result, err = repo.ImportCanonical(ctx, extra)
	require.NoError(t, err)
	require.Equal(t, 1, result.Inserted)
	actual, err = accounts.GetByID(ctx, tenant, account.ID)
	require.NoError(t, err)
	require.Equal(t, "-1.50", actual.Balance.StringFixed(2))
	stored, err = repo.GetByID(ctx, tenant, rows[0].ID)
	require.NoError(t, err)
	stored.Metadata["provenance"].(map[string]any)["source_sha256"] = "synthetic tampering"
	require.Error(t, repo.Update(ctx, stored), "retained provenance cannot be edited along with classification")
}
