package app

import (
	"context"
	"errors"
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
	txRepo           ports.TransactionRepository
	accountRepo      ports.AccountRepository
	invRepo          ports.InvoiceRepository
	pendingActionSvc ports.PendingActionService
}

// NewTransferReconciliationService constructs a new TransferReconciliationService instance.
func NewTransferReconciliationService(
	txRepo ports.TransactionRepository,
	accountRepo ports.AccountRepository,
	invRepo ports.InvoiceRepository,
	pendingActionSvc ports.PendingActionService,
) *TransferReconciliationService {
	return &TransferReconciliationService{
		txRepo:           txRepo,
		accountRepo:      accountRepo,
		invRepo:          invRepo,
		pendingActionSvc: pendingActionSvc,
	}
}

// ScanAndPropose performs heuristic pairing on unlinked transactions within a tenant.
func (s *TransferReconciliationService) ScanAndPropose(ctx context.Context, tenantID uuid.UUID) (*ports.ReconciliationScanResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	if s == nil || s.pendingActionSvc == nil {
		return nil, errors.New("pending action service is required")
	}
	if s.accountRepo == nil || s.txRepo == nil {
		return nil, errors.New("account and transaction repositories are required")
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

	// 2. Load existing pending actions index to avoid duplicate proposals
	pendingIdx, err := s.pendingActionSvc.LoadPendingIndex(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to load pending actions index: %w", err)
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
		// Only consider settled/completed transactions that are not already paired
		if tx.Status != domain.TransactionStatusCompleted || tx.TransferPeerTransactionID != nil {
			continue
		}
		if pendingIdx.HasTransferLeg(tx.ID) {
			continue
		}

		if tx.Amount.IsNegative() {
			// Credit card outflows (purchases/charges) are never outbound cash transfers to another account.
			if acc := accMap[tx.AccountID]; acc != nil && acc.Type == domain.AccountTypeCreditCard {
				continue
			}
			outflows = append(outflows, tx)
		} else if tx.Amount.IsPositive() {
			inflows = append(inflows, tx)
		}
	}

	// 4. Load invoices for Invoice Settlement detection
	var invoices []*domain.CreditCardInvoice
	if s.invRepo != nil {
		invoices, err = s.invRepo.List(ctx, tenantID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list invoices: %w", err)
		}
	}

	// Prioritize Liquid Account outflows whose description matches Invoice Settlement keywords
	// so generic same-amount debits do not preempt explicit card-payment outflows.
	sort.SliceStable(outflows, func(i, j int) bool {
		iMatch := domain.IsCheckingInvoicePaymentDescription(outflows[i].Description)
		jMatch := domain.IsCheckingInvoicePaymentDescription(outflows[j].Description)
		return iMatch && !jMatch
	})

	pairedInflows := make(map[uuid.UUID]bool)
	proposedActions := make([]*domain.PendingAction, 0)

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

		// Classify transaction pair
		srcAcc := accMap[out.AccountID]
		dstAcc := accMap[bestInflow.AccountID]

		if isBillPay, ok := bestInflow.Metadata["is_bill_payment"].(bool); ok && isBillPay && (dstAcc == nil || dstAcc.Type != domain.AccountTypeCreditCard) {
			continue
		}

		var matchedInvoiceID *uuid.UUID

		// Check if destination is Credit Card (Invoice Settlement)
		if dstAcc != nil && dstAcc.Type == domain.AccountTypeCreditCard {
			matchedInv, hasInvoicesForCard := domain.MatchInvoiceForSettlementLeg(
				invoices,
				dstAcc.ID,
				bestInflow.Amount,
				bestInflow.Date,
				domain.DefaultSettlementWindow(nil),
			)
			if matchedInv != nil {
				if matchedInv.Status == domain.InvoiceStatusPaid {
					continue
				}
				matchedInvoiceID = &matchedInv.ID
			} else if hasInvoicesForCard {
				continue
			}
		}

		proposal, propErr := domain.NewReconcileTransferProposalFromEntities(out, bestInflow, srcAcc, dstAcc, matchedInvoiceID)
		if propErr != nil {
			continue
		}

		action, saveErr := s.pendingActionSvc.ProposeTransferReconciliation(ctx, tenantID, "transfer-reconciler", proposal)
		if saveErr != nil {
			if errors.Is(saveErr, domain.ErrDuplicatePendingAction) {
				continue
			}
			return nil, fmt.Errorf("failed saving reconciliation pending action: %w", saveErr)
		}

		pairedInflows[bestInflow.ID] = true
		pendingIdx.Record(action)
		proposedActions = append(proposedActions, action)
	}

	return &ports.ReconciliationScanResult{
		ScannedTransactionsCount: len(txs),
		ProposedActionsCount:     len(proposedActions),
		ProposedActions:          proposedActions,
	}, nil
}
