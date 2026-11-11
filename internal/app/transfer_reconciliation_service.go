package app

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.TransferReconciliationService = (*TransferReconciliationService)(nil)

// TransferReconciliationService scans for mutual cash movements and proposes HITL reconciliation actions.
type TransferReconciliationService struct {
	txRepo            ports.TransactionRepository
	accountRepo       ports.AccountRepository
	invRepo           ports.InvoiceRepository
	pendingActionRepo ports.PendingActionRepository
}

// NewTransferReconciliationService constructs a new TransferReconciliationService instance.
func NewTransferReconciliationService(
	txRepo ports.TransactionRepository,
	accountRepo ports.AccountRepository,
	invRepo ports.InvoiceRepository,
	pendingActionRepo ports.PendingActionRepository,
) *TransferReconciliationService {
	return &TransferReconciliationService{
		txRepo:            txRepo,
		accountRepo:       accountRepo,
		invRepo:           invRepo,
		pendingActionRepo: pendingActionRepo,
	}
}

// ScanAndPropose performs heuristic pairing on unlinked transactions within a tenant.
func (s *TransferReconciliationService) ScanAndPropose(ctx context.Context, tenantID uuid.UUID) (*ports.ReconciliationScanResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}

	// 1. Load accounts to identify types and names
	accounts, err := s.accountRepo.List(ctx, tenantID, ports.AccountFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}
	accMap := make(map[uuid.UUID]*domain.Account)
	for _, acc := range accounts {
		accMap[acc.ID] = acc
	}

	// 2. Load existing pending actions to avoid duplicate proposals
	pendingActions, err := s.pendingActionRepo.ListPending(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending actions: %w", err)
	}
	existingProposedTxIDs := make(map[uuid.UUID]bool)
	for _, act := range pendingActions {
		if act.ActionType == domain.ActionTypeReconcileTransfer {
			if outStr, ok := act.Payload["outflow_transaction_id"].(string); ok {
				if id, pErr := uuid.Parse(outStr); pErr == nil {
					existingProposedTxIDs[id] = true
				}
			}
			if inStr, ok := act.Payload["inflow_transaction_id"].(string); ok {
				if id, pErr := uuid.Parse(inStr); pErr == nil {
					existingProposedTxIDs[id] = true
				}
			}
		}
	}

	// 3. Load transactions (up to 1000 recent)
	limit := 1000
	txs, _, err := s.txRepo.List(ctx, tenantID, ports.TransactionFilter{Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	var outflows []*domain.Transaction
	var inflows []*domain.Transaction

	for _, tx := range txs {
		// Only consider transactions that are not void and not already paired
		if tx.Status == domain.TransactionStatusVoid || tx.TransferPeerTransactionID != nil {
			continue
		}
		if existingProposedTxIDs[tx.ID] {
			continue
		}

		if tx.Amount.IsNegative() {
			outflows = append(outflows, tx)
		} else if tx.Amount.IsPositive() {
			inflows = append(inflows, tx)
		}
	}

	// 4. Load invoices for bill payment detection
	invoices, _ := s.invRepo.List(ctx, tenantID, nil)

	pairedInflows := make(map[uuid.UUID]bool)
	var proposedActions []*domain.PendingAction

	// 5. Heuristic matching
	for _, out := range outflows {
		absOutAmount := out.Amount.Abs()
		var candidateInflows []*domain.Transaction

		for _, in := range inflows {
			if pairedInflows[in.ID] {
				continue
			}
			// Must be different accounts
			if in.AccountID == out.AccountID {
				continue
			}
			// Exact amount match
			if !in.Amount.Equal(absOutAmount) {
				continue
			}
			// Date tolerance: <= 2 calendar days (48 hours)
			dateDiffDays := math.Abs(out.Date.Sub(in.Date).Hours()) / 24.0
			if dateDiffDays <= 2.0 {
				candidateInflows = append(candidateInflows, in)
			}
		}

		if len(candidateInflows) == 0 {
			continue
		}

		// Pick the closest date
		sort.Slice(candidateInflows, func(i, j int) bool {
			diffI := math.Abs(out.Date.Sub(candidateInflows[i].Date).Hours())
			diffJ := math.Abs(out.Date.Sub(candidateInflows[j].Date).Hours())
			return diffI < diffJ
		})

		bestInflow := candidateInflows[0]
		pairedInflows[bestInflow.ID] = true

		// Classify transaction pair
		srcAcc := accMap[out.AccountID]
		dstAcc := accMap[bestInflow.AccountID]

		srcName := "Conta de Origem"
		if srcAcc != nil {
			srcName = srcAcc.Name
		}
		dstName := "Conta de Destino"
		if dstAcc != nil {
			dstName = dstAcc.Name
		}

		reconciliationType := "INTERNAL_TRANSFER"
		var matchedInvoiceID *uuid.UUID

		// Check if destination is Credit Card (Bill Payment)
		if dstAcc != nil && dstAcc.Type == domain.AccountTypeCreditCard {
			reconciliationType = "BILL_PAYMENT"
			// Try to match an invoice for this credit card account
			for _, inv := range invoices {
				if inv.AccountID == dstAcc.ID {
					if inv.TotalAmount.Equal(bestInflow.Amount) || math.Abs(inv.DueDate.Sub(bestInflow.Date).Hours())/24.0 <= 10.0 {
						matchedInvoiceID = &inv.ID
						break
					}
				}
			}
		}

		var desc string
		if reconciliationType == "BILL_PAYMENT" {
			desc = fmt.Sprintf("Pagamento de fatura do %s (R$ %s) a partir de %s", dstName, absOutAmount.StringFixed(2), srcName)
		} else {
			desc = fmt.Sprintf("Transferência de R$ %s de %s para %s", absOutAmount.StringFixed(2), srcName, dstName)
		}

		payload := map[string]any{
			"outflow_transaction_id":   out.ID.String(),
			"inflow_transaction_id":    bestInflow.ID.String(),
			"amount":                   absOutAmount.StringFixed(2),
			"source_account_id":        out.AccountID.String(),
			"source_account_name":      srcName,
			"destination_account_id":   bestInflow.AccountID.String(),
			"destination_account_name": dstName,
			"outflow_date":             out.Date.Format("2006-01-02"),
			"inflow_date":              bestInflow.Date.Format("2006-01-02"),
			"reconciliation_type":      reconciliationType,
			"description":              desc,
		}
		if matchedInvoiceID != nil {
			payload["invoice_id"] = matchedInvoiceID.String()
		}

		action, actErr := domain.NewPendingAction(
			tenantID,
			"transfer-reconciler",
			domain.ActionTypeReconcileTransfer,
			payload,
		)
		if actErr != nil {
			continue
		}

		if saveErr := s.pendingActionRepo.Save(ctx, action); saveErr != nil {
			return nil, fmt.Errorf("failed saving reconciliation pending action: %w", saveErr)
		}

		proposedActions = append(proposedActions, action)
	}

	return &ports.ReconciliationScanResult{
		ScannedTransactionsCount: len(txs),
		ProposedActionsCount:     len(proposedActions),
		ProposedActions:          proposedActions,
	}, nil
}
