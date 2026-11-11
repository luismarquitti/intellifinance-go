package dto_test

import (
	"encoding/json"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReviewTransferRequiresIdempotencyKey(t *testing.T) {
	var req dto.CreateTransferRequest
	require.NoError(t, json.Unmarshal([]byte(`{"source_account_id":"11111111-1111-4111-8111-111111111111","destination_account_id":"22222222-2222-4222-8222-222222222222","amount":"10","date":"2026-02-01"}`), &req))
	require.NotEmpty(t, req.Validate())
	require.NoError(t, json.Unmarshal([]byte(`{"idempotency_key":"synthetic-key"}`), &req))
	require.Empty(t, req.Validate())
}

func TestReviewTransferAmountPrecisionValidation(t *testing.T) {
	baseJSON := `{"idempotency_key":"k","source_account_id":"11111111-1111-4111-8111-111111111111","destination_account_id":"22222222-2222-4222-8222-222222222222","date":"2026-02-01"}`
	cases := []struct {
		amount  string
		wantErr bool
	}{
		{"1.001", true},
		{"0.001", true},
		{"-10.00", true},
		{"0", true},
		{"10000000000000.00", true}, // exceeds numeric(15,2)
		{"1.000", false},            // excess trailing zeros without fractional cent loss
		{"10.50", false},
		{"9999999999999.99", false}, // max supported numeric(15,2)
	}
	for _, tc := range cases {
		var req dto.CreateTransferRequest
		require.NoError(t, json.Unmarshal([]byte(baseJSON), &req))
		req.Amount = tc.amount
		errs := req.Validate()
		if tc.wantErr {
			require.NotEmpty(t, errs, "expected error for amount %s", tc.amount)
		} else {
			require.Empty(t, errs, "expected no error for amount %s", tc.amount)
		}
	}
}

