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
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestPool(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("TEST_DATABASE_URL")
	}
	if dbURL == "" {
		//nolint:gosec // #nosec G101 -- default local test database connection string
		dbURL = "postgres://intellifinance_app:CHANGE_IN_PRODUCTION@localhost:5432/intellifinance_test?sslmode=disable"
	}
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

func runMCPRequest(t *testing.T, server *MCPServer, req map[string]any) JSONRPCResponse {
	t.Helper()
	reqBytes, err := json.Marshal(req)
	require.NoError(t, err)
	reqBytes = append(reqBytes, '\n')

	inR, inW := io.Pipe()
	var outBuf bytes.Buffer

	go func() {
		defer func() { _ = inW.Close() }()
		_, _ = inW.Write(reqBytes)
	}()

	server.Run(inR, &outBuf)

	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal(outBuf.Bytes(), &resp))
	return resp
}

type memPendingActionRepo struct {
	actions map[uuid.UUID]*domain.PendingAction
}

func newMemPendingActionRepo() *memPendingActionRepo {
	return &memPendingActionRepo{actions: make(map[uuid.UUID]*domain.PendingAction)}
}

func (r *memPendingActionRepo) Save(_ context.Context, action *domain.PendingAction) error {
	cp := *action
	r.actions[action.ID] = &cp
	return nil
}

func (r *memPendingActionRepo) FindByID(_ context.Context, tenantID, id uuid.UUID) (*domain.PendingAction, error) {
	act, ok := r.actions[id]
	if !ok || act.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	cp := *act
	return &cp, nil
}

func (r *memPendingActionRepo) ListPending(_ context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error) {
	out := make([]*domain.PendingAction, 0)
	for _, act := range r.actions {
		if act.TenantID == tenantID && act.Status == domain.ActionStatusPending {
			cp := *act
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memPendingActionRepo) UpdateDecision(_ context.Context, _, _ uuid.UUID, _ domain.PendingActionStatus, _ *uuid.UUID, _ *string) error {
	return nil
}

func (r *memPendingActionRepo) MarkExecuted(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

type memTxRepo struct {
	txs map[uuid.UUID]*domain.Transaction
}

func newMemTxRepo() *memTxRepo {
	return &memTxRepo{txs: make(map[uuid.UUID]*domain.Transaction)}
}

func (r *memTxRepo) Create(_ context.Context, tx *domain.Transaction) error {
	cp := *tx
	r.txs[tx.ID] = &cp
	return nil
}
func (r *memTxRepo) BatchCreate(_ context.Context, _ []*domain.Transaction) error { return nil }
func (r *memTxRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error) {
	tx, ok := r.txs[id]
	if !ok || tx.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	cp := *tx
	return &cp, nil
}
func (r *memTxRepo) List(_ context.Context, _ uuid.UUID, _ ports.TransactionFilter) ([]*domain.Transaction, int64, error) {
	return nil, 0, nil
}
func (r *memTxRepo) Summarize(_ context.Context, _ uuid.UUID, _ ports.TransactionFilter) (*ports.TransactionSummary, error) {
	return nil, nil
}
func (r *memTxRepo) Update(_ context.Context, _ *domain.Transaction) error           { return nil }
func (r *memTxRepo) Delete(_ context.Context, _, _ uuid.UUID) error                  { return nil }
func (r *memTxRepo) BatchDelete(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error { return nil }
func (r *memTxRepo) BatchUpdateCategory(_ context.Context, _ uuid.UUID, _ []uuid.UUID, _ *uuid.UUID) error {
	return nil
}
func (r *memTxRepo) FindExistingFingerprints(_ context.Context, _ uuid.UUID, _ []string) (map[string]bool, error) {
	return nil, nil
}
func (r *memTxRepo) CreateSplits(_ context.Context, _, _ uuid.UUID, _ []domain.Split) error {
	return nil
}
func (r *memTxRepo) GetSplits(_ context.Context, _, _ uuid.UUID) ([]domain.Split, error) {
	return nil, nil
}
func (r *memTxRepo) SetBillID(_ context.Context, _, _, _ uuid.UUID) error { return nil }
func (r *memTxRepo) ListByBillID(_ context.Context, _, _ uuid.UUID) ([]*domain.Transaction, error) {
	return nil, nil
}
func (r *memTxRepo) ReconcileExternalTransaction(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID, _ domain.TransactionStatus) error {
	return nil
}

type memCategoryRepo struct {
	cats map[uuid.UUID]*domain.Category
}

func newMemCategoryRepo() *memCategoryRepo {
	return &memCategoryRepo{cats: make(map[uuid.UUID]*domain.Category)}
}

func (r *memCategoryRepo) Create(_ context.Context, cat *domain.Category) error {
	cp := *cat
	r.cats[cat.ID] = &cp
	return nil
}
func (r *memCategoryRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Category, error) {
	cat, ok := r.cats[id]
	if !ok || cat.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	cp := *cat
	return &cp, nil
}
func (r *memCategoryRepo) List(_ context.Context, _ uuid.UUID, _ ports.CategoryFilter) ([]*domain.Category, error) {
	return nil, nil
}
func (r *memCategoryRepo) Update(_ context.Context, _ *domain.Category) error { return nil }
func (r *memCategoryRepo) Delete(_ context.Context, _, _ uuid.UUID) error     { return nil }
func (r *memCategoryRepo) CountTransactionsByCategory(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func TestMCPServer_InitializeAndListTools(t *testing.T) {
	server := &MCPServer{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
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

func TestMCPServer_ProposeCategorizationHydrationAndListPending(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()

	actionRepo := newMemPendingActionRepo()
	txRepo := newMemTxRepo()
	catRepo := newMemCategoryRepo()

	cat, err := domain.NewCategory(tenantID, nil, "Alimentação & Mercado", domain.TransactionTypeExpense, true)
	require.NoError(t, err)
	require.NoError(t, catRepo.Create(ctx, cat))

	txDate := time.Date(2026, 11, 10, 15, 30, 0, 0, domain.BrazilLocation)
	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       accountID,
		CreatedByUserID: &userID,
		Date:            txDate,
		Amount:          decimal.RequireFromString("-145.90"),
		Description:     "COMPRA CARTAO SUPERMERCADO PAO DE ACUCAR",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeManual,
	})
	require.NoError(t, err)
	require.NoError(t, txRepo.Create(ctx, tx))

	pendingSvc := app.NewPendingActionService(actionRepo, txRepo, catRepo, nil, nil, nil)
	server := &MCPServer{
		tenantID:             tenantID,
		logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		pendingActionService: pendingSvc,
	}

	// 1. propose_categorization hydrates UI snapshot fields
	proposeResp := runMCPRequest(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "propose_categorization",
			"arguments": map[string]any{
				"transaction_id": tx.ID.String(),
				"category_id":    cat.ID.String(),
				"reason":         "Supermercado Pão de Açúcar",
			},
		},
	})
	require.Nil(t, proposeResp.Error)
	proposeResMap := proposeResp.Result.(map[string]any)
	assert.Nil(t, proposeResMap["isError"])
	proposeContent := proposeResMap["content"].([]any)[0].(map[string]any)
	assert.Contains(t, proposeContent["text"], "Proposal recorded successfully")

	// 2. list_pending_actions returns hydrated proposal
	listResp := runMCPRequest(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "list_pending_actions",
			"arguments": map[string]any{},
		},
	})
	require.Nil(t, listResp.Error)
	listResMap := listResp.Result.(map[string]any)
	assert.Nil(t, listResMap["isError"])
	listText := listResMap["content"].([]any)[0].(map[string]any)["text"].(string)

	var listed []domain.PendingAction
	require.NoError(t, json.Unmarshal([]byte(listText), &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, "mcp-server", listed[0].CreatedByAgent)
	assert.Equal(t, tx.ID.String(), listed[0].Payload["transaction_id"])
	assert.Equal(t, cat.ID.String(), listed[0].Payload["category_id"])
	assert.Equal(t, "COMPRA CARTAO SUPERMERCADO PAO DE ACUCAR", listed[0].Payload["description"])
	assert.Equal(t, tx.CleanDescription, listed[0].Payload["clean_description"])
	assert.Equal(t, "-145.90", listed[0].Payload["amount"])
	assert.Equal(t, "2026-11-10", listed[0].Payload["date"])
	assert.Equal(t, cat.Name, listed[0].Payload["category_name"])
	assert.Equal(t, "Supermercado Pão de Açúcar", listed[0].Payload["reason"])

	// 3. Duplicate proposal for same transaction fails with isError: true
	dupResp := runMCPRequest(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "propose_categorization",
			"arguments": map[string]any{
				"transaction_id": tx.ID.String(),
				"category_id":    cat.ID.String(),
				"reason":         "Duplicate attempt",
			},
		},
	})
	require.Nil(t, dupResp.Error)
	dupResMap := dupResp.Result.(map[string]any)
	assert.Equal(t, true, dupResMap["isError"])
	dupText := dupResMap["content"].([]any)[0].(map[string]any)["text"].(string)
	assert.Contains(t, dupText, domain.ErrDuplicatePendingAction.Error())

	// 4. Missing transaction fails with isError: true
	missingResp := runMCPRequest(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "propose_categorization",
			"arguments": map[string]any{
				"transaction_id": uuid.New().String(),
				"category_id":    cat.ID.String(),
				"reason":         "Missing transaction",
			},
		},
	})
	require.Nil(t, missingResp.Error)
	missingResMap := missingResp.Result.(map[string]any)
	assert.Equal(t, true, missingResMap["isError"])
}

func TestMCPServer_ProposeCategorizationHITL(t *testing.T) {
	pool, tenantID := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'hash', 'MCP Test User')",
		userID, "mcp-"+userID.String()[:8]+"@example.com")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO tenant_members (tenant_id, user_id, role) VALUES ($1, $2, 'OWNER')", tenantID, userID)
	require.NoError(t, err)

	accountRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	catRepo := postgres.NewCategoryRepository(pool)
	invRepo := postgres.NewInvoiceRepository(pool)
	snapRepo := postgres.NewProviderBalanceSnapshotRepository(pool)
	actionRepo := postgres.NewPendingActionRepository(pool)
	transferRepo := postgres.NewTransferRepository(pool)

	account, err := domain.NewAccount(tenantID, userID, "Conta Corrente MCP", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accountRepo.Create(ctx, account))

	cat, err := domain.NewCategory(tenantID, nil, "Alimentação & Mercado", domain.TransactionTypeExpense, true)
	require.NoError(t, err)
	require.NoError(t, catRepo.Create(ctx, cat))

	txDate := time.Date(2026, 11, 10, 14, 0, 0, 0, domain.BrazilLocation)
	tx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenantID,
		AccountID:       account.ID,
		CreatedByUserID: &userID,
		Date:            txDate,
		Amount:          decimal.RequireFromString("-145.90"),
		Description:     "SUPERMERCADO PAO DE ACUCAR",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeManual,
	})
	require.NoError(t, err)
	require.NoError(t, txRepo.Create(ctx, tx))

	txService := app.NewTransactionService(txRepo)
	transferService := app.NewTransferService(transferRepo)
	pendingActionService := app.NewPendingActionService(actionRepo, txRepo, catRepo, accountRepo, invRepo, transferService)

	server := &MCPServer{
		pool:                 pool,
		tenantID:             tenantID,
		logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		accountRepo:          accountRepo,
		txService:            txService,
		invRepo:              invRepo,
		snapRepo:             snapRepo,
		pendingActionService: pendingActionService,
	}

	callResp := runMCPRequest(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "propose_categorization",
			"arguments": map[string]any{
				"transaction_id": tx.ID.String(),
				"category_id":    cat.ID.String(),
				"reason":         "Transaction description contains Pao de Acucar",
			},
		},
	})
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

	// Verify that hydrated pending action exists in database
	actions, err := actionRepo.ListPending(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, tx.ID.String(), actions[0].Payload["transaction_id"])
	require.Equal(t, cat.ID.String(), actions[0].Payload["category_id"])
	require.Equal(t, "Transaction description contains Pao de Acucar", actions[0].Payload["reason"])
	require.Equal(t, "SUPERMERCADO PAO DE ACUCAR", actions[0].Payload["description"])
	require.Equal(t, tx.CleanDescription, actions[0].Payload["clean_description"])
	require.Equal(t, "-145.90", actions[0].Payload["amount"])
	require.Equal(t, "2026-11-10", actions[0].Payload["date"])
	require.Equal(t, cat.Name, actions[0].Payload["category_name"])
}
