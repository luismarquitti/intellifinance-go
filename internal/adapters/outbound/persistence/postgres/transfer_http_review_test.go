package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReviewTransferHTTPStableReplay(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic HTTP A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic HTTP B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	handler := handlers.NewTransferHandler(app.NewTransferService(postgres.NewTransferRepository(pool)))
	call := func(key, amount string, want int) dto.TransferResponse {
		body, err := json.Marshal(dto.CreateTransferRequest{SourceAccountID: a.ID.String(), DestinationAccountID: b.ID.String(), Amount: amount, Date: "2026-02-01", Description: "Synthetic HTTP", IdempotencyKey: key})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewReader(body))
		req = req.WithContext(middleware.WithUserID(middleware.WithTenantID(req.Context(), tenant), user))
		rr := httptest.NewRecorder()
		handler.CreateTransfer(rr, req)
		require.Equal(t, want, rr.Code, rr.Body.String())
		var response dto.TransferResponse
		if want == 201 {
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
		}
		return response
	}
	call("", "10", 422)
	first := call("synthetic-http", "10", 201)
	replay := call("synthetic-http", "10.00", 201)
	require.Equal(t, first.Outflow.ID, replay.Outflow.ID)
	require.Equal(t, first.Inflow.ID, replay.Inflow.ID)
	second := call("synthetic-http-second", "10", 201)
	require.NotEqual(t, first.Outflow.ID, second.Outflow.ID)
	call("synthetic-http", "11", 409)
	ledger := postgres.NewTransactionRepository(pool)
	for _, id := range []uuid.UUID{first.Outflow.ID, first.Inflow.ID, second.Outflow.ID, second.Inflow.ID} {
		_, err := ledger.GetByID(ctx, tenant, id)
		require.NoError(t, err)
	}
	balance, err := accounts.GetByID(ctx, tenant, a.ID)
	require.NoError(t, err)
	require.Equal(t, "-20.00", balance.Balance.StringFixed(2))
}

func TestReviewTransferHTTPSubCentAndLimits(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic HTTP SubCent A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic HTTP SubCent B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	handler := handlers.NewTransferHandler(app.NewTransferService(postgres.NewTransferRepository(pool)))
	call := func(key, amount string, want int) dto.TransferResponse {
		body, err := json.Marshal(dto.CreateTransferRequest{
			SourceAccountID:      a.ID.String(),
			DestinationAccountID: b.ID.String(),
			Amount:               amount,
			Date:                 "2026-02-01",
			Description:          "SubCent Test",
			IdempotencyKey:       key,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewReader(body))
		req = req.WithContext(middleware.WithUserID(middleware.WithTenantID(req.Context(), tenant), user))
		rr := httptest.NewRecorder()
		handler.CreateTransfer(rr, req)
		require.Equal(t, want, rr.Code, rr.Body.String())
		var response dto.TransferResponse
		if want == 201 {
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
		}
		return response
	}

	// 1.001 and 0.001 must be rejected with 422
	call("subcent-1", "1.001", 422)
	call("subcent-2", "0.001", 422)
	// 10000000000000.00 exceeds numeric(15,2) limit -> 422
	call("overflow-1", "10000000000000.00", 422)

	// Valid cent values:
	// 1.000 has extra zeros but is mathematically equal to 1.00 (representable in cents) -> 201
	resp1 := call("exact-1", "1.000", 201)
	require.Equal(t, "1.00", resp1.Inflow.Amount)

	// Replay with 1.00 should match resp1
	replay1 := call("exact-1", "1.00", 201)
	require.Equal(t, resp1.Outflow.ID, replay1.Outflow.ID)

	// Max NUMERIC(15,2) value on fresh accounts starting at 0.00 balance
	c, _ := domain.NewAccount(tenant, user, "Synthetic Max C", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	d, _ := domain.NewAccount(tenant, user, "Synthetic Max D", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, c))
	require.NoError(t, accounts.Create(ctx, d))

	bodyMax, err := json.Marshal(dto.CreateTransferRequest{
		SourceAccountID:      c.ID.String(),
		DestinationAccountID: d.ID.String(),
		Amount:               "9999999999999.99",
		Date:                 "2026-02-01",
		Description:          "Max NUMERIC(15,2)",
		IdempotencyKey:       "max-val",
	})
	require.NoError(t, err)
	reqMax := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewReader(bodyMax))
	reqMax = reqMax.WithContext(middleware.WithUserID(middleware.WithTenantID(reqMax.Context(), tenant), user))
	rrMax := httptest.NewRecorder()
	handler.CreateTransfer(rrMax, reqMax)
	require.Equal(t, 201, rrMax.Code, rrMax.Body.String())
	var respMax dto.TransferResponse
	require.NoError(t, json.Unmarshal(rrMax.Body.Bytes(), &respMax))
	require.Equal(t, "9999999999999.99", respMax.Inflow.Amount)
}

