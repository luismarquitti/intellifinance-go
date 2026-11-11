package dto

import (
	"strings"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// ConfirmTransferRequest defines the payload for explicitly confirming a pair of transactions as a transfer.
type ConfirmTransferRequest struct {
	SourceTransactionID      string `json:"source_transaction_id"`
	DestinationTransactionID string `json:"destination_transaction_id"`
}

// Validate validates input fields of ConfirmTransferRequest.
func (r *ConfirmTransferRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam

	if strings.TrimSpace(r.SourceTransactionID) == "" {
		errs = append(errs, problem.InvalidParam{Name: "source_transaction_id", Reason: "cannot be empty"})
	} else if id, err := uuid.Parse(strings.TrimSpace(r.SourceTransactionID)); err != nil || id == uuid.Nil {
		errs = append(errs, problem.InvalidParam{Name: "source_transaction_id", Reason: "must be a valid non-nil UUID"})
	}

	if strings.TrimSpace(r.DestinationTransactionID) == "" {
		errs = append(errs, problem.InvalidParam{Name: "destination_transaction_id", Reason: "cannot be empty"})
	} else if id, err := uuid.Parse(strings.TrimSpace(r.DestinationTransactionID)); err != nil || id == uuid.Nil {
		errs = append(errs, problem.InvalidParam{Name: "destination_transaction_id", Reason: "must be a valid non-nil UUID"})
	}

	if r.SourceTransactionID != "" && r.SourceTransactionID == r.DestinationTransactionID {
		errs = append(errs, problem.InvalidParam{Name: "destination_transaction_id", Reason: "source and destination cannot be the same transaction"})
	}

	return errs
}

// CreateTransferRequest defines the payload for creating a dual-leg internal transfer directly.
type CreateTransferRequest struct {
	IdempotencyKey       string `json:"idempotency_key"`
	SourceAccountID      string `json:"source_account_id"`
	DestinationAccountID string `json:"destination_account_id"`
	Amount               string `json:"amount"`
	Date                 string `json:"date"`
	Description          string `json:"description"`
}

// Validate validates input fields of CreateTransferRequest.
func (r *CreateTransferRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam
	if strings.TrimSpace(r.IdempotencyKey) == "" || len(r.IdempotencyKey) > 200 {
		errs = append(errs, problem.InvalidParam{Name: "idempotency_key", Reason: "required; at most 200 bytes"})
	}

	if strings.TrimSpace(r.SourceAccountID) == "" {
		errs = append(errs, problem.InvalidParam{Name: "source_account_id", Reason: "cannot be empty"})
	} else if id, err := uuid.Parse(strings.TrimSpace(r.SourceAccountID)); err != nil || id == uuid.Nil {
		errs = append(errs, problem.InvalidParam{Name: "source_account_id", Reason: "must be a valid non-nil UUID"})
	}

	if strings.TrimSpace(r.DestinationAccountID) == "" {
		errs = append(errs, problem.InvalidParam{Name: "destination_account_id", Reason: "cannot be empty"})
	} else if id, err := uuid.Parse(strings.TrimSpace(r.DestinationAccountID)); err != nil || id == uuid.Nil {
		errs = append(errs, problem.InvalidParam{Name: "destination_account_id", Reason: "must be a valid non-nil UUID"})
	}

	if r.SourceAccountID != "" && r.SourceAccountID == r.DestinationAccountID {
		errs = append(errs, problem.InvalidParam{Name: "destination_account_id", Reason: "source and destination must be different accounts"})
	}

	if strings.TrimSpace(r.Amount) == "" {
		errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "cannot be empty"})
	} else {
		amt, err := decimal.NewFromString(strings.TrimSpace(r.Amount))
		if err != nil {
			errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "must be a valid decimal number"})
		} else if !amt.IsPositive() {
			errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "must be a positive decimal number"})
		} else if !domain.IsRepresentableInCents(amt) {
			errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "must be representable in exact cents without fractional cents"})
		} else if amt.GreaterThan(domain.MaxMonetaryAmount) {
			errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "amount exceeds maximum supported value"})
		}
	}

	if strings.TrimSpace(r.Date) == "" {
		errs = append(errs, problem.InvalidParam{Name: "date", Reason: "cannot be empty"})
	} else if _, err := ParseDate(r.Date); err != nil {
		errs = append(errs, problem.InvalidParam{Name: "date", Reason: "must be formatted as YYYY-MM-DD or RFC3339"})
	}

	return errs
}

// UnlinkTransferRequest defines the payload for unlinking an existing transfer pair.
type UnlinkTransferRequest struct {
	TransactionID string `json:"transaction_id"`
}

// Validate validates input fields of UnlinkTransferRequest.
func (r *UnlinkTransferRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam
	if strings.TrimSpace(r.TransactionID) == "" {
		errs = append(errs, problem.InvalidParam{Name: "transaction_id", Reason: "cannot be empty"})
	} else if id, err := uuid.Parse(strings.TrimSpace(r.TransactionID)); err != nil || id == uuid.Nil {
		errs = append(errs, problem.InvalidParam{Name: "transaction_id", Reason: "must be a valid non-nil UUID"})
	}
	return errs
}

// TransferResponse encapsulates both legs of a confirmed transfer pair.
type TransferResponse struct {
	Outflow TransactionResponse `json:"outflow"`
	Inflow  TransactionResponse `json:"inflow"`
}

// ToTransferResponse maps two domain transactions to a TransferResponse.
func ToTransferResponse(legA, legB *domain.Transaction) TransferResponse {
	outflow := legA
	inflow := legB
	if legA != nil && legA.Amount.IsPositive() {
		outflow = legB
		inflow = legA
	}

	var outResp, inResp TransactionResponse
	if outflow != nil {
		outResp = ToTransactionResponse(outflow)
	}
	if inflow != nil {
		inResp = ToTransactionResponse(inflow)
	}

	return TransferResponse{
		Outflow: outResp,
		Inflow:  inResp,
	}
}
