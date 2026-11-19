package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

var _ ports.PendingActionService = (*PendingActionService)(nil)

// PendingActionService implements Human-in-the-Loop proposal hydration, deduplication,
// single and batch decision workflows, and fail-closed cross-aggregate execution.
type PendingActionService struct {
	actionRepo      ports.PendingActionRepository
	txRepo          ports.TransactionRepository
	catRepo         ports.CategoryRepository
	accountRepo     ports.AccountRepository
	invoiceRepo     ports.InvoiceRepository
	transferService ports.TransferService
}

// NewPendingActionService constructs a PendingActionService backed by repository and transfer ports.
func NewPendingActionService(
	actionRepo ports.PendingActionRepository,
	txRepo ports.TransactionRepository,
	catRepo ports.CategoryRepository,
	accountRepo ports.AccountRepository,
	invoiceRepo ports.InvoiceRepository,
	transferService ports.TransferService,
) *PendingActionService {
	return &PendingActionService{
		actionRepo:      actionRepo,
		txRepo:          txRepo,
		catRepo:         catRepo,
		accountRepo:     accountRepo,
		invoiceRepo:     invoiceRepo,
		transferService: transferService,
	}
}

// ListPending retrieves all PENDING actions awaiting review within a tenant.
func (s *PendingActionService) ListPending(ctx context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	if s == nil || s.actionRepo == nil {
		return nil, errors.New("pending action repository is required")
	}

	actions, err := s.actionRepo.ListPending(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed listing pending actions: %w", err)
	}
	if actions == nil {
		return []*domain.PendingAction{}, nil
	}
	return actions, nil
}

// LoadPendingIndex builds an O(1) deduplication index from active PENDING actions within a tenant.
func (s *PendingActionService) LoadPendingIndex(ctx context.Context, tenantID uuid.UUID) (*domain.PendingActionIndex, error) {
	actions, err := s.ListPending(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return domain.NewPendingActionIndex(actions), nil
}

// ProposeCategorization validates, optionally hydrates, deduplicates, and persists a categorization proposal.
func (s *PendingActionService) ProposeCategorization(
	ctx context.Context,
	tenantID uuid.UUID,
	createdByAgent string,
	proposal domain.CategorizeTransactionProposal,
) (*domain.PendingAction, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	if s == nil || s.actionRepo == nil {
		return nil, errors.New("pending action repository is required")
	}
	if hdrErr := proposal.ValidateHeader(); hdrErr != nil {
		return nil, hdrErr
	}
	if !proposal.NeedsHydration() {
		if valErr := proposal.Validate(); valErr != nil {
			return nil, valErr
		}
	}

	idx, idxErr := s.LoadPendingIndex(ctx, tenantID)
	if idxErr != nil {
		return nil, idxErr
	}
	if idx.HasCategorizationFor(proposal.TransactionID) || idx.HasDeduplicationKey(proposal.DeduplicationKey()) {
		return nil, domain.ErrDuplicatePendingAction
	}

	if proposal.NeedsHydration() {
		hydrated, hydErr := s.hydrateCategorizationProposal(ctx, tenantID, proposal)
		if hydErr != nil {
			return nil, hydErr
		}
		proposal = hydrated
	}

	action, actErr := domain.NewCategorizePendingAction(tenantID, createdByAgent, proposal)
	if actErr != nil {
		return nil, actErr
	}
	if saveErr := s.actionRepo.Save(ctx, action); saveErr != nil {
		return nil, fmt.Errorf("failed saving categorization pending action: %w", saveErr)
	}
	return action, nil
}

func (s *PendingActionService) hydrateCategorizationProposal(
	ctx context.Context,
	tenantID uuid.UUID,
	proposal domain.CategorizeTransactionProposal,
) (domain.CategorizeTransactionProposal, error) {
	if s.txRepo == nil || s.catRepo == nil {
		return domain.CategorizeTransactionProposal{}, errors.New("transaction and category repositories are required to hydrate categorization proposal")
	}

	tx, txErr := s.loadTransaction(ctx, tenantID, proposal.TransactionID)
	if txErr != nil {
		return domain.CategorizeTransactionProposal{}, txErr
	}

	cat, catErr := s.loadCategory(ctx, tenantID, proposal.CategoryID)
	if catErr != nil {
		return domain.CategorizeTransactionProposal{}, catErr
	}

	return domain.NewCategorizeProposalFromEntities(tx, cat, proposal.Confidence, proposal.Reason)
}

// ProposeTransferReconciliation validates, deduplicates, and persists a transfer reconciliation proposal.
func (s *PendingActionService) ProposeTransferReconciliation(
	ctx context.Context,
	tenantID uuid.UUID,
	createdByAgent string,
	proposal domain.ReconcileTransferProposal,
) (*domain.PendingAction, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	if s == nil || s.actionRepo == nil {
		return nil, errors.New("pending action repository is required")
	}

	action, actErr := domain.NewReconcileTransferPendingAction(tenantID, createdByAgent, proposal)
	if actErr != nil {
		return nil, actErr
	}

	idx, idxErr := s.LoadPendingIndex(ctx, tenantID)
	if idxErr != nil {
		return nil, idxErr
	}
	if idx.HasTransferLeg(proposal.OutflowTransactionID) ||
		idx.HasTransferLeg(proposal.InflowTransactionID) ||
		idx.HasDeduplicationKey(proposal.DeduplicationKey()) {
		return nil, domain.ErrDuplicatePendingAction
	}

	if saveErr := s.actionRepo.Save(ctx, action); saveErr != nil {
		return nil, fmt.Errorf("failed saving transfer reconciliation pending action: %w", saveErr)
	}
	return action, nil
}

// Approve executes a single pending action and marks it APPROVED and EXECUTED only if execution succeeds.
func (s *PendingActionService) Approve(
	ctx context.Context,
	tenantID, actionID, decidedBy uuid.UUID,
) (*domain.PendingAction, error) {
	action, err := s.loadAction(ctx, tenantID, actionID)
	if err != nil {
		return nil, err
	}
	return s.approveLoadedAction(ctx, tenantID, action, decidedBy)
}

func (s *PendingActionService) loadAction(
	ctx context.Context,
	tenantID, actionID uuid.UUID,
) (*domain.PendingAction, error) {
	if tenantID == uuid.Nil || actionID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	if s == nil || s.actionRepo == nil {
		return nil, errors.New("pending action repository is required")
	}

	action, err := s.actionRepo.FindByID(ctx, tenantID, actionID)
	return ensureLoadedEntity(
		action,
		err,
		func(a *domain.PendingAction) uuid.UUID { return a.TenantID },
		tenantID,
		fmt.Sprintf("pending action %s", actionID),
	)
}

func (s *PendingActionService) approveLoadedAction(
	ctx context.Context,
	tenantID uuid.UUID,
	action *domain.PendingAction,
	decidedBy uuid.UUID,
) (*domain.PendingAction, error) {
	if valErr := validatePendingActionForDecision(action, tenantID); valErr != nil {
		return nil, valErr
	}

	if execErr := s.executeAction(ctx, tenantID, action); execErr != nil {
		return nil, execErr
	}

	decidedByPtr := optionalUserIDPtr(decidedBy)
	if updErr := s.actionRepo.UpdateDecision(ctx, tenantID, action.ID, domain.ActionStatusApproved, decidedByPtr, nil); updErr != nil {
		return nil, fmt.Errorf("failed updating action approval decision: %w", updErr)
	}
	if markErr := s.actionRepo.MarkExecuted(ctx, tenantID, action.ID); markErr != nil {
		return nil, fmt.Errorf("failed marking action as executed: %w", markErr)
	}

	if appErr := action.Approve(decidedBy); appErr != nil {
		return nil, appErr
	}
	if domMarkErr := action.MarkExecuted(); domMarkErr != nil {
		return nil, domMarkErr
	}
	return action, nil
}

func (s *PendingActionService) executeAction(
	ctx context.Context,
	tenantID uuid.UUID,
	action *domain.PendingAction,
) error {
	switch action.ActionType {
	case domain.ActionTypeCategorizeTransaction:
		return s.executeCategorization(ctx, tenantID, action)
	case domain.ActionTypeReconcileTransfer:
		return s.executeReconcileTransfer(ctx, tenantID, action)
	default:
		return fmt.Errorf("%w: %s", domain.ErrUnsupportedPendingActionType, action.ActionType)
	}
}

type invoiceSettlementPreflight struct {
	inv       *domain.CreditCardInvoice
	liquidAcc *domain.Account
}

func (s *PendingActionService) executeReconcileTransfer(
	ctx context.Context,
	tenantID uuid.UUID,
	action *domain.PendingAction,
) error {
	proposal, decErr := domain.DecodeReconcileTransferPayload(action)
	if decErr != nil {
		return decErr
	}
	if s.transferService == nil {
		return errors.New("transfer service is required")
	}

	var preflight invoiceSettlementPreflight
	if proposal.InvoiceID != nil {
		var preErr error
		preflight, preErr = s.preflightInvoiceSettlement(ctx, tenantID, proposal)
		if preErr != nil {
			return preErr
		}
	}

	outflowTx, inflowTx, confErr := s.transferService.ConfirmTransfer(
		ctx,
		tenantID,
		proposal.OutflowTransactionID,
		proposal.InflowTransactionID,
	)
	if confErr != nil {
		return fmt.Errorf("failed confirming reconciled transfer: %w", confErr)
	}

	if proposal.InvoiceID != nil {
		return s.settleReconciledInvoice(ctx, tenantID, proposal, preflight, outflowTx, inflowTx)
	}
	return nil
}

func (s *PendingActionService) preflightInvoiceSettlement(
	ctx context.Context,
	tenantID uuid.UUID,
	proposal domain.ReconcileTransferProposal,
) (invoiceSettlementPreflight, error) {
	if s.invoiceRepo == nil || s.accountRepo == nil {
		return invoiceSettlementPreflight{}, errors.New("invoice and account repositories are required for invoice settlement")
	}

	inv, invErr := s.loadInvoice(ctx, tenantID, *proposal.InvoiceID)
	if invErr != nil {
		return invoiceSettlementPreflight{}, invErr
	}
	if proposal.DestinationAccountID != uuid.Nil && inv.AccountID != proposal.DestinationAccountID {
		return invoiceSettlementPreflight{}, fmt.Errorf("%w: invoice account %s does not match destination account %s", domain.ErrInvalidPendingActionPayload, inv.AccountID, proposal.DestinationAccountID)
	}

	var liquidAcc *domain.Account
	if proposal.SourceAccountID != uuid.Nil {
		var accErr error
		liquidAcc, accErr = s.loadAccount(ctx, tenantID, proposal.SourceAccountID, "source account")
		if accErr != nil {
			return invoiceSettlementPreflight{}, accErr
		}
	}

	if _, cardErr := s.loadAccount(ctx, tenantID, inv.AccountID, "credit card account"); cardErr != nil {
		return invoiceSettlementPreflight{}, cardErr
	}
	if _, listErr := s.invoiceRepo.ListByAccount(ctx, tenantID, inv.AccountID); listErr != nil {
		return invoiceSettlementPreflight{}, fmt.Errorf("failed listing invoices for credit card account %s: %w", inv.AccountID, listErr)
	}

	return invoiceSettlementPreflight{
		inv:       inv,
		liquidAcc: liquidAcc,
	}, nil
}

func (s *PendingActionService) settleReconciledInvoice(
	ctx context.Context,
	tenantID uuid.UUID,
	proposal domain.ReconcileTransferProposal,
	preflight invoiceSettlementPreflight,
	outflowTx, inflowTx *domain.Transaction,
) error {
	inv := preflight.inv
	if inv == nil {
		var invErr error
		inv, invErr = s.loadInvoice(ctx, tenantID, *proposal.InvoiceID)
		if invErr != nil {
			return invErr
		}
	}
	if inflowTx != nil && inflowTx.AccountID != uuid.Nil && inv.AccountID != inflowTx.AccountID {
		return fmt.Errorf("%w: invoice account %s does not match inflow account %s", domain.ErrInvalidPendingActionPayload, inv.AccountID, inflowTx.AccountID)
	}
	if (inflowTx == nil || inflowTx.AccountID == uuid.Nil) && proposal.DestinationAccountID != uuid.Nil && inv.AccountID != proposal.DestinationAccountID {
		return fmt.Errorf("%w: invoice account %s does not match destination account %s", domain.ErrInvalidPendingActionPayload, inv.AccountID, proposal.DestinationAccountID)
	}

	sourceAccID := proposal.SourceAccountID
	if outflowTx != nil && outflowTx.AccountID != uuid.Nil {
		sourceAccID = outflowTx.AccountID
	}
	if sourceAccID == uuid.Nil {
		return fmt.Errorf("%w: source_account_id: %w", domain.ErrInvalidPendingActionPayload, domain.ErrInvalidID)
	}

	liquidAcc := preflight.liquidAcc
	if liquidAcc == nil || sourceAccID != proposal.SourceAccountID {
		var accErr error
		liquidAcc, accErr = s.loadAccount(ctx, tenantID, sourceAccID, "source account")
		if accErr != nil {
			return accErr
		}
	}

	paidAt := resolveSettlementPaidAt(outflowTx, proposal.OutflowDate)
	inv.ApplySettlement(domain.InvoiceSettlementMatch{
		Matched:         true,
		LiquidAccount:   liquidAcc,
		LiquidOutflowTx: outflowTx,
		PaidAt:          paidAt,
		PaidAmount:      inv.TotalAmount,
	})
	if saveErr := s.invoiceRepo.Save(ctx, inv); saveErr != nil {
		return fmt.Errorf("failed saving settled invoice %s: %w", inv.ID, saveErr)
	}

	cardAcc, cardErr := s.loadAccount(ctx, tenantID, inv.AccountID, "credit card account")
	if cardErr != nil {
		return cardErr
	}

	cardInvoices, listErr := s.invoiceRepo.ListByAccount(ctx, tenantID, inv.AccountID)
	if listErr != nil {
		return fmt.Errorf("failed listing invoices for credit card account %s: %w", inv.AccountID, listErr)
	}

	delta := domain.ComputeCreditCardLiabilityDelta(cardAcc.Balance, cardInvoices)
	if !delta.IsZero() {
		if balErr := s.accountRepo.UpdateBalance(ctx, tenantID, inv.AccountID, delta); balErr != nil {
			return fmt.Errorf("failed updating credit card account %s liability balance: %w", inv.AccountID, balErr)
		}
	}
	return nil
}

func resolveSettlementPaidAt(outflowTx *domain.Transaction, outflowDate string) time.Time {
	if outflowTx != nil && !outflowTx.Date.IsZero() {
		return outflowTx.Date
	}
	if outflowDate != "" {
		if parsedDate, parseErr := time.ParseInLocation("2006-01-02", outflowDate, domain.BrazilLocation); parseErr == nil {
			return parsedDate
		}
	}
	return time.Now().UTC()
}

func (s *PendingActionService) executeCategorization(
	ctx context.Context,
	tenantID uuid.UUID,
	action *domain.PendingAction,
) error {
	proposal, decErr := domain.DecodeCategorizePayload(action)
	if decErr != nil {
		return decErr
	}
	if s.txRepo == nil {
		return errors.New("transaction repository is required")
	}

	tx, getErr := s.loadTransaction(ctx, tenantID, proposal.TransactionID)
	if getErr != nil {
		return getErr
	}

	var confPtr *decimal.Decimal
	if proposal.Confidence.IsPositive() {
		confCopy := proposal.Confidence
		confPtr = &confCopy
	}

	if applyErr := tx.ApplyApprovedCategorization(proposal.CategoryID, confPtr, action.CreatedByAgent); applyErr != nil {
		return applyErr
	}
	if valErr := tx.Validate(); valErr != nil {
		return valErr
	}
	if updErr := s.txRepo.Update(ctx, tx); updErr != nil {
		return fmt.Errorf("failed updating categorized transaction %s: %w", tx.ID, updErr)
	}
	return nil
}

func optionalUserIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	idCopy := id
	return &idCopy
}

// Reject marks a single pending action as REJECTED with an optional trimmed reason.
func (s *PendingActionService) Reject(
	ctx context.Context,
	tenantID, actionID, decidedBy uuid.UUID,
	reason string,
) (*domain.PendingAction, error) {
	action, err := s.loadAction(ctx, tenantID, actionID)
	if err != nil {
		return nil, err
	}
	return s.rejectLoadedAction(ctx, tenantID, action, decidedBy, reason)
}

func (s *PendingActionService) rejectLoadedAction(
	ctx context.Context,
	tenantID uuid.UUID,
	action *domain.PendingAction,
	decidedBy uuid.UUID,
	reason string,
) (*domain.PendingAction, error) {
	if valErr := validatePendingActionForDecision(action, tenantID); valErr != nil {
		return nil, valErr
	}

	trimmedReason := strings.TrimSpace(reason)
	var reasonPtr *string
	if trimmedReason != "" {
		reasonPtr = &trimmedReason
	}
	decidedByPtr := optionalUserIDPtr(decidedBy)

	if updErr := s.actionRepo.UpdateDecision(ctx, tenantID, action.ID, domain.ActionStatusRejected, decidedByPtr, reasonPtr); updErr != nil {
		return nil, fmt.Errorf("failed updating action rejection decision: %w", updErr)
	}
	if rejErr := action.Reject(decidedBy, trimmedReason); rejErr != nil {
		return nil, rejErr
	}
	return action, nil
}

// ApproveBatch applies the selector to pending actions and approves each one with per-item failure isolation.
func (s *PendingActionService) ApproveBatch(
	ctx context.Context,
	tenantID, decidedBy uuid.UUID,
	selector domain.PendingActionSelector,
) (*ports.BatchDecisionResult, error) {
	return s.runBatchDecision(ctx, tenantID, selector, true, func(act *domain.PendingAction) error {
		_, err := s.approveLoadedAction(ctx, tenantID, act, decidedBy)
		return err
	})
}

// RejectBatch applies the selector to pending actions and rejects each one with per-item failure isolation.
func (s *PendingActionService) RejectBatch(
	ctx context.Context,
	tenantID, decidedBy uuid.UUID,
	selector domain.PendingActionSelector,
	reason string,
) (*ports.BatchDecisionResult, error) {
	return s.runBatchDecision(ctx, tenantID, selector, false, func(act *domain.PendingAction) error {
		_, err := s.rejectLoadedAction(ctx, tenantID, act, decidedBy, reason)
		return err
	})
}

func (s *PendingActionService) runBatchDecision(
	ctx context.Context,
	tenantID uuid.UUID,
	selector domain.PendingActionSelector,
	isApprove bool,
	apply func(act *domain.PendingAction) error,
) (*ports.BatchDecisionResult, error) {
	allPending, err := s.ListPending(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	selected := domain.SelectPendingActions(allPending, selector)
	result := &ports.BatchDecisionResult{
		ProcessedCount: len(selected),
		Errors:         []string{},
	}

	for _, act := range selected {
		if applyErr := apply(act); applyErr != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("action %s: %v", act.ID, applyErr))
			continue
		}
		if isApprove {
			result.ApprovedCount++
		} else {
			result.RejectedCount++
		}
	}

	return result, nil
}

func (s *PendingActionService) loadTransaction(ctx context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, error) {
	tx, err := s.txRepo.GetByID(ctx, tenantID, txID)
	return ensureLoadedEntity(
		tx,
		err,
		func(t *domain.Transaction) uuid.UUID { return t.TenantID },
		tenantID,
		fmt.Sprintf("transaction %s", txID),
	)
}

func (s *PendingActionService) loadCategory(ctx context.Context, tenantID, catID uuid.UUID) (*domain.Category, error) {
	cat, err := s.catRepo.GetByID(ctx, tenantID, catID)
	return ensureLoadedEntity(
		cat,
		err,
		func(c *domain.Category) uuid.UUID { return c.TenantID },
		tenantID,
		fmt.Sprintf("category %s", catID),
	)
}

func (s *PendingActionService) loadInvoice(ctx context.Context, tenantID, invoiceID uuid.UUID) (*domain.CreditCardInvoice, error) {
	inv, err := s.invoiceRepo.FindByID(ctx, tenantID, invoiceID)
	return ensureLoadedEntity(
		inv,
		err,
		func(i *domain.CreditCardInvoice) uuid.UUID { return i.TenantID },
		tenantID,
		fmt.Sprintf("invoice %s", invoiceID),
	)
}

func (s *PendingActionService) loadAccount(ctx context.Context, tenantID, accountID uuid.UUID, role string) (*domain.Account, error) {
	acc, err := s.accountRepo.GetByID(ctx, tenantID, accountID)
	return ensureLoadedEntity(
		acc,
		err,
		func(a *domain.Account) uuid.UUID { return a.TenantID },
		tenantID,
		fmt.Sprintf("%s %s", role, accountID),
	)
}

func validatePendingActionForDecision(action *domain.PendingAction, tenantID uuid.UUID) error {
	var entityTenantID uuid.UUID
	if action != nil {
		entityTenantID = action.TenantID
	}
	if err := ensureTenantEntity(action == nil, entityTenantID, tenantID, ""); err != nil {
		return err
	}
	if action.Status != domain.ActionStatusPending {
		return domain.ErrPendingActionNotPending
	}
	return nil
}

func ensureLoadedEntity[T any](
	entity *T,
	loadErr error,
	tenantIDOf func(*T) uuid.UUID,
	expectedTenantID uuid.UUID,
	label string,
) (*T, error) {
	if loadErr != nil {
		return nil, fmt.Errorf("failed loading %s: %w", label, loadErr)
	}
	var entityTenantID uuid.UUID
	if entity != nil {
		entityTenantID = tenantIDOf(entity)
	}
	if err := ensureTenantEntity(entity == nil, entityTenantID, expectedTenantID, label); err != nil {
		return nil, err
	}
	return entity, nil
}

func ensureTenantEntity(isNil bool, entityTenantID, expectedTenantID uuid.UUID, label string) error {
	if isNil {
		if label == "" {
			return domain.ErrNotFound
		}
		return fmt.Errorf("%s: %w", label, domain.ErrNotFound)
	}
	if entityTenantID != uuid.Nil && entityTenantID != expectedTenantID {
		return domain.ErrUnauthorized
	}
	return nil
}
