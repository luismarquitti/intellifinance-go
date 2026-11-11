package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id"`
	Result  any         `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPServer struct {
	pool        *pgxpool.Pool
	tenantID    uuid.UUID
	logger      *slog.Logger
	accountRepo ports.AccountRepository
	txService   ports.TransactionService
	invRepo     ports.InvoiceRepository
	snapRepo    ports.ProviderBalanceSnapshotRepository
	actionRepo  ports.PendingActionRepository
}

func main() {
	// Standard error logger so stdout remains clean for MCP JSON-RPC
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL is required to run intellifinance MCP server")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed connecting to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	tenantID := cfg.DevTenantID
	if envTenant := os.Getenv("INTELLIFINANCE_TENANT_ID"); envTenant != "" {
		if parsed, pErr := uuid.Parse(envTenant); pErr == nil {
			tenantID = parsed
		}
	}
	if tenantID == uuid.Nil {
		// Discover first tenant
		_ = pool.QueryRow(ctx, "SELECT id FROM tenants ORDER BY created_at ASC LIMIT 1").Scan(&tenantID)
	}

	accountRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	invRepo := postgres.NewInvoiceRepository(pool)
	snapRepo := postgres.NewProviderBalanceSnapshotRepository(pool)
	actionRepo := postgres.NewPendingActionRepository(pool)

	txService := app.NewTransactionService(txRepo)

	server := &MCPServer{
		pool:        pool,
		tenantID:    tenantID,
		logger:      logger,
		accountRepo: accountRepo,
		txService:   txService,
		invRepo:     invRepo,
		snapRepo:    snapRepo,
		actionRepo:  actionRepo,
	}

	server.Run(os.Stdin, os.Stdout)
}

func (s *MCPServer) Run(reader io.Reader, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	// Buffer up to 10MB per message
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(writer, nil, -32700, "Parse error")
			continue
		}

		resp := s.handleRequest(context.Background(), req)
		if resp != nil {
			respBytes, _ := json.Marshal(resp)
			_, _ = fmt.Fprintf(writer, "%s\n", string(respBytes))
		}
	}
}

func (s *MCPServer) sendError(w io.Writer, id any, code int, msg string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	}
	data, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(w, "%s\n", string(data))
}

func (s *MCPServer) handleRequest(ctx context.Context, req JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]any{
					"name":    "intellifinance-mcp",
					"version": "1.0.0",
				},
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
			},
		}

	case "notifications/initialized":
		return nil

	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": s.getToolDefinitions(),
			},
		}

	case "tools/call":
		var callParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &RPCError{Code: -32602, Message: "Invalid parameters"},
			}
		}

		resultText, err := s.executeTool(ctx, callParams.Name, callParams.Arguments)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
					},
					"isError": true,
				},
			}
		}

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": resultText},
				},
			},
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("Method '%s' not found", req.Method)},
		}
	}
}

func (s *MCPServer) getToolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "list_accounts",
			"description": "Lists all financial accounts registered in the user tenant with balances, types, and institutions.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "get_balances",
			"description": "Retrieves the ledger balance alongside the latest Open Finance external provider balance snapshot for all accounts.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "list_transactions",
			"description": "Retrieves transactions with optional filters by account ID, limit and category.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"account_id": map[string]any{"type": "string", "description": "Optional account UUID filter"},
					"limit":      map[string]any{"type": "integer", "description": "Maximum number of transactions to return (default 50)"},
				},
			},
		},
		{
			"name":        "list_invoices",
			"description": "Lists credit card invoice cycles, due dates, minimum payments and closure statuses.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"account_id": map[string]any{"type": "string", "description": "Account UUID for the credit card"},
				},
				"required": []string{"account_id"},
			},
		},
		{
			"name":        "list_pending_actions",
			"description": "Lists all pending mutations awaiting Human-in-the-Loop review and approval.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "propose_categorization",
			"description": "Proposes a category for an uncategorized transaction through Human-in-the-Loop governance (never mutates directly).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"transaction_id": map[string]any{"type": "string", "description": "Transaction UUID"},
					"category_id":    map[string]any{"type": "string", "description": "Proposed category UUID"},
					"reason":         map[string]any{"type": "string", "description": "AI reasoning or rationale for proposal"},
				},
				"required": []string{"transaction_id", "category_id"},
			},
		},
	}
}

func (s *MCPServer) executeTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "list_accounts":
		accounts, err := s.accountRepo.List(ctx, s.tenantID, ports.AccountFilter{})
		if err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(accounts, "", "  ")
		return string(data), nil

	case "get_balances":
		accounts, err := s.accountRepo.List(ctx, s.tenantID, ports.AccountFilter{})
		if err != nil {
			return "", err
		}

		type balanceComparison struct {
			AccountID         uuid.UUID `json:"account_id"`
			Name              string    `json:"name"`
			Type              string    `json:"type"`
			LedgerBalance     string    `json:"ledger_balance"`
			ProviderBalance   string    `json:"provider_balance,omitempty"`
			ObservedAt        string    `json:"observed_at,omitempty"`
			ReconciledDiff    string    `json:"reconciled_diff,omitempty"`
		}

		var comparisons []balanceComparison
		for _, acc := range accounts {
			comp := balanceComparison{
				AccountID:     acc.ID,
				Name:          acc.Name,
				Type:          string(acc.Type),
				LedgerBalance: acc.Balance.StringFixed(2),
			}
			snap, snapErr := s.snapRepo.GetLatestByAccount(ctx, s.tenantID, acc.ID)
			if snapErr == nil && snap != nil {
				comp.ProviderBalance = snap.Balance.StringFixed(2)
				comp.ObservedAt = snap.ObservedAt.Format(time.RFC3339)
				diff := acc.Balance.Sub(snap.Balance)
				comp.ReconciledDiff = diff.StringFixed(2)
			}
			comparisons = append(comparisons, comp)
		}
		data, _ := json.MarshalIndent(comparisons, "", "  ")
		return string(data), nil

	case "list_transactions":
		var params struct {
			AccountID string `json:"account_id"`
			Limit     int    `json:"limit"`
		}
		if len(args) > 0 {
			_ = json.Unmarshal(args, &params)
		}
		limit := params.Limit
		if limit <= 0 {
			limit = 50
		}

		filter := ports.TransactionFilter{
			Limit: limit,
		}
		if params.AccountID != "" {
			if parsed, pErr := uuid.Parse(params.AccountID); pErr == nil {
				filter.AccountID = &parsed
			}
		}

		txs, _, err := s.txService.List(ctx, s.tenantID, filter)
		if err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(txs, "", "  ")
		return string(data), nil

	case "list_invoices":
		var params struct {
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", errors.New("invalid arguments for list_invoices")
		}
		accountID, err := uuid.Parse(params.AccountID)
		if err != nil {
			return "", fmt.Errorf("invalid account_id: %w", err)
		}

		invoices, err := s.invRepo.ListByAccount(ctx, s.tenantID, accountID)
		if err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(invoices, "", "  ")
		return string(data), nil

	case "list_pending_actions":
		actions, err := s.actionRepo.ListPending(ctx, s.tenantID)
		if err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(actions, "", "  ")
		return string(data), nil

	case "propose_categorization":
		var params struct {
			TransactionID string `json:"transaction_id"`
			CategoryID    string `json:"category_id"`
			Reason        string `json:"reason"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", errors.New("invalid arguments for propose_categorization")
		}
		txID, err := uuid.Parse(params.TransactionID)
		if err != nil {
			return "", fmt.Errorf("invalid transaction_id: %w", err)
		}
		catID, err := uuid.Parse(params.CategoryID)
		if err != nil {
			return "", fmt.Errorf("invalid category_id: %w", err)
		}

		action, err := domain.NewPendingAction(
			s.tenantID,
			"mcp-server",
			domain.ActionTypeCategorizeTransaction,
			map[string]any{
				"transaction_id": txID.String(),
				"category_id":    catID.String(),
				"reason":         params.Reason,
			},
		)
		if err != nil {
			return "", fmt.Errorf("failed creating pending action proposal: %w", err)
		}

		if err := s.actionRepo.Save(ctx, action); err != nil {
			return "", fmt.Errorf("failed persisting pending action: %w", err)
		}

		return fmt.Sprintf("Proposal recorded successfully. PendingAction ID: %s (Status: PENDING). A human review is required before changes take effect.", action.ID), nil

	default:
		return "", fmt.Errorf("unknown tool '%s'", name)
	}
}
