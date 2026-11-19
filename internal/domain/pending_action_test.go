package domain_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPendingAction_ValidationAndDecisions(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()

	// Missing tenant
	_, err := domain.NewPendingAction(uuid.Nil, "mcp-server", domain.ActionTypeCategorizeTransaction, nil)
	require.Error(t, err)

	// Valid creation
	action, err := domain.NewPendingAction(
		tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction,
		map[string]any{"transaction_id": targetID.String(), "suggested_category_id": uuid.New().String()},
	)
	require.NoError(t, err)
	require.Equal(t, domain.ActionStatusPending, action.Status)

	// Approve
	decider := uuid.New()
	require.NoError(t, action.Approve(decider))
	require.Equal(t, domain.ActionStatusApproved, action.Status)
	require.Equal(t, &decider, action.DecidedByUserID)

	// Re-approving fails
	require.Error(t, action.Approve(decider))

	// Mark executed
	require.NoError(t, action.MarkExecuted())
	require.Equal(t, domain.ActionStatusExecuted, action.Status)
	require.NotNil(t, action.ExecutedAt)

	// Rejection workflow
	action2, err := domain.NewPendingAction(
		tenantID, "mcp-server", domain.ActionTypeSplitTransaction, nil,
	)
	require.NoError(t, err)
	require.NoError(t, action2.Reject(decider, "Incorrect proportions"))
	require.Equal(t, domain.ActionStatusRejected, action2.Status)
	require.Equal(t, "Incorrect proportions", *action2.RejectionReason)
}

func TestPendingAction_LifecycleErrorsAndFKSafety(t *testing.T) {
	tenantID := uuid.New()

	t.Run("NewPendingAction returns domain sentinels on missing tenant or action type", func(t *testing.T) {
		_, err := domain.NewPendingAction(uuid.Nil, "mcp-server", domain.ActionTypeCategorizeTransaction, nil)
		require.ErrorIs(t, err, domain.ErrInvalidID)

		_, err = domain.NewPendingAction(tenantID, "mcp-server", domain.PendingActionType("   "), nil)
		require.ErrorIs(t, err, domain.ErrUnsupportedPendingActionType)
	})

	t.Run("Approve with nil decider preserves nil DecidedByUserID and enforces state transitions", func(t *testing.T) {
		action, err := domain.NewPendingAction(tenantID, "", domain.ActionTypeCategorizeTransaction, nil)
		require.NoError(t, err)
		require.Equal(t, "system", action.CreatedByAgent)

		// Cannot execute before approval
		err = action.MarkExecuted()
		require.ErrorIs(t, err, domain.ErrPendingActionNotApproved)

		// Approve without user UUID leaves DecidedByUserID nil for FK safety
		require.NoError(t, action.Approve(uuid.Nil))
		require.Equal(t, domain.ActionStatusApproved, action.Status)
		require.Nil(t, action.DecidedByUserID)
		require.NotNil(t, action.DecidedAt)

		// Re-approve or reject after approval fails with ErrPendingActionNotPending
		require.ErrorIs(t, action.Approve(uuid.New()), domain.ErrPendingActionNotPending)
		require.ErrorIs(t, action.Reject(uuid.New(), "too late"), domain.ErrPendingActionNotPending)

		// Execute succeeds
		require.NoError(t, action.MarkExecuted())
		require.Equal(t, domain.ActionStatusExecuted, action.Status)
		require.ErrorIs(t, action.MarkExecuted(), domain.ErrPendingActionNotApproved)
	})

	t.Run("Reject trims reason and preserves nil DecidedByUserID on uuid.Nil", func(t *testing.T) {
		blankAction, err := domain.NewPendingAction(tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction, nil)
		require.NoError(t, err)

		require.NoError(t, blankAction.Reject(uuid.Nil, "   \t\n "))
		require.Equal(t, domain.ActionStatusRejected, blankAction.Status)
		require.Nil(t, blankAction.DecidedByUserID)
		require.Nil(t, blankAction.RejectionReason)
		require.ErrorIs(t, blankAction.Reject(uuid.New(), "again"), domain.ErrPendingActionNotPending)

		reasonAction, err := domain.NewPendingAction(tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction, nil)
		require.NoError(t, err)
		decider := uuid.New()
		require.NoError(t, reasonAction.Reject(decider, "  wrong category  "))
		require.NotNil(t, reasonAction.DecidedByUserID)
		require.Equal(t, decider, *reasonAction.DecidedByUserID)
		require.NotNil(t, reasonAction.RejectionReason)
		require.Equal(t, "wrong category", *reasonAction.RejectionReason)
	})
}

func TestCategorizeTransactionProposal_ValidationAndHelpers(t *testing.T) {
	txID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	catID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	valid := func() domain.CategorizeTransactionProposal {
		return domain.CategorizeTransactionProposal{
			TransactionID:    txID,
			Description:      "UBER *TRIP",
			CleanDescription: "UBER TRIP",
			Amount:           decimal.RequireFromString("-42.50"),
			Date:             "2026-10-15",
			CategoryID:       catID,
			CategoryName:     "Transporte",
			Confidence:       decimal.RequireFromString("0.92"),
			Reason:           "Matched keyword UBER",
		}
	}

	t.Run("valid expense and income proposals pass and expose deterministic dedup key", func(t *testing.T) {
		p := valid()
		require.NoError(t, p.ValidateHeader())
		require.NoError(t, p.Validate())
		require.Equal(t, "categorize_11111111-1111-1111-1111-111111111111", p.DeduplicationKey())
		require.False(t, p.NeedsHydration())

		pIDOnly := domain.CategorizeTransactionProposal{
			TransactionID: txID,
			CategoryID:    catID,
			Confidence:    decimal.RequireFromString("0.85"),
		}
		require.NoError(t, pIDOnly.ValidateHeader())
		require.ErrorIs(t, pIDOnly.Validate(), domain.ErrInvalidAmount)

		pIncome := valid()
		pIncome.Amount = decimal.RequireFromString("5500.00")
		pIncome.Confidence = decimal.Zero
		require.NoError(t, pIncome.Validate())

		pMaxConf := valid()
		pMaxConf.Confidence = decimal.NewFromInt(1)
		require.NoError(t, pMaxConf.Validate())
	})

	t.Run("validation failures wrap ErrInvalidPendingActionPayload and specific sentinel", func(t *testing.T) {
		cases := []struct {
			name        string
			mutate      func(p *domain.CategorizeTransactionProposal)
			expectedErr error
		}{
			{
				name:        "nil transaction_id",
				mutate:      func(p *domain.CategorizeTransactionProposal) { p.TransactionID = uuid.Nil },
				expectedErr: domain.ErrInvalidID,
			},
			{
				name:        "nil category_id",
				mutate:      func(p *domain.CategorizeTransactionProposal) { p.CategoryID = uuid.Nil },
				expectedErr: domain.ErrInvalidID,
			},
			{
				name:        "zero amount",
				mutate:      func(p *domain.CategorizeTransactionProposal) { p.Amount = decimal.Zero },
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name:        "sub-cent amount",
				mutate:      func(p *domain.CategorizeTransactionProposal) { p.Amount = decimal.RequireFromString("-12.345") },
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "amount exceeds MaxMonetaryAmount",
				mutate: func(p *domain.CategorizeTransactionProposal) {
					p.Amount = decimal.RequireFromString("-10000000000000.00")
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name:        "negative confidence",
				mutate:      func(p *domain.CategorizeTransactionProposal) { p.Confidence = decimal.RequireFromString("-0.01") },
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name:        "confidence above one",
				mutate:      func(p *domain.CategorizeTransactionProposal) { p.Confidence = decimal.RequireFromString("1.01") },
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := valid()
				tc.mutate(&p)
				err := p.Validate()
				require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
				require.ErrorIs(t, err, tc.expectedErr)
			})
		}
	})

	t.Run("NeedsHydration detects missing display context fields", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(p *domain.CategorizeTransactionProposal)
		}{
			{name: "blank description", mutate: func(p *domain.CategorizeTransactionProposal) { p.Description = "   " }},
			{name: "blank category name", mutate: func(p *domain.CategorizeTransactionProposal) { p.CategoryName = "" }},
			{name: "zero amount", mutate: func(p *domain.CategorizeTransactionProposal) { p.Amount = decimal.Zero }},
			{name: "blank date", mutate: func(p *domain.CategorizeTransactionProposal) { p.Date = " " }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := valid()
				tc.mutate(&p)
				require.True(t, p.NeedsHydration())
			})
		}
	})
}

func TestNewCategorizeProposalFromEntities(t *testing.T) {
	tenantID := uuid.New()
	otherTenantID := uuid.New()
	accountID := uuid.New()

	cat, err := domain.NewCategory(tenantID, nil, "  Alimentação  ", domain.TransactionTypeExpense, false)
	require.NoError(t, err)

	// 2026-10-16 01:30 UTC is 2026-10-15 22:30 in America/Sao_Paulo (-03:00)
	txTime := time.Date(2026, 10, 16, 1, 30, 0, 0, time.UTC)
	tx := &domain.Transaction{
		ID:          uuid.New(),
		TenantID:    tenantID,
		AccountID:   accountID,
		Date:        txTime,
		Amount:      decimal.RequireFromString("-89.90"),
		Description: "  COMPRA CARTAO - PADARIA CENTRAL 15/10  ",
		Type:        domain.TransactionTypeExpense,
		Status:      domain.TransactionStatusCompleted,
	}

	t.Run("builds normalized proposal with Brazil date and clean description fallback", func(t *testing.T) {
		prop, propErr := domain.NewCategorizeProposalFromEntities(
			tx, cat, decimal.RequireFromString("0.85"), "  Regra de padaria  ",
		)
		require.NoError(t, propErr)
		require.Equal(t, tx.ID, prop.TransactionID)
		require.Equal(t, "COMPRA CARTAO - PADARIA CENTRAL 15/10", prop.Description)
		require.Equal(t, "PADARIA CENTRAL", prop.CleanDescription)
		require.True(t, decimal.RequireFromString("-89.90").Equal(prop.Amount))
		require.Equal(t, "2026-10-15", prop.Date)
		require.Equal(t, cat.ID, prop.CategoryID)
		require.Equal(t, "Alimentação", prop.CategoryName)
		require.True(t, decimal.RequireFromString("0.85").Equal(prop.Confidence))
		require.Equal(t, "Regra de padaria", prop.Reason)
	})

	t.Run("rejects invalid entities and states", func(t *testing.T) {
		voidTx := *tx
		voidTx.Status = domain.TransactionStatusVoid

		transferTx := *tx
		transferTx.Type = domain.TransactionTypeTransfer

		otherTenantCat := *cat
		otherTenantCat.TenantID = otherTenantID

		cases := []struct {
			name        string
			tx          *domain.Transaction
			cat         *domain.Category
			confidence  decimal.Decimal
			expectedErr error
		}{
			{
				name:        "nil transaction",
				tx:          nil,
				cat:         cat,
				confidence:  decimal.RequireFromString("0.90"),
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name:        "nil category",
				tx:          tx,
				cat:         nil,
				confidence:  decimal.RequireFromString("0.90"),
				expectedErr: domain.ErrInvalidID,
			},
			{
				name:        "cross-tenant mismatch",
				tx:          tx,
				cat:         &otherTenantCat,
				confidence:  decimal.RequireFromString("0.90"),
				expectedErr: domain.ErrUnauthorized,
			},
			{
				name:        "void transaction",
				tx:          &voidTx,
				cat:         cat,
				confidence:  decimal.RequireFromString("0.90"),
				expectedErr: domain.ErrInvalidStatus,
			},
			{
				name:        "transfer transaction",
				tx:          &transferTx,
				cat:         cat,
				confidence:  decimal.RequireFromString("0.90"),
				expectedErr: domain.ErrInvalidTransactionType,
			},
			{
				name:        "invalid confidence",
				tx:          tx,
				cat:         cat,
				confidence:  decimal.RequireFromString("1.50"),
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, buildErr := domain.NewCategorizeProposalFromEntities(tc.tx, tc.cat, tc.confidence, "reason")
				require.ErrorIs(t, buildErr, tc.expectedErr)
			})
		}
	})
}

func TestCategorizePendingAction_CodecAndRoundTrip(t *testing.T) {
	tenantID := uuid.New()
	txID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	catID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	proposal := domain.CategorizeTransactionProposal{
		TransactionID:    txID,
		Description:      "  PIX ENVIADO - FARMACIA SAUDE  ",
		CleanDescription: "  FARMACIA SAUDE  ",
		Amount:           decimal.RequireFromString("-129.90"),
		Date:             "2026-10-15",
		CategoryID:       catID,
		CategoryName:     "  Saúde  ",
		Confidence:       decimal.RequireFromString("0.88"),
		Reason:           "  Despesa de farmácia  ",
	}

	t.Run("constructs frontend-compatible payload and metadata and round-trips through JSON", func(t *testing.T) {
		action, err := domain.NewCategorizePendingAction(tenantID, "ai_categorization_agent", proposal)
		require.NoError(t, err)
		require.Equal(t, domain.ActionStatusPending, action.Status)
		require.Equal(t, domain.ActionTypeCategorizeTransaction, action.ActionType)
		require.Equal(t, "categorize_11111111-1111-1111-1111-111111111111", action.Metadata["deduplication_key"])
		require.Equal(t, "0.88", action.Metadata["confidence"])

		require.Equal(t, txID.String(), action.Payload["transaction_id"])
		require.Equal(t, "PIX ENVIADO - FARMACIA SAUDE", action.Payload["description"])
		require.Equal(t, "FARMACIA SAUDE", action.Payload["clean_description"])
		require.Equal(t, "-129.90", action.Payload["amount"])
		require.Equal(t, "2026-10-15", action.Payload["date"])
		require.Equal(t, catID.String(), action.Payload["category_id"])
		require.Equal(t, "Saúde", action.Payload["category_name"])
		require.Equal(t, "0.88", action.Payload["confidence"])
		require.Equal(t, "Despesa de farmácia", action.Payload["reason"])

		rawJSON, marshalErr := json.Marshal(action)
		require.NoError(t, marshalErr)

		var decodedAction domain.PendingAction
		require.NoError(t, json.Unmarshal(rawJSON, &decodedAction))

		decoded, decErr := domain.DecodeCategorizePayload(&decodedAction)
		require.NoError(t, decErr)
		require.Equal(t, txID, decoded.TransactionID)
		require.Equal(t, "PIX ENVIADO - FARMACIA SAUDE", decoded.Description)
		require.Equal(t, "FARMACIA SAUDE", decoded.CleanDescription)
		require.True(t, decimal.RequireFromString("-129.90").Equal(decoded.Amount))
		require.Equal(t, "2026-10-15", decoded.Date)
		require.Equal(t, catID, decoded.CategoryID)
		require.Equal(t, "Saúde", decoded.CategoryName)
		require.True(t, decimal.RequireFromString("0.88").Equal(decoded.Confidence))
		require.Equal(t, "Despesa de farmácia", decoded.Reason)
	})

	t.Run("NewCategorizePendingAction rejects invalid tenant or proposal", func(t *testing.T) {
		_, err := domain.NewCategorizePendingAction(uuid.Nil, "agent", proposal)
		require.ErrorIs(t, err, domain.ErrInvalidID)

		bad := proposal
		bad.Amount = decimal.Zero
		_, err = domain.NewCategorizePendingAction(tenantID, "agent", bad)
		require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
		require.ErrorIs(t, err, domain.ErrInvalidAmount)
	})

	t.Run("DecodeCategorizePayload supports legacy float confidence with exact amounts and rejects float64 currency", func(t *testing.T) {
		legacyFloatConfRaw := []byte(`{
			"transaction_id": "11111111-1111-1111-1111-111111111111",
			"category_id": "22222222-2222-2222-2222-222222222222",
			"confidence": 0.92,
			"amount": "-42.50",
			"reason": "Legacy float confidence row"
		}`)

		// Unmarshal via standard json (confidence -> float64, amount -> string)
		var floatConfMap map[string]any
		require.NoError(t, json.Unmarshal(legacyFloatConfRaw, &floatConfMap))
		floatConfAct, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, floatConfMap)
		require.NoError(t, err)
		gotFloatConf, err := domain.DecodeCategorizePayload(floatConfAct)
		require.NoError(t, err)
		require.True(t, decimal.RequireFromString("0.92").Equal(gotFloatConf.Confidence))
		require.True(t, decimal.RequireFromString("-42.50").Equal(gotFloatConf.Amount))

		// Unmarshal numeric JSON with UseNumber (produces json.Number)
		legacyNumericRaw := []byte(`{
			"transaction_id": "11111111-1111-1111-1111-111111111111",
			"category_id": "22222222-2222-2222-2222-222222222222",
			"confidence": 0.92,
			"amount": -42.5,
			"reason": "UseNumber numeric row"
		}`)
		var numMap map[string]any
		dec := json.NewDecoder(bytes.NewReader(legacyNumericRaw))
		dec.UseNumber()
		require.NoError(t, dec.Decode(&numMap))
		numAct, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, numMap)
		require.NoError(t, err)
		gotNum, err := domain.DecodeCategorizePayload(numAct)
		require.NoError(t, err)
		require.True(t, decimal.RequireFromString("0.92").Equal(gotNum.Confidence))
		require.True(t, decimal.RequireFromString("-42.50").Equal(gotNum.Amount))

		// Standard json.Unmarshal on numeric amount produces float64 and must fail closed
		var floatAmtMap map[string]any
		require.NoError(t, json.Unmarshal(legacyNumericRaw, &floatAmtMap))
		floatAmtAct, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, floatAmtMap)
		require.NoError(t, err)
		_, err = domain.DecodeCategorizePayload(floatAmtAct)
		require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
		require.ErrorIs(t, err, domain.ErrInvalidAmount)

		// Integer confidence and omitted amount (old MCP proposal)
		mcpAct, err := domain.NewPendingAction(tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction, map[string]any{
			"transaction_id": txID.String(),
			"category_id":    catID.String(),
			"confidence":     1,
			"reason":         "MCP ID-only",
		})
		require.NoError(t, err)
		gotMCP, err := domain.DecodeCategorizePayload(mcpAct)
		require.NoError(t, err)
		require.True(t, decimal.NewFromInt(1).Equal(gotMCP.Confidence))
		require.True(t, gotMCP.Amount.IsZero())
		require.True(t, gotMCP.NeedsHydration())

		// Int64 amount
		int64Act, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, map[string]any{
			"transaction_id": txID.String(),
			"category_id":    catID.String(),
			"amount":         int64(-100),
		})
		require.NoError(t, err)
		gotInt64, err := domain.DecodeCategorizePayload(int64Act)
		require.NoError(t, err)
		require.True(t, decimal.NewFromInt(-100).Equal(gotInt64.Amount))

		// Decimal amount
		decAct, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, map[string]any{
			"transaction_id": txID.String(),
			"category_id":    catID.String(),
			"amount":         decimal.RequireFromString("-55.25"),
		})
		require.NoError(t, err)
		gotDec, err := domain.DecodeCategorizePayload(decAct)
		require.NoError(t, err)
		require.True(t, decimal.RequireFromString("-55.25").Equal(gotDec.Amount))
	})

	t.Run("DecodeCategorizePayload fails closed on corrupt or invalid payloads", func(t *testing.T) {
		cases := []struct {
			name        string
			action      *domain.PendingAction
			expectedErr error
		}{
			{name: "nil action", action: nil, expectedErr: domain.ErrInvalidPendingActionPayload},
			{
				name: "wrong action type",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String()},
				},
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name: "nil payload",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    nil,
				},
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name: "missing transaction_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"category_id": catID.String()},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "nil UUID transaction_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": uuid.Nil.String(), "category_id": catID.String()},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "invalid transaction_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": "not-a-uuid", "category_id": catID.String()},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "missing category_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String()},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "nil UUID category_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": uuid.Nil.String()},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "float64 amount",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String(), "amount": -42.5},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "zero amount string",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String(), "amount": "0.00"},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "sub-cent amount",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String(), "amount": "-10.125"},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "overflow amount",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String(), "amount": "10000000000000.00"},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "non-numeric amount",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String(), "amount": "abc"},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "negative confidence",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String(), "confidence": "-0.1"},
				},
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name: "confidence over 1",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String(), "confidence": "1.01"},
				},
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name: "non-numeric confidence",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload:    map[string]any{"transaction_id": txID.String(), "category_id": catID.String(), "confidence": true},
				},
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := domain.DecodeCategorizePayload(tc.action)
				require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
				require.ErrorIs(t, err, tc.expectedErr)
			})
		}
	})
}

func TestReconcileTransferProposal_ValidationAndEntityBuilder(t *testing.T) {
	outflowID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	inflowID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	srcAccID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	dstAccID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	invoiceID := uuid.MustParse("77777777-7777-7777-7777-777777777777")

	t.Run("ReconciliationType.IsValid", func(t *testing.T) {
		require.True(t, domain.ReconciliationTypeInternalTransfer.IsValid())
		require.True(t, domain.ReconciliationTypeBillPayment.IsValid())
		require.False(t, domain.ReconciliationType("").IsValid())
		require.False(t, domain.ReconciliationType("OTHER").IsValid())
	})

	valid := func() domain.ReconcileTransferProposal {
		return domain.ReconcileTransferProposal{
			OutflowTransactionID:   outflowID,
			InflowTransactionID:    inflowID,
			Amount:                 decimal.RequireFromString("1250.50"),
			SourceAccountID:        srcAccID,
			SourceAccountName:      "Itaú Checking",
			DestinationAccountID:   dstAccID,
			DestinationAccountName: "Nubank",
			OutflowDate:            "2026-10-14",
			InflowDate:             "2026-10-15",
			ReconciliationType:     domain.ReconciliationTypeInternalTransfer,
			Description:            "Transferência de R$ 1250.50 de Itaú Checking para Nubank",
		}
	}

	t.Run("valid proposal passes and generates deterministic deduplication key", func(t *testing.T) {
		p := valid()
		require.NoError(t, p.Validate())
		require.Equal(t, "reconcile_33333333-3333-3333-3333-333333333333_44444444-4444-4444-4444-444444444444", p.DeduplicationKey())

		// Partial legacy proposal with nil account IDs passes if legs and amount are valid
		p.SourceAccountID = uuid.Nil
		p.DestinationAccountID = uuid.Nil
		require.NoError(t, p.Validate())
	})

	t.Run("Validate rejects invalid fields with ErrInvalidPendingActionPayload and specific sentinel", func(t *testing.T) {
		nilUUID := uuid.Nil
		cases := []struct {
			name        string
			mutate      func(p *domain.ReconcileTransferProposal)
			expectedErr error
		}{
			{
				name:        "nil outflow transaction id",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.OutflowTransactionID = uuid.Nil },
				expectedErr: domain.ErrInvalidID,
			},
			{
				name:        "nil inflow transaction id",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.InflowTransactionID = uuid.Nil },
				expectedErr: domain.ErrInvalidID,
			},
			{
				name:        "identical leg transaction ids",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.InflowTransactionID = p.OutflowTransactionID },
				expectedErr: domain.ErrInvalidTransferPair,
			},
			{
				name:        "same non-nil accounts",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.DestinationAccountID = p.SourceAccountID },
				expectedErr: domain.ErrTransferSameAccount,
			},
			{
				name:        "zero amount",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.Amount = decimal.Zero },
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name:        "negative amount",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.Amount = decimal.RequireFromString("-100.00") },
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name:        "sub-cent amount",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.Amount = decimal.RequireFromString("100.001") },
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "amount overflow",
				mutate: func(p *domain.ReconcileTransferProposal) {
					p.Amount = decimal.RequireFromString("10000000000000.00")
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name:        "invalid reconciliation type",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.ReconciliationType = domain.ReconciliationType("BOGUS") },
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name:        "nil UUID pointer for invoice_id",
				mutate:      func(p *domain.ReconcileTransferProposal) { p.InvoiceID = &nilUUID },
				expectedErr: domain.ErrInvalidID,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := valid()
				tc.mutate(&p)
				err := p.Validate()
				require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
				require.ErrorIs(t, err, tc.expectedErr)
			})
		}
	})

	t.Run("NewReconcileTransferProposalFromEntities builds internal transfer and bill payment proposals", func(t *testing.T) {
		tenantID := uuid.New()
		outflow := &domain.Transaction{
			ID:        outflowID,
			TenantID:  tenantID,
			AccountID: srcAccID,
			Date:      time.Date(2026, 10, 15, 2, 0, 0, 0, time.UTC), // 2026-10-14 23:00 BRT
			Amount:    decimal.RequireFromString("-450.00"),
			Status:    domain.TransactionStatusCompleted,
		}
		inflow := &domain.Transaction{
			ID:        inflowID,
			TenantID:  tenantID,
			AccountID: dstAccID,
			Date:      time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), // pure calendar date
			Amount:    decimal.RequireFromString("450.00"),
			Status:    domain.TransactionStatusCompleted,
		}
		checkingSrc := &domain.Account{ID: srcAccID, TenantID: tenantID, Name: "  Itaú CC  ", Type: domain.AccountTypeChecking}
		savingsDst := &domain.Account{ID: dstAccID, TenantID: tenantID, Name: "  Poupança  ", Type: domain.AccountTypeSavings}
		cardDst := &domain.Account{ID: dstAccID, TenantID: tenantID, Name: "Itaú Black", Type: domain.AccountTypeCreditCard}

		// Internal transfer with explicit accounts
		internalProp, err := domain.NewReconcileTransferProposalFromEntities(outflow, inflow, checkingSrc, savingsDst, nil)
		require.NoError(t, err)
		require.Equal(t, domain.ReconciliationTypeInternalTransfer, internalProp.ReconciliationType)
		require.Equal(t, "Itaú CC", internalProp.SourceAccountName)
		require.Equal(t, "Poupança", internalProp.DestinationAccountName)
		require.Equal(t, "2026-10-14", internalProp.OutflowDate)
		require.Equal(t, "2026-10-15", internalProp.InflowDate)
		require.True(t, decimal.RequireFromString("450.00").Equal(internalProp.Amount))
		require.Equal(t, "Transferência de R$ 450.00 de Itaú CC para Poupança", internalProp.Description)
		require.Nil(t, internalProp.InvoiceID)

		// Fallback account names when accounts are nil
		fallbackProp, err := domain.NewReconcileTransferProposalFromEntities(outflow, inflow, nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, "Conta de Origem", fallbackProp.SourceAccountName)
		require.Equal(t, "Conta de Destino", fallbackProp.DestinationAccountName)
		require.Equal(t, "Transferência de R$ 450.00 de Conta de Origem para Conta de Destino", fallbackProp.Description)

		// Bill payment triggered by credit card destination account and invoiceID copy
		invCopy := invoiceID
		billProp, err := domain.NewReconcileTransferProposalFromEntities(outflow, inflow, checkingSrc, cardDst, &invCopy)
		require.NoError(t, err)
		invCopy = uuid.Nil // mutating caller's UUID must not mutate proposal.InvoiceID
		require.Equal(t, domain.ReconciliationTypeBillPayment, billProp.ReconciliationType)
		require.NotNil(t, billProp.InvoiceID)
		require.Equal(t, invoiceID, *billProp.InvoiceID)
		require.Equal(t, "Pagamento de fatura do Itaú Black (R$ 450.00) a partir de Itaú CC", billProp.Description)

		// Bill payment triggered by invoiceID even when dstAcc is nil
		billFromInvOnly, err := domain.NewReconcileTransferProposalFromEntities(outflow, inflow, checkingSrc, nil, &invoiceID)
		require.NoError(t, err)
		require.Equal(t, domain.ReconciliationTypeBillPayment, billFromInvOnly.ReconciliationType)
		require.Equal(t, "Pagamento de fatura do Conta de Destino (R$ 450.00) a partir de Itaú CC", billFromInvOnly.Description)
	})

	t.Run("NewReconcileTransferProposalFromEntities rejects invalid pairs", func(t *testing.T) {
		tenantA := uuid.New()
		tenantB := uuid.New()
		baseOut := &domain.Transaction{
			ID:        outflowID,
			TenantID:  tenantA,
			AccountID: srcAccID,
			Date:      time.Now().UTC(),
			Amount:    decimal.RequireFromString("-200.00"),
			Status:    domain.TransactionStatusCompleted,
		}
		baseIn := &domain.Transaction{
			ID:        inflowID,
			TenantID:  tenantA,
			AccountID: dstAccID,
			Date:      time.Now().UTC(),
			Amount:    decimal.RequireFromString("200.00"),
			Status:    domain.TransactionStatusCompleted,
		}
		nilUUID := uuid.Nil

		cases := []struct {
			name        string
			out         *domain.Transaction
			in          *domain.Transaction
			invID       *uuid.UUID
			expectedErr error
		}{
			{name: "nil outflow", out: nil, in: baseIn, expectedErr: domain.ErrInvalidTransferPair},
			{name: "nil inflow", out: baseOut, in: nil, expectedErr: domain.ErrInvalidTransferPair},
			{
				name: "nil leg ID",
				out: &domain.Transaction{
					ID: uuid.Nil, TenantID: tenantA, AccountID: srcAccID, Amount: baseOut.Amount, Status: baseOut.Status,
				},
				in:          baseIn,
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "same leg ID",
				out:  baseOut,
				in: &domain.Transaction{
					ID: outflowID, TenantID: tenantA, AccountID: dstAccID, Amount: baseIn.Amount, Status: baseIn.Status,
				},
				expectedErr: domain.ErrInvalidTransferPair,
			},
			{
				name: "same account",
				out:  baseOut,
				in: &domain.Transaction{
					ID: inflowID, TenantID: tenantA, AccountID: srcAccID, Amount: baseIn.Amount, Status: baseIn.Status,
				},
				expectedErr: domain.ErrTransferSameAccount,
			},
			{
				name: "void outflow leg",
				out: &domain.Transaction{
					ID: outflowID, TenantID: tenantA, AccountID: srcAccID, Amount: baseOut.Amount, Status: domain.TransactionStatusVoid,
				},
				in:          baseIn,
				expectedErr: domain.ErrTransferVoidLeg,
			},
			{
				name: "cross-tenant",
				out:  baseOut,
				in: &domain.Transaction{
					ID: inflowID, TenantID: tenantB, AccountID: dstAccID, Amount: baseIn.Amount, Status: baseIn.Status,
				},
				expectedErr: domain.ErrTransferCrossTenant,
			},
			{
				name: "amount mismatch",
				out:  baseOut,
				in: &domain.Transaction{
					ID: inflowID, TenantID: tenantA, AccountID: dstAccID, Amount: decimal.RequireFromString("200.01"), Status: baseIn.Status,
				},
				expectedErr: domain.ErrTransferAmountMismatch,
			},
			{
				name: "swapped signs",
				out: &domain.Transaction{
					ID: outflowID, TenantID: tenantA, AccountID: srcAccID, Amount: decimal.RequireFromString("200.00"), Status: baseOut.Status,
				},
				in: &domain.Transaction{
					ID: inflowID, TenantID: tenantA, AccountID: dstAccID, Amount: decimal.RequireFromString("-200.00"), Status: baseIn.Status,
				},
				expectedErr: domain.ErrTransferAmountMismatch,
			},
			{
				name:        "nil UUID pointer for invoiceID",
				out:         baseOut,
				in:          baseIn,
				invID:       &nilUUID,
				expectedErr: domain.ErrInvalidID,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := domain.NewReconcileTransferProposalFromEntities(tc.out, tc.in, nil, nil, tc.invID)
				require.ErrorIs(t, err, tc.expectedErr)
			})
		}
	})
}

func TestReconcileTransferPendingAction_CodecAndRoundTrip(t *testing.T) {
	tenantID := uuid.New()
	outflowID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	inflowID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	srcAccID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	dstAccID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	invoiceID := uuid.MustParse("77777777-7777-7777-7777-777777777777")

	t.Run("internal transfer omits invoice_id, stamps dedup metadata, and round-trips through JSON", func(t *testing.T) {
		proposal := domain.ReconcileTransferProposal{
			OutflowTransactionID:   outflowID,
			InflowTransactionID:    inflowID,
			Amount:                 decimal.RequireFromString("500.00"),
			SourceAccountID:        srcAccID,
			SourceAccountName:      "  Itaú CC  ",
			DestinationAccountID:   dstAccID,
			DestinationAccountName: "  Poupança  ",
			OutflowDate:            "2026-10-14",
			InflowDate:             "2026-10-15",
			ReconciliationType:     domain.ReconciliationTypeInternalTransfer,
			Description:            "  Transferência de R$ 500.00 de Itaú CC para Poupança  ",
		}

		action, err := domain.NewReconcileTransferPendingAction(tenantID, "transfer-reconciler", proposal)
		require.NoError(t, err)
		require.Equal(t, domain.ActionStatusPending, action.Status)
		require.Equal(t, domain.ActionTypeReconcileTransfer, action.ActionType)
		require.Equal(t, "reconcile_33333333-3333-3333-3333-333333333333_44444444-4444-4444-4444-444444444444", action.Metadata["deduplication_key"])

		_, hasInvoiceKey := action.Payload["invoice_id"]
		require.False(t, hasInvoiceKey)
		require.Equal(t, outflowID.String(), action.Payload["outflow_transaction_id"])
		require.Equal(t, inflowID.String(), action.Payload["inflow_transaction_id"])
		require.Equal(t, "500.00", action.Payload["amount"])
		require.Equal(t, srcAccID.String(), action.Payload["source_account_id"])
		require.Equal(t, "Itaú CC", action.Payload["source_account_name"])
		require.Equal(t, dstAccID.String(), action.Payload["destination_account_id"])
		require.Equal(t, "Poupança", action.Payload["destination_account_name"])
		require.Equal(t, "2026-10-14", action.Payload["outflow_date"])
		require.Equal(t, "2026-10-15", action.Payload["inflow_date"])
		require.Equal(t, "INTERNAL_TRANSFER", action.Payload["reconciliation_type"])
		require.Equal(t, "Transferência de R$ 500.00 de Itaú CC para Poupança", action.Payload["description"])

		rawJSON, marshalErr := json.Marshal(action)
		require.NoError(t, marshalErr)

		var unmarshaled domain.PendingAction
		require.NoError(t, json.Unmarshal(rawJSON, &unmarshaled))

		decoded, decErr := domain.DecodeReconcileTransferPayload(&unmarshaled)
		require.NoError(t, decErr)
		require.Equal(t, outflowID, decoded.OutflowTransactionID)
		require.Equal(t, inflowID, decoded.InflowTransactionID)
		require.True(t, decimal.RequireFromString("500.00").Equal(decoded.Amount))
		require.Equal(t, srcAccID, decoded.SourceAccountID)
		require.Equal(t, "Itaú CC", decoded.SourceAccountName)
		require.Equal(t, dstAccID, decoded.DestinationAccountID)
		require.Equal(t, "Poupança", decoded.DestinationAccountName)
		require.Equal(t, "2026-10-14", decoded.OutflowDate)
		require.Equal(t, "2026-10-15", decoded.InflowDate)
		require.Equal(t, domain.ReconciliationTypeInternalTransfer, decoded.ReconciliationType)
		require.Nil(t, decoded.InvoiceID)
	})

	t.Run("bill payment includes invoice_id and round-trips through JSON", func(t *testing.T) {
		inv := invoiceID
		proposal := domain.ReconcileTransferProposal{
			OutflowTransactionID:   outflowID,
			InflowTransactionID:    inflowID,
			Amount:                 decimal.RequireFromString("2100.45"),
			SourceAccountID:        srcAccID,
			SourceAccountName:      "Itaú CC",
			DestinationAccountID:   dstAccID,
			DestinationAccountName: "Itaú Black",
			OutflowDate:            "2026-10-15",
			InflowDate:             "2026-10-15",
			ReconciliationType:     domain.ReconciliationTypeBillPayment,
			Description:            "Pagamento de fatura do Itaú Black (R$ 2100.45) a partir de Itaú CC",
			InvoiceID:              &inv,
		}

		action, err := domain.NewReconcileTransferPendingAction(tenantID, "transfer-reconciler", proposal)
		require.NoError(t, err)
		require.Equal(t, invoiceID.String(), action.Payload["invoice_id"])

		decoded, decErr := domain.DecodeReconcileTransferPayload(action)
		require.NoError(t, decErr)
		require.Equal(t, domain.ReconciliationTypeBillPayment, decoded.ReconciliationType)
		require.NotNil(t, decoded.InvoiceID)
		require.Equal(t, invoiceID, *decoded.InvoiceID)
	})

	t.Run("DecodeReconcileTransferPayload defaults reconciliation_type when omitted", func(t *testing.T) {
		minimalTransfer := &domain.PendingAction{
			ActionType: domain.ActionTypeReconcileTransfer,
			Payload: map[string]any{
				"outflow_transaction_id": outflowID.String(),
				"inflow_transaction_id":  inflowID.String(),
				"amount":                 "150.00",
			},
		}
		gotTransfer, err := domain.DecodeReconcileTransferPayload(minimalTransfer)
		require.NoError(t, err)
		require.Equal(t, domain.ReconciliationTypeInternalTransfer, gotTransfer.ReconciliationType)
		require.Nil(t, gotTransfer.InvoiceID)

		minimalBill := &domain.PendingAction{
			ActionType: domain.ActionTypeReconcileTransfer,
			Payload: map[string]any{
				"outflow_transaction_id": outflowID.String(),
				"inflow_transaction_id":  inflowID.String(),
				"invoice_id":             invoiceID.String(),
			},
		}
		gotBill, err := domain.DecodeReconcileTransferPayload(minimalBill)
		require.NoError(t, err)
		require.Equal(t, domain.ReconciliationTypeBillPayment, gotBill.ReconciliationType)
		require.NotNil(t, gotBill.InvoiceID)
		require.Equal(t, invoiceID, *gotBill.InvoiceID)
	})

	t.Run("DecodeReconcileTransferPayload fails closed on corrupt or invalid fields", func(t *testing.T) {
		cases := []struct {
			name        string
			action      *domain.PendingAction
			expectedErr error
		}{
			{name: "nil action", action: nil, expectedErr: domain.ErrInvalidPendingActionPayload},
			{
				name: "wrong action type",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeCategorizeTransaction,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
					},
				},
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name: "nil payload",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload:    nil,
				},
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
			{
				name: "missing outflow_transaction_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload:    map[string]any{"inflow_transaction_id": inflowID.String()},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "nil UUID inflow_transaction_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  uuid.Nil.String(),
					},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "equal leg IDs",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  outflowID.String(),
					},
				},
				expectedErr: domain.ErrInvalidTransferPair,
			},
			{
				name: "corrupt source_account_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"source_account_id":      "not-a-uuid",
					},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "same non-nil source and destination accounts",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"source_account_id":      srcAccID.String(),
						"destination_account_id": srcAccID.String(),
					},
				},
				expectedErr: domain.ErrTransferSameAccount,
			},
			{
				name: "corrupt string invoice_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"invoice_id":             "corrupt-invoice-id",
					},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "empty string invoice_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"invoice_id":             "",
					},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "nil UUID invoice_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"invoice_id":             uuid.Nil.String(),
					},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "non-string invoice_id",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"invoice_id":             42,
					},
				},
				expectedErr: domain.ErrInvalidID,
			},
			{
				name: "float64 amount",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"amount":                 42.5,
					},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "negative amount",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"amount":                 "-50.00",
					},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "sub-cent amount",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"amount":                 "10.555",
					},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "overflow amount",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"amount":                 "10000000000000.00",
					},
				},
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name: "invalid reconciliation_type",
				action: &domain.PendingAction{
					ActionType: domain.ActionTypeReconcileTransfer,
					Payload: map[string]any{
						"outflow_transaction_id": outflowID.String(),
						"inflow_transaction_id":  inflowID.String(),
						"reconciliation_type":    "UNKNOWN",
					},
				},
				expectedErr: domain.ErrInvalidPendingActionPayload,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := domain.DecodeReconcileTransferPayload(tc.action)
				require.ErrorIs(t, err, domain.ErrInvalidPendingActionPayload)
				require.ErrorIs(t, err, tc.expectedErr)
			})
		}
	})
}

func TestPendingActionIndex(t *testing.T) {
	tenantID := uuid.New()
	catTxID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	catID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	outflowID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	inflowID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	ignoredTxID := uuid.MustParse("99999999-9999-9999-9999-999999999999")

	// Pending categorization WITHOUT explicit Metadata["deduplication_key"] (tests derived key)
	catPending, err := domain.NewPendingAction(tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction, map[string]any{
		"transaction_id": catTxID.String(),
		"category_id":    catID.String(),
	})
	require.NoError(t, err)

	// Pending reconcile transfer WITHOUT explicit Metadata["deduplication_key"] (tests derived key & legs)
	recPending, err := domain.NewPendingAction(tenantID, "transfer-reconciler", domain.ActionTypeReconcileTransfer, map[string]any{
		"outflow_transaction_id": outflowID.String(),
		"inflow_transaction_id":  inflowID.String(),
	})
	require.NoError(t, err)

	// Approved / Rejected / Executed actions must be ignored by the index
	approvedCat, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, map[string]any{
		"transaction_id": ignoredTxID.String(),
	})
	require.NoError(t, err)
	approvedCat.Metadata["deduplication_key"] = "categorize_ignored"
	require.NoError(t, approvedCat.Approve(uuid.New()))

	rejectedRec, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeReconcileTransfer, map[string]any{
		"outflow_transaction_id": ignoredTxID.String(),
		"inflow_transaction_id":  uuid.New().String(),
	})
	require.NoError(t, err)
	require.NoError(t, rejectedRec.Reject(uuid.New(), "nope"))

	// Custom metadata-only deduplication key
	metaAction, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeSplitTransaction, nil)
	require.NoError(t, err)
	metaAction.Metadata["deduplication_key"] = "custom_dedup_key"

	idx := domain.NewPendingActionIndex([]*domain.PendingAction{
		nil,
		catPending,
		recPending,
		approvedCat,
		rejectedRec,
		metaAction,
	})

	require.True(t, idx.HasCategorizationFor(catTxID))
	require.True(t, idx.HasDeduplicationKey("categorize_11111111-1111-1111-1111-111111111111"))
	require.False(t, idx.HasCategorizationFor(ignoredTxID))
	require.False(t, idx.HasDeduplicationKey("categorize_ignored"))
	require.False(t, idx.HasCategorizationFor(uuid.Nil))

	require.True(t, idx.HasTransferLeg(outflowID))
	require.True(t, idx.HasTransferLeg(inflowID))
	require.True(t, idx.HasDeduplicationKey("reconcile_33333333-3333-3333-3333-333333333333_44444444-4444-4444-4444-444444444444"))
	require.False(t, idx.HasTransferLeg(ignoredTxID))
	require.False(t, idx.HasTransferLeg(uuid.Nil))

	require.True(t, idx.HasDeduplicationKey("  custom_dedup_key  "))
	require.False(t, idx.HasDeduplicationKey("   "))

	// Incremental Record after construction
	lateTxID := uuid.New()
	lateAction, err := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, map[string]any{
		"transaction_id": lateTxID.String(),
	})
	require.NoError(t, err)
	require.False(t, idx.HasCategorizationFor(lateTxID))
	idx.Record(lateAction)
	require.True(t, idx.HasCategorizationFor(lateTxID))

	// Nil index safety
	var nilIdx *domain.PendingActionIndex
	nilIdx.Record(lateAction)
	require.False(t, nilIdx.HasCategorizationFor(catTxID))
	require.False(t, nilIdx.HasTransferLeg(outflowID))
	require.False(t, nilIdx.HasDeduplicationKey("custom_dedup_key"))
}

func TestSelectPendingActions(t *testing.T) {
	tenantID := uuid.New()

	cat1, _ := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, nil)
	rec1, _ := domain.NewPendingAction(tenantID, "rec", domain.ActionTypeReconcileTransfer, nil)
	cat2, _ := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, nil)
	approvedAct, _ := domain.NewPendingAction(tenantID, "ai", domain.ActionTypeCategorizeTransaction, nil)
	require.NoError(t, approvedAct.Approve(uuid.New()))

	input := []*domain.PendingAction{
		nil,
		cat1,
		approvedAct,
		rec1,
		cat1, // duplicate pointer / same ID
		cat2,
	}

	catType := domain.ActionTypeCategorizeTransaction
	recType := domain.ActionTypeReconcileTransfer

	t.Run("All true preserves order, filters non-PENDING and nil, and deduplicates by ID", func(t *testing.T) {
		got := domain.SelectPendingActions(input, domain.PendingActionSelector{All: true})
		require.Equal(t, []*domain.PendingAction{cat1, rec1, cat2}, got)
	})

	t.Run("All with ActionType filters by type", func(t *testing.T) {
		got := domain.SelectPendingActions(input, domain.PendingActionSelector{
			All:        true,
			ActionType: &catType,
		})
		require.Equal(t, []*domain.PendingAction{cat1, cat2}, got)
	})

	t.Run("ActionIDs selects only matching pending actions in input order", func(t *testing.T) {
		got := domain.SelectPendingActions(input, domain.PendingActionSelector{
			ActionIDs: []uuid.UUID{cat2.ID, approvedAct.ID, cat1.ID, rec1.ID},
		})
		require.Equal(t, []*domain.PendingAction{cat1, rec1, cat2}, got)
	})

	t.Run("ActionIDs combined with ActionType enforces both constraints", func(t *testing.T) {
		got := domain.SelectPendingActions(input, domain.PendingActionSelector{
			ActionIDs:  []uuid.UUID{cat1.ID, rec1.ID, cat2.ID},
			ActionType: &recType,
		})
		require.Equal(t, []*domain.PendingAction{rec1}, got)
	})

	t.Run("empty selector returns empty slice", func(t *testing.T) {
		got := domain.SelectPendingActions(input, domain.PendingActionSelector{})
		require.Empty(t, got)
	})
}

func TestTransaction_ApplyApprovedCategorization(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	catID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	t.Run("applies category, AI flags, manual verification, copied confidence, and trimmed model version", func(t *testing.T) {
		tx, err := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   accountID,
			Date:        time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
			Amount:      decimal.RequireFromString("-75.00"),
			Description: "MERCADO CENTRAL",
			Type:        domain.TransactionTypeExpense,
			Status:      domain.TransactionStatusCompleted,
		})
		require.NoError(t, err)
		oldUpdated := tx.UpdatedAt.Add(-time.Minute)
		tx.UpdatedAt = oldUpdated

		conf := decimal.RequireFromString("0.91")
		require.NoError(t, tx.ApplyApprovedCategorization(catID, &conf, "  ai_categorization_agent  "))

		// Mutate caller's confidence after call to verify defensive copy
		conf = decimal.Zero

		require.NotNil(t, tx.CategoryID)
		require.Equal(t, catID, *tx.CategoryID)
		require.True(t, tx.IsAICategorized)
		require.True(t, tx.IsManuallyVerified)
		require.NotNil(t, tx.AIConfidence)
		require.True(t, decimal.RequireFromString("0.91").Equal(*tx.AIConfidence))
		require.Equal(t, "ai_categorization_agent", tx.AIModelVersion)
		require.True(t, tx.UpdatedAt.After(oldUpdated))
	})

	t.Run("handles nil confidence and blank model version without mutating optional fields", func(t *testing.T) {
		tx, err := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:    tenantID,
			AccountID:   accountID,
			Date:        time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
			Amount:      decimal.RequireFromString("3200.00"),
			Description: "SALARIO MENSAL",
			Type:        domain.TransactionTypeIncome,
			Status:      domain.TransactionStatusCompleted,
		})
		require.NoError(t, err)

		require.NoError(t, tx.ApplyApprovedCategorization(catID, nil, "   "))
		require.NotNil(t, tx.CategoryID)
		require.Equal(t, catID, *tx.CategoryID)
		require.True(t, tx.IsAICategorized)
		require.True(t, tx.IsManuallyVerified)
		require.Nil(t, tx.AIConfidence)
		require.Empty(t, tx.AIModelVersion)
	})

	t.Run("rejects invalid category, void status, transfer type, and out-of-range confidence without partial mutation", func(t *testing.T) {
		negConf := decimal.RequireFromString("-0.01")
		highConf := decimal.RequireFromString("1.01")

		cases := []struct {
			name        string
			txStatus    domain.TransactionStatus
			txType      domain.TransactionType
			categoryID  uuid.UUID
			confidence  *decimal.Decimal
			expectedErr error
		}{
			{
				name:        "nil category ID",
				txStatus:    domain.TransactionStatusCompleted,
				txType:      domain.TransactionTypeExpense,
				categoryID:  uuid.Nil,
				expectedErr: domain.ErrInvalidID,
			},
			{
				name:        "void transaction",
				txStatus:    domain.TransactionStatusVoid,
				txType:      domain.TransactionTypeExpense,
				categoryID:  catID,
				expectedErr: domain.ErrInvalidStatus,
			},
			{
				name:        "transfer transaction",
				txStatus:    domain.TransactionStatusCompleted,
				txType:      domain.TransactionTypeTransfer,
				categoryID:  catID,
				expectedErr: domain.ErrInvalidTransactionType,
			},
			{
				name:        "negative confidence",
				txStatus:    domain.TransactionStatusCompleted,
				txType:      domain.TransactionTypeExpense,
				categoryID:  catID,
				confidence:  &negConf,
				expectedErr: domain.ErrInvalidAmount,
			},
			{
				name:        "confidence above 1",
				txStatus:    domain.TransactionStatusCompleted,
				txType:      domain.TransactionTypeExpense,
				categoryID:  catID,
				confidence:  &highConf,
				expectedErr: domain.ErrInvalidAmount,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				tx := &domain.Transaction{
					ID:        uuid.New(),
					TenantID:  tenantID,
					AccountID: accountID,
					Date:      time.Now().UTC(),
					Amount:    decimal.RequireFromString("-50.00"),
					Type:      tc.txType,
					Status:    tc.txStatus,
				}
				err := tx.ApplyApprovedCategorization(tc.categoryID, tc.confidence, "mcp-server")
				require.ErrorIs(t, err, tc.expectedErr)
				require.Nil(t, tx.CategoryID)
				require.False(t, tx.IsAICategorized)
				require.False(t, tx.IsManuallyVerified)
				require.Nil(t, tx.AIConfidence)
				require.Empty(t, tx.AIModelVersion)
			})
		}
	})
}
