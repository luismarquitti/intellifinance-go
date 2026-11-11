package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTransferRepository_ConfirmAndUnlinkRealLedger(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenant, user := createTestTenantAndUser(t, pool)
	accountsRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	transferRepo := postgres.NewTransferRepository(pool)

	acc1, err := domain.NewAccount(tenant, user, "Conta Corrente", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accountsRepo.Create(ctx, acc1))

	acc2, err := domain.NewAccount(tenant, user, "Poupança Reserva", domain.AccountTypeSavings, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accountsRepo.Create(ctx, acc2))

	parser := parsers.NewCanonicalCSVParser()
	importLeg := func(accountID uuid.UUID, content string) *domain.Transaction {
		rows, parseErr := parser.ParseCanonical(ctx, ports.CanonicalImportRequest{TenantID: tenant, UserID: user, AccountID: accountID, SourceName: "synthetic-transfer.csv", Content: []byte(content)})
		require.NoError(t, parseErr)
		result, importErr := txRepo.ImportCanonical(ctx, rows)
		require.NoError(t, importErr)
		require.Equal(t, 1, result.Inserted)
		return rows[0]
	}
	outflow := importLeg(acc1.ID, "ID,Data,Descrição,Valor,Tipo\nEXT-OUT-001,01/02/2026,Synthetic outflow,500.00,Despesa\n")
	inflow := importLeg(acc2.ID, "ID,Data,Descrição,Valor,Tipo\nEXT-IN-002,01/02/2026,Synthetic inflow,500.00,Receita\n")

	// Antes da confirmação: outflow conta como despesa de 500, inflow conta como receita de 500
	summaryBefore, err := txRepo.Summarize(ctx, tenant, ports.TransactionFilter{})
	require.NoError(t, err)
	require.Equal(t, "500.00", summaryBefore.TotalIncome.StringFixed(2))
	require.Equal(t, "500.00", summaryBefore.TotalExpense.StringFixed(2))

	// Confirmação explícita do par de transferência
	legA, legB, err := transferRepo.ConfirmTransfer(ctx, tenant, outflow.ID, inflow.ID)
	require.NoError(t, err)
	require.Equal(t, &legB.ID, legA.TransferPeerTransactionID)
	require.Equal(t, &legA.ID, legB.TransferPeerTransactionID)
	require.Equal(t, domain.TransactionTypeTransfer, legA.Type)
	require.Equal(t, domain.TransactionTypeTransfer, legB.Type)

	// Após a confirmação: transferência interna NÃO distorce receitas nem despesas consolidadas!
	summaryAfter, err := txRepo.Summarize(ctx, tenant, ports.TransactionFilter{})
	require.NoError(t, err)
	require.Equal(t, "0.00", summaryAfter.TotalIncome.StringFixed(2), "Transferência interna não pode contar como receita consolidada")
	require.Equal(t, "0.00", summaryAfter.TotalExpense.StringFixed(2), "Transferência interna não pode contar como despesa consolidada")

	// Proveniência de importação continua intacta após a confirmação
	readOutflow, err := txRepo.GetByID(ctx, tenant, outflow.ID)
	require.NoError(t, err)
	require.Equal(t, "EXT-OUT-001", readOutflow.ExternalID)
	require.NotNil(t, readOutflow.Metadata["provenance"])
	expectedProvenance, err := json.Marshal(outflow.Metadata["provenance"])
	require.NoError(t, err)
	actualProvenance, err := json.Marshal(readOutflow.Metadata["provenance"])
	require.NoError(t, err)
	require.JSONEq(t, string(expectedProvenance), string(actualProvenance))

	// Teste de reversão / desassociação (somente em QA)
	unlinkedA, unlinkedB, err := transferRepo.UnlinkTransfer(ctx, tenant, outflow.ID)
	require.NoError(t, err)
	require.Nil(t, unlinkedA.TransferPeerTransactionID)
	require.Nil(t, unlinkedB.TransferPeerTransactionID)
	require.Equal(t, domain.TransactionTypeExpense, unlinkedA.Type)
	require.Equal(t, domain.TransactionTypeIncome, unlinkedB.Type)

	// Após desassociar, volta aos tipos originais de despesa e receita
	summaryReverted, err := txRepo.Summarize(ctx, tenant, ports.TransactionFilter{})
	require.NoError(t, err)
	require.Equal(t, "500.00", summaryReverted.TotalIncome.StringFixed(2))
	require.Equal(t, "500.00", summaryReverted.TotalExpense.StringFixed(2))
}

func TestTransferRepository_CreateDualLegTransfer(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenant, user := createTestTenantAndUser(t, pool)
	accountsRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	transferRepo := postgres.NewTransferRepository(pool)

	acc1, err := domain.NewAccount(tenant, user, "Origem CC", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accountsRepo.Create(ctx, acc1))

	acc2, err := domain.NewAccount(tenant, user, "Destino Invest", domain.AccountTypeInvestment, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accountsRepo.Create(ctx, acc2))

	amt := decimal.NewFromFloat(1234.56)
	now := time.Now().UTC()

	outflow, inflow, err := domain.NewInternalTransfer(tenant, acc1.ID, acc2.ID, amt, now, "Aporte CDB", &user)
	require.NoError(t, err)

	outflow.Metadata = map[string]any{"transfer_idempotency_key": "synthetic-create"}
	inflow.Metadata = map[string]any{"transfer_idempotency_key": "synthetic-create"}
	err = transferRepo.CreateTransfer(ctx, outflow, inflow)
	require.NoError(t, err)

	// Saldos das contas atualizados atomicamente exatamente uma vez pelo trigger
	readAcc1, err := accountsRepo.GetByID(ctx, tenant, acc1.ID)
	require.NoError(t, err)
	require.Equal(t, "-1234.56", readAcc1.Balance.StringFixed(2))

	readAcc2, err := accountsRepo.GetByID(ctx, tenant, acc2.ID)
	require.NoError(t, err)
	require.Equal(t, "1234.56", readAcc2.Balance.StringFixed(2))

	// Total consolidado de receitas e despesas não alterado
	summary, err := txRepo.Summarize(ctx, tenant, ports.TransactionFilter{})
	require.NoError(t, err)
	require.Equal(t, "0.00", summary.TotalIncome.StringFixed(2))
	require.Equal(t, "0.00", summary.TotalExpense.StringFixed(2))
}

func TestTransferRepository_RejectsCrossTenantAndInvalidPairs(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenant1, user1 := createTestTenantAndUser(t, pool)
	tenant2, user2 := createTestTenantAndUser(t, pool)

	accountsRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	transferRepo := postgres.NewTransferRepository(pool)

	acc1, _ := domain.NewAccount(tenant1, user1, "Conta 1", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accountsRepo.Create(ctx, acc1))

	acc2, _ := domain.NewAccount(tenant2, user2, "Conta 2", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accountsRepo.Create(ctx, acc2))

	amt := decimal.NewFromFloat(100.00)
	now := time.Now().UTC()

	tx1, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenant1,
		AccountID:   acc1.ID,
		Date:        now,
		Amount:      amt.Neg(),
		Description: "Outflow",
		Type:        domain.TransactionTypeExpense,
	})
	require.NoError(t, txRepo.Create(ctx, tx1))

	tx2, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenant2, // Cross-tenant!
		AccountID:   acc2.ID,
		Date:        now,
		Amount:      amt,
		Description: "Inflow",
		Type:        domain.TransactionTypeIncome,
	})
	require.NoError(t, txRepo.Create(ctx, tx2))

	// Tenant 1 tenta confirmar com transação do Tenant 2 -> Deve falhar por isolamento de tenant
	_, _, err := transferRepo.ConfirmTransfer(ctx, tenant1, tx1.ID, tx2.ID)
	require.Error(t, err)

	// Mesma conta -> deve falhar
	tx3, _ := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:    tenant1,
		AccountID:   acc1.ID, // Mesma conta que tx1!
		Date:        now,
		Amount:      amt,
		Description: "Inflow Mesma Conta",
		Type:        domain.TransactionTypeIncome,
	})
	require.NoError(t, txRepo.Create(ctx, tx3))

	_, _, err = transferRepo.ConfirmTransfer(ctx, tenant1, tx1.ID, tx3.ID)
	require.ErrorIs(t, err, domain.ErrTransferSameAccount)

	// Inexistente -> deve falhar
	_, _, err = transferRepo.ConfirmTransfer(ctx, tenant1, tx1.ID, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}
