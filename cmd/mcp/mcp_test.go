package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/stretchr/testify/require"
)

func getTestPool(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("TEST_DATABASE_URL")
	}
	require.NotEmpty(t, dbURL, "DATABASE_URL is required for tests")
	require.NoError(t, config.ValidateDisposableDatabase(dbURL, os.Getenv("ENVIRONMENT")))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)

	tenantID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO tenants (id, name, type, currency) VALUES ($1, $2, 'HOUSEHOLD', 'BRL')", tenantID, "MCP Test Tenant")
	require.NoError(t, err)

	return pool, tenantID
}

func TestMCPServer_InitializeAndListTools(t *testing.T) {
	pool, tenantID := getTestPool(t)
	defer pool.Close()

	accountRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	invRepo := postgres.NewInvoiceRepository(pool)
	snapRepo := postgres.NewProviderBalanceSnapshotRepository(pool)
	actionRepo := postgres.NewPendingActionRepository(pool)
	txService := app.NewTransactionService(txRepo)

	server := &MCPServer{
		pool:        pool,
		tenantID:    tenantID,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		accountRepo: accountRepo,
		txService:   txService,
		invRepo:     invRepo,
		snapRepo:    snapRepo,
		actionRepo:  actionRepo,
	}

	// 1. Test initialize
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	inR, inW := io.Pipe()
	var outBuf bytes.Buffer

	go func() {
		defer func() { _ = inW.Close() }()
		_, _ = inW.Write([]byte(initReq))
	}()

	server.Run(inR, &outBuf)

	var initResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(outBuf.Bytes(), &initResp))
	require.Equal(t, float64(1), initResp.ID)
	resMap, ok := initResp.Result.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "2024-11-05", resMap["protocolVersion"])

	// 2. Test tools/list
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	inR2, inW2 := io.Pipe()
	var outBuf2 bytes.Buffer

	go func() {
		defer func() { _ = inW2.Close() }()
		_, _ = inW2.Write([]byte(listReq))
	}()

	server.Run(inR2, &outBuf2)

	var listResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(outBuf2.Bytes(), &listResp))
	require.Equal(t, float64(2), listResp.ID)
	toolsResult, ok := listResp.Result.(map[string]any)
	require.True(t, ok)
	tools, ok := toolsResult["tools"].([]any)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(tools), 5)

	toolNames := []string{}
	for _, tool := range tools {
		tmap := tool.(map[string]any)
		toolNames = append(toolNames, tmap["name"].(string))
	}
	require.Contains(t, toolNames, "list_accounts")
	require.Contains(t, toolNames, "get_balances")
	require.Contains(t, toolNames, "list_transactions")
	require.Contains(t, toolNames, "list_invoices")
	require.Contains(t, toolNames, "list_pending_actions")
	require.Contains(t, toolNames, "propose_categorization")
}

func TestMCPServer_ProposeCategorizationHITL(t *testing.T) {
	pool, tenantID := getTestPool(t)
	defer pool.Close()

	accountRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	invRepo := postgres.NewInvoiceRepository(pool)
	snapRepo := postgres.NewProviderBalanceSnapshotRepository(pool)
	actionRepo := postgres.NewPendingActionRepository(pool)
	txService := app.NewTransactionService(txRepo)

	server := &MCPServer{
		pool:        pool,
		tenantID:    tenantID,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		accountRepo: accountRepo,
		txService:   txService,
		invRepo:     invRepo,
		snapRepo:    snapRepo,
		actionRepo:  actionRepo,
	}

	txID := uuid.New()
	catID := uuid.New()

	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "propose_categorization",
			"arguments": map[string]any{
				"transaction_id": txID.String(),
				"category_id":    catID.String(),
				"reason":         "Transaction description contains Pao de Acucar",
			},
		},
	}
	callReqBytes, _ := json.Marshal(callReq)
	callReqBytes = append(callReqBytes, '\n')

	inR, inW := io.Pipe()
	var outBuf bytes.Buffer

	go func() {
		defer func() { _ = inW.Close() }()
		_, _ = inW.Write(callReqBytes)
	}()

	server.Run(inR, &outBuf)

	var callResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(outBuf.Bytes(), &callResp))
	require.Equal(t, float64(10), callResp.ID)
	require.Nil(t, callResp.Error)

	resMap, ok := callResp.Result.(map[string]any)
	require.True(t, ok)
	content, ok := resMap["content"].([]any)
	require.True(t, ok)
	firstItem := content[0].(map[string]any)
	text := firstItem["text"].(string)
	require.Contains(t, text, "Proposal recorded successfully")
	require.Contains(t, text, "Status: PENDING")

	// Verify that pending action exists in database
	actions, err := actionRepo.ListPending(context.Background(), tenantID)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, txID.String(), actions[0].Payload["transaction_id"])
	require.Equal(t, catID.String(), actions[0].Payload["category_id"])
}
