package domain

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PendingActionType defines the category of mutation requiring human approval.
type PendingActionType string

// Pending action type definitions.
const (
	// ActionTypeCategorizeTransaction proposes setting category and metadata on a transaction.
	ActionTypeCategorizeTransaction PendingActionType = "CATEGORIZE_TRANSACTION"
	// ActionTypeSplitTransaction proposes dividing a transaction into sub-splits.
	ActionTypeSplitTransaction PendingActionType = "SPLIT_TRANSACTION"
	// ActionTypeReconcileTransfer proposes linking two peer transactions into an internal transfer.
	ActionTypeReconcileTransfer PendingActionType = "RECONCILE_TRANSFER"
	// ActionTypeBalanceAdjustment proposes reconciling external and internal account balances.
	ActionTypeBalanceAdjustment PendingActionType = "BALANCE_ADJUSTMENT"
)

// PendingActionStatus defines the decision state.
type PendingActionStatus string

// Pending action status definitions.
const (
	// ActionStatusPending indicates the action awaits human review and approval.
	ActionStatusPending PendingActionStatus = "PENDING"
	// ActionStatusApproved indicates the action was authorized by an authenticated user.
	ActionStatusApproved PendingActionStatus = "APPROVED"
	// ActionStatusRejected indicates the action was declined by an authenticated user.
	ActionStatusRejected PendingActionStatus = "REJECTED"
	// ActionStatusExecuted indicates the mutation has been committed to the ledger.
	ActionStatusExecuted PendingActionStatus = "EXECUTED"
)

// ReconciliationType classifies an internal transfer reconciliation proposal.
type ReconciliationType string

// Reconciliation type definitions.
const (
	// ReconciliationTypeInternalTransfer represents a cashflow-neutral transfer between own accounts.
	ReconciliationTypeInternalTransfer ReconciliationType = "INTERNAL_TRANSFER"
	// ReconciliationTypeBillPayment represents a credit card invoice settlement payment from a liquid account.
	ReconciliationTypeBillPayment ReconciliationType = "BILL_PAYMENT"
)

// IsValid reports whether the ReconciliationType is a supported reconciliation classification.
func (r ReconciliationType) IsValid() bool {
	switch r {
	case ReconciliationTypeInternalTransfer, ReconciliationTypeBillPayment:
		return true
	default:
		return false
	}
}

// PendingAction enforces Human-in-the-Loop governance for MCP and AI operations.
type PendingAction struct {
	ID               uuid.UUID           `json:"id"`
	TenantID         uuid.UUID           `json:"tenant_id"`
	CreatedByAgent   string              `json:"created_by_agent"`
	ActionType       PendingActionType   `json:"action_type"`
	Payload          map[string]any      `json:"payload"`
	Status           PendingActionStatus `json:"status"`
	RejectionReason  *string             `json:"rejection_reason,omitempty"`
	DecidedByUserID  *uuid.UUID          `json:"decided_by_user_id,omitempty"`
	DecidedAt        *time.Time          `json:"decided_at,omitempty"`
	ExecutedAt       *time.Time          `json:"executed_at,omitempty"`
	Metadata         map[string]any      `json:"metadata"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// NewPendingAction creates a proposed action in PENDING status.
func NewPendingAction(
	tenantID uuid.UUID,
	createdByAgent string,
	actionType PendingActionType,
	payload map[string]any,
) (*PendingAction, error) {
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("%w: tenant_id is required", ErrInvalidID)
	}
	trimmedAgent := strings.TrimSpace(createdByAgent)
	if trimmedAgent == "" {
		trimmedAgent = "system"
	}
	if strings.TrimSpace(string(actionType)) == "" {
		return nil, fmt.Errorf("%w: action_type is required", ErrUnsupportedPendingActionType)
	}
	if payload == nil {
		payload = make(map[string]any)
	}

	now := time.Now().UTC()
	return &PendingAction{
		ID:             uuid.New(),
		TenantID:       tenantID,
		CreatedByAgent: trimmedAgent,
		ActionType:     actionType,
		Payload:        payload,
		Status:         ActionStatusPending,
		Metadata:       make(map[string]any),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Approve transitions the action to APPROVED state with the deciding user ID.
func (p *PendingAction) Approve(decidedBy uuid.UUID) error {
	if p == nil || p.Status != ActionStatusPending {
		return ErrPendingActionNotPending
	}
	p.recordDecision(ActionStatusApproved, decidedBy)
	return nil
}

// Reject transitions the action to REJECTED state with an optional trimmed reason.
func (p *PendingAction) Reject(decidedBy uuid.UUID, reason string) error {
	if p == nil || p.Status != ActionStatusPending {
		return ErrPendingActionNotPending
	}
	p.recordDecision(ActionStatusRejected, decidedBy)
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		p.RejectionReason = nil
	} else {
		p.RejectionReason = &trimmedReason
	}
	return nil
}

func (p *PendingAction) recordDecision(status PendingActionStatus, decidedBy uuid.UUID) {
	now := time.Now().UTC()
	p.Status = status
	p.DecidedAt = &now
	if decidedBy != uuid.Nil {
		idCopy := decidedBy
		p.DecidedByUserID = &idCopy
	} else {
		p.DecidedByUserID = nil
	}
	p.UpdatedAt = now
}

// MarkExecuted marks an approved action as executed.
func (p *PendingAction) MarkExecuted() error {
	if p == nil || p.Status != ActionStatusApproved {
		return ErrPendingActionNotApproved
	}
	now := time.Now().UTC()
	p.Status = ActionStatusExecuted
	p.ExecutedAt = &now
	p.UpdatedAt = now
	return nil
}

// CategorizeTransactionProposal represents a typed proposal to assign a category to a transaction.
type CategorizeTransactionProposal struct {
	TransactionID    uuid.UUID
	Description      string
	CleanDescription string
	Amount           decimal.Decimal
	Date             string // YYYY-MM-DD (America/Sao_Paulo)
	CategoryID       uuid.UUID
	CategoryName     string
	Confidence       decimal.Decimal // [0.00, 1.00]
	Reason           string
}

// ValidateHeader checks that required identifiers and confidence bounds are valid before hydration.
func (p CategorizeTransactionProposal) ValidateHeader() error {
	if p.TransactionID == uuid.Nil {
		return fmt.Errorf("%w: transaction_id: %w", ErrInvalidPendingActionPayload, ErrInvalidID)
	}
	if p.CategoryID == uuid.Nil {
		return fmt.Errorf("%w: category_id: %w", ErrInvalidPendingActionPayload, ErrInvalidID)
	}
	if !isValidConfidence(p.Confidence) {
		return fmt.Errorf("%w: confidence must be between 0 and 1", ErrInvalidPendingActionPayload)
	}
	return nil
}

// Validate checks that required identifiers, monetary precision, and confidence bounds are valid.
func (p CategorizeTransactionProposal) Validate() error {
	if err := p.ValidateHeader(); err != nil {
		return err
	}
	if p.Amount.IsZero() || !IsRepresentableInCents(p.Amount) || p.Amount.Abs().GreaterThan(MaxMonetaryAmount) {
		return fmt.Errorf("%w: amount: %w", ErrInvalidPendingActionPayload, ErrInvalidAmount)
	}
	return nil
}

// DeduplicationKey returns the deterministic deduplication key for this categorization proposal.
func (p CategorizeTransactionProposal) DeduplicationKey() string {
	return fmt.Sprintf("categorize_%s", p.TransactionID)
}

// NeedsHydration reports whether display snapshot fields are missing and require hydration from entities.
func (p CategorizeTransactionProposal) NeedsHydration() bool {
	return strings.TrimSpace(p.Description) == "" ||
		strings.TrimSpace(p.CategoryName) == "" ||
		p.Amount.IsZero() ||
		strings.TrimSpace(p.Date) == ""
}

// NewCategorizeProposalFromEntities constructs a validated CategorizeTransactionProposal from domain entities.
func NewCategorizeProposalFromEntities(
	tx *Transaction,
	cat *Category,
	confidence decimal.Decimal,
	reason string,
) (CategorizeTransactionProposal, error) {
	if tx == nil || cat == nil {
		return CategorizeTransactionProposal{}, fmt.Errorf("%w: %w", ErrInvalidPendingActionPayload, ErrInvalidID)
	}
	if tx.TenantID != uuid.Nil && cat.TenantID != uuid.Nil && tx.TenantID != cat.TenantID {
		return CategorizeTransactionProposal{}, ErrUnauthorized
	}
	if tx.Status == TransactionStatusVoid {
		return CategorizeTransactionProposal{}, ErrInvalidStatus
	}
	if tx.Type == TransactionTypeTransfer {
		return CategorizeTransactionProposal{}, ErrInvalidTransactionType
	}

	trimmedDesc := strings.TrimSpace(tx.Description)
	cleanDesc := strings.TrimSpace(tx.CleanDescription)
	if cleanDesc == "" {
		cleanDesc = CleanDescription(trimmedDesc)
	}

	proposal := CategorizeTransactionProposal{
		TransactionID:    tx.ID,
		Description:      trimmedDesc,
		CleanDescription: cleanDesc,
		Amount:           tx.Amount,
		Date:             StandardizeDateToBrazil(tx.Date),
		CategoryID:       cat.ID,
		CategoryName:     strings.TrimSpace(cat.Name),
		Confidence:       confidence,
		Reason:           strings.TrimSpace(reason),
	}
	if err := proposal.Validate(); err != nil {
		return CategorizeTransactionProposal{}, err
	}
	return proposal, nil
}

// NewCategorizePendingAction builds a PENDING action with frontend-compatible payload keys and deduplication metadata.
func NewCategorizePendingAction(
	tenantID uuid.UUID,
	createdByAgent string,
	proposal CategorizeTransactionProposal,
) (*PendingAction, error) {
	if err := proposal.Validate(); err != nil {
		return nil, err
	}

	payload := map[string]any{
		"transaction_id":    proposal.TransactionID.String(),
		"description":       strings.TrimSpace(proposal.Description),
		"clean_description": strings.TrimSpace(proposal.CleanDescription),
		"amount":            proposal.Amount.StringFixed(2),
		"date":              strings.TrimSpace(proposal.Date),
		"category_id":       proposal.CategoryID.String(),
		"category_name":     strings.TrimSpace(proposal.CategoryName),
		"confidence":        proposal.Confidence.StringFixed(2),
		"reason":            strings.TrimSpace(proposal.Reason),
	}

	action, err := NewPendingAction(tenantID, createdByAgent, ActionTypeCategorizeTransaction, payload)
	if err != nil {
		return nil, err
	}
	action.Metadata["deduplication_key"] = proposal.DeduplicationKey()
	action.Metadata["confidence"] = proposal.Confidence.StringFixed(2)
	return action, nil
}

// DecodeCategorizePayload validates and decodes a CATEGORIZE_TRANSACTION pending action payload.
func DecodeCategorizePayload(action *PendingAction) (CategorizeTransactionProposal, error) {
	if err := validateActionEnvelope(action, ActionTypeCategorizeTransaction); err != nil {
		return CategorizeTransactionProposal{}, err
	}

	txID, err := parseRequiredUUID(action.Payload, "transaction_id")
	if err != nil {
		return CategorizeTransactionProposal{}, err
	}
	catID, err := parseRequiredUUID(action.Payload, "category_id")
	if err != nil {
		return CategorizeTransactionProposal{}, err
	}

	amount, err := parseOptionalMonetaryAmount(action.Payload, false)
	if err != nil {
		return CategorizeTransactionProposal{}, err
	}

	confidence, hasConfidence, err := parseOptionalConfidence(action.Payload, "confidence")
	if err != nil {
		return CategorizeTransactionProposal{}, fmt.Errorf("%w: invalid confidence: %v", ErrInvalidPendingActionPayload, err)
	}
	if hasConfidence && !isValidConfidence(confidence) {
		return CategorizeTransactionProposal{}, fmt.Errorf("%w: confidence must be between 0 and 1", ErrInvalidPendingActionPayload)
	}

	return CategorizeTransactionProposal{
		TransactionID:    txID,
		Description:      optionalPayloadString(action.Payload, "description"),
		CleanDescription: optionalPayloadString(action.Payload, "clean_description"),
		Amount:           amount,
		Date:             optionalPayloadString(action.Payload, "date"),
		CategoryID:       catID,
		CategoryName:     optionalPayloadString(action.Payload, "category_name"),
		Confidence:       confidence,
		Reason:           optionalPayloadString(action.Payload, "reason"),
	}, nil
}

func validateActionEnvelope(action *PendingAction, expectedType PendingActionType) error {
	if action == nil || action.Payload == nil {
		return fmt.Errorf("%w: pending action payload is required", ErrInvalidPendingActionPayload)
	}
	if action.ActionType != expectedType {
		return fmt.Errorf("%w: unexpected action type %q", ErrInvalidPendingActionPayload, action.ActionType)
	}
	return nil
}

func parseRequiredUUID(payload map[string]any, key string) (uuid.UUID, error) {
	parsed, ok, err := parseOptionalUUID(payload, key, true)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, fmt.Errorf("%w: %s: %w", ErrInvalidPendingActionPayload, key, ErrInvalidID)
	}
	return parsed, nil
}

func parseOptionalMonetaryAmount(payload map[string]any, requirePositive bool) (decimal.Decimal, error) {
	amount, hasAmount, err := parseOptionalExactDecimal(payload, "amount")
	if err != nil {
		return decimal.Zero, fmt.Errorf("%w: amount: %w", ErrInvalidPendingActionPayload, ErrInvalidAmount)
	}
	if !hasAmount {
		return decimal.Zero, nil
	}
	if amount.IsZero() || (requirePositive && !amount.IsPositive()) || !IsRepresentableInCents(amount) || amount.Abs().GreaterThan(MaxMonetaryAmount) {
		return decimal.Zero, fmt.Errorf("%w: amount: %w", ErrInvalidPendingActionPayload, ErrInvalidAmount)
	}
	return amount, nil
}

func parseDecimalString(raw string) (decimal.Decimal, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return decimal.Zero, false, nil
	}
	parsed, parseErr := decimal.NewFromString(trimmed)
	if parseErr != nil {
		return decimal.Zero, false, parseErr
	}
	return parsed, true, nil
}

func parseOptionalExactDecimal(payload map[string]any, key string) (decimal.Decimal, bool, error) {
	raw, exists := payload[key]
	if !exists || raw == nil {
		return decimal.Zero, false, nil
	}

	switch v := raw.(type) {
	case string:
		return parseDecimalString(v)
	case json.Number:
		return parseDecimalString(v.String())
	case int:
		return decimal.NewFromInt(int64(v)), true, nil
	case int64:
		return decimal.NewFromInt(v), true, nil
	case decimal.Decimal:
		return v, true, nil
	default:
		return decimal.Zero, false, ErrInvalidAmount
	}
}

func parseOptionalConfidence(payload map[string]any, key string) (decimal.Decimal, bool, error) {
	raw, exists := payload[key]
	if !exists || raw == nil {
		return decimal.Zero, false, nil
	}

	if f, isFloat := raw.(float64); isFloat {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return decimal.Zero, false, ErrInvalidAmount
		}
		return decimal.NewFromFloat(f), true, nil
	}
	return parseOptionalExactDecimal(payload, key)
}

func optionalPayloadString(payload map[string]any, key string) string {
	if s, ok := payload[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// ReconcileTransferProposal represents a typed proposal to link an outflow leg and an inflow leg
// as an internal transfer or credit card invoice settlement.
type ReconcileTransferProposal struct {
	OutflowTransactionID   uuid.UUID
	InflowTransactionID    uuid.UUID
	Amount                 decimal.Decimal // Positive absolute amount
	SourceAccountID        uuid.UUID
	SourceAccountName      string
	DestinationAccountID   uuid.UUID
	DestinationAccountName string
	OutflowDate            string
	InflowDate             string
	ReconciliationType     ReconciliationType
	Description            string
	InvoiceID              *uuid.UUID
}

// Validate checks that leg identifiers, optional account/invoice IDs, monetary precision, and reconciliation type are valid.
func (p ReconcileTransferProposal) Validate() error {
	if p.OutflowTransactionID == uuid.Nil {
		return fmt.Errorf("%w: outflow_transaction_id: %w", ErrInvalidPendingActionPayload, ErrInvalidID)
	}
	if p.InflowTransactionID == uuid.Nil {
		return fmt.Errorf("%w: inflow_transaction_id: %w", ErrInvalidPendingActionPayload, ErrInvalidID)
	}
	if p.OutflowTransactionID == p.InflowTransactionID {
		return fmt.Errorf("%w: %w", ErrInvalidPendingActionPayload, ErrInvalidTransferPair)
	}
	if p.SourceAccountID != uuid.Nil && p.DestinationAccountID != uuid.Nil && p.SourceAccountID == p.DestinationAccountID {
		return fmt.Errorf("%w: %w", ErrInvalidPendingActionPayload, ErrTransferSameAccount)
	}
	if !p.Amount.IsPositive() || !IsRepresentableInCents(p.Amount) || p.Amount.GreaterThan(MaxMonetaryAmount) {
		return fmt.Errorf("%w: amount: %w", ErrInvalidPendingActionPayload, ErrInvalidAmount)
	}
	if !p.ReconciliationType.IsValid() {
		return fmt.Errorf("%w: invalid reconciliation_type %q", ErrInvalidPendingActionPayload, p.ReconciliationType)
	}
	if p.InvoiceID != nil && *p.InvoiceID == uuid.Nil {
		return fmt.Errorf("%w: invoice_id: %w", ErrInvalidPendingActionPayload, ErrInvalidID)
	}
	return nil
}

// DeduplicationKey returns the deterministic deduplication key for this transfer reconciliation proposal.
func (p ReconcileTransferProposal) DeduplicationKey() string {
	return fmt.Sprintf("reconcile_%s_%s", p.OutflowTransactionID, p.InflowTransactionID)
}

// NewReconcileTransferProposalFromEntities constructs a validated ReconcileTransferProposal from matched legs and accounts.
func NewReconcileTransferProposalFromEntities(
	outflow, inflow *Transaction,
	srcAcc, dstAcc *Account,
	invoiceID *uuid.UUID,
) (ReconcileTransferProposal, error) {
	if outflow == nil || inflow == nil {
		return ReconcileTransferProposal{}, fmt.Errorf("%w: %w", ErrInvalidPendingActionPayload, ErrInvalidTransferPair)
	}
	if outflow.ID == uuid.Nil || inflow.ID == uuid.Nil {
		return ReconcileTransferProposal{}, fmt.Errorf("%w: %w", ErrInvalidPendingActionPayload, ErrInvalidID)
	}
	if outflow.ID == inflow.ID {
		return ReconcileTransferProposal{}, fmt.Errorf("%w: %w", ErrInvalidPendingActionPayload, ErrInvalidTransferPair)
	}
	if outflow.TenantID != uuid.Nil && inflow.TenantID != uuid.Nil && outflow.TenantID != inflow.TenantID {
		return ReconcileTransferProposal{}, ErrTransferCrossTenant
	}
	if outflow.AccountID == inflow.AccountID {
		return ReconcileTransferProposal{}, ErrTransferSameAccount
	}
	if outflow.Status == TransactionStatusVoid || inflow.Status == TransactionStatusVoid {
		return ReconcileTransferProposal{}, ErrTransferVoidLeg
	}
	if !outflow.Amount.IsNegative() || !inflow.Amount.IsPositive() || !outflow.Amount.Add(inflow.Amount).IsZero() {
		return ReconcileTransferProposal{}, ErrTransferAmountMismatch
	}

	srcName := "Conta de Origem"
	if srcAcc != nil && strings.TrimSpace(srcAcc.Name) != "" {
		srcName = strings.TrimSpace(srcAcc.Name)
	}
	dstName := "Conta de Destino"
	if dstAcc != nil && strings.TrimSpace(dstAcc.Name) != "" {
		dstName = strings.TrimSpace(dstAcc.Name)
	}

	recType := ReconciliationTypeInternalTransfer
	if (dstAcc != nil && dstAcc.Type == AccountTypeCreditCard) || (invoiceID != nil && *invoiceID != uuid.Nil) {
		recType = ReconciliationTypeBillPayment
	}

	absAmount := inflow.Amount
	var desc string
	if recType == ReconciliationTypeBillPayment {
		desc = fmt.Sprintf("Pagamento de fatura do %s (R$ %s) a partir de %s", dstName, absAmount.StringFixed(2), srcName)
	} else {
		desc = fmt.Sprintf("Transferência de R$ %s de %s para %s", absAmount.StringFixed(2), srcName, dstName)
	}

	var invCopy *uuid.UUID
	if invoiceID != nil {
		idVal := *invoiceID
		invCopy = &idVal
	}

	proposal := ReconcileTransferProposal{
		OutflowTransactionID:   outflow.ID,
		InflowTransactionID:    inflow.ID,
		Amount:                 absAmount,
		SourceAccountID:        outflow.AccountID,
		SourceAccountName:      srcName,
		DestinationAccountID:   inflow.AccountID,
		DestinationAccountName: dstName,
		OutflowDate:            StandardizeDateToBrazil(outflow.Date),
		InflowDate:             StandardizeDateToBrazil(inflow.Date),
		ReconciliationType:     recType,
		Description:            desc,
		InvoiceID:              invCopy,
	}
	if err := proposal.Validate(); err != nil {
		return ReconcileTransferProposal{}, err
	}
	return proposal, nil
}

// NewReconcileTransferPendingAction builds a PENDING action with frontend-compatible payload keys and deduplication metadata.
func NewReconcileTransferPendingAction(
	tenantID uuid.UUID,
	createdByAgent string,
	proposal ReconcileTransferProposal,
) (*PendingAction, error) {
	if err := proposal.Validate(); err != nil {
		return nil, err
	}

	payload := map[string]any{
		"outflow_transaction_id":   proposal.OutflowTransactionID.String(),
		"inflow_transaction_id":    proposal.InflowTransactionID.String(),
		"amount":                   proposal.Amount.StringFixed(2),
		"source_account_id":        proposal.SourceAccountID.String(),
		"source_account_name":      strings.TrimSpace(proposal.SourceAccountName),
		"destination_account_id":   proposal.DestinationAccountID.String(),
		"destination_account_name": strings.TrimSpace(proposal.DestinationAccountName),
		"outflow_date":             strings.TrimSpace(proposal.OutflowDate),
		"inflow_date":              strings.TrimSpace(proposal.InflowDate),
		"reconciliation_type":      string(proposal.ReconciliationType),
		"description":              strings.TrimSpace(proposal.Description),
	}
	if proposal.InvoiceID != nil {
		payload["invoice_id"] = proposal.InvoiceID.String()
	}

	action, err := NewPendingAction(tenantID, createdByAgent, ActionTypeReconcileTransfer, payload)
	if err != nil {
		return nil, err
	}
	action.Metadata["deduplication_key"] = proposal.DeduplicationKey()
	return action, nil
}

// DecodeReconcileTransferPayload validates and decodes a RECONCILE_TRANSFER pending action payload.
func DecodeReconcileTransferPayload(action *PendingAction) (ReconcileTransferProposal, error) {
	if err := validateActionEnvelope(action, ActionTypeReconcileTransfer); err != nil {
		return ReconcileTransferProposal{}, err
	}

	outflowID, err := parseRequiredUUID(action.Payload, "outflow_transaction_id")
	if err != nil {
		return ReconcileTransferProposal{}, err
	}
	inflowID, err := parseRequiredUUID(action.Payload, "inflow_transaction_id")
	if err != nil {
		return ReconcileTransferProposal{}, err
	}
	if outflowID == inflowID {
		return ReconcileTransferProposal{}, fmt.Errorf("%w: %w", ErrInvalidPendingActionPayload, ErrInvalidTransferPair)
	}

	srcAccID, _, err := parseOptionalUUID(action.Payload, "source_account_id", false)
	if err != nil {
		return ReconcileTransferProposal{}, err
	}
	dstAccID, _, err := parseOptionalUUID(action.Payload, "destination_account_id", false)
	if err != nil {
		return ReconcileTransferProposal{}, err
	}
	if srcAccID != uuid.Nil && dstAccID != uuid.Nil && srcAccID == dstAccID {
		return ReconcileTransferProposal{}, fmt.Errorf("%w: %w", ErrInvalidPendingActionPayload, ErrTransferSameAccount)
	}

	var invoiceID *uuid.UUID
	parsedInvID, hasInvID, err := parseOptionalUUID(action.Payload, "invoice_id", true)
	if err != nil {
		return ReconcileTransferProposal{}, err
	}
	if hasInvID {
		invCopy := parsedInvID
		invoiceID = &invCopy
	}

	amount, err := parseOptionalMonetaryAmount(action.Payload, true)
	if err != nil {
		return ReconcileTransferProposal{}, err
	}

	recType := ReconciliationTypeInternalTransfer
	if invoiceID != nil {
		recType = ReconciliationTypeBillPayment
	}
	if rawType, hasType := action.Payload["reconciliation_type"]; hasType && rawType != nil {
		rawTypeStr, ok := rawType.(string)
		if !ok {
			return ReconcileTransferProposal{}, fmt.Errorf("%w: invalid reconciliation_type", ErrInvalidPendingActionPayload)
		}
		parsedType := ReconciliationType(strings.TrimSpace(rawTypeStr))
		if !parsedType.IsValid() {
			return ReconcileTransferProposal{}, fmt.Errorf("%w: invalid reconciliation_type %q", ErrInvalidPendingActionPayload, parsedType)
		}
		recType = parsedType
	}

	return ReconcileTransferProposal{
		OutflowTransactionID:   outflowID,
		InflowTransactionID:    inflowID,
		Amount:                 amount,
		SourceAccountID:        srcAccID,
		SourceAccountName:      optionalPayloadString(action.Payload, "source_account_name"),
		DestinationAccountID:   dstAccID,
		DestinationAccountName: optionalPayloadString(action.Payload, "destination_account_name"),
		OutflowDate:            optionalPayloadString(action.Payload, "outflow_date"),
		InflowDate:             optionalPayloadString(action.Payload, "inflow_date"),
		ReconciliationType:     recType,
		Description:            optionalPayloadString(action.Payload, "description"),
		InvoiceID:              invoiceID,
	}, nil
}

func parseOptionalUUID(payload map[string]any, key string, rejectNil bool) (uuid.UUID, bool, error) {
	raw, exists := payload[key]
	if !exists || raw == nil {
		return uuid.Nil, false, nil
	}
	s, ok := raw.(string)
	if !ok {
		return uuid.Nil, false, fmt.Errorf("%w: %s: %w", ErrInvalidPendingActionPayload, key, ErrInvalidID)
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		if rejectNil {
			return uuid.Nil, false, fmt.Errorf("%w: %s: %w", ErrInvalidPendingActionPayload, key, ErrInvalidID)
		}
		return uuid.Nil, false, nil
	}
	parsed, parseErr := uuid.Parse(trimmed)
	if parseErr != nil || (rejectNil && parsed == uuid.Nil) {
		return uuid.Nil, false, fmt.Errorf("%w: %s: %w", ErrInvalidPendingActionPayload, key, ErrInvalidID)
	}
	return parsed, true, nil
}

// PendingActionIndex provides O(1) in-memory deduplication lookups over active PENDING actions.
type PendingActionIndex struct {
	categorizations map[uuid.UUID]struct{}
	transferLegs    map[uuid.UUID]struct{}
	dedupKeys       map[string]struct{}
}

// NewPendingActionIndex builds an O(1) lookup index from existing pending actions.
func NewPendingActionIndex(actions []*PendingAction) *PendingActionIndex {
	idx := &PendingActionIndex{
		categorizations: make(map[uuid.UUID]struct{}),
		transferLegs:    make(map[uuid.UUID]struct{}),
		dedupKeys:       make(map[string]struct{}),
	}
	for _, act := range actions {
		idx.Record(act)
	}
	return idx
}

// Record indexes an active PENDING action by deduplication key and target transaction/leg UUIDs.
// Nil or non-PENDING actions are ignored.
func (idx *PendingActionIndex) Record(action *PendingAction) {
	if idx == nil || action == nil || action.Status != ActionStatusPending {
		return
	}
	if idx.categorizations == nil {
		idx.categorizations = make(map[uuid.UUID]struct{})
	}
	if idx.transferLegs == nil {
		idx.transferLegs = make(map[uuid.UUID]struct{})
	}
	if idx.dedupKeys == nil {
		idx.dedupKeys = make(map[string]struct{})
	}

	if action.Metadata != nil {
		if rawKey, ok := action.Metadata["deduplication_key"].(string); ok {
			if key := strings.TrimSpace(rawKey); key != "" {
				idx.dedupKeys[key] = struct{}{}
			}
		}
	}

	if action.Payload == nil {
		return
	}

	switch action.ActionType {
	case ActionTypeCategorizeTransaction:
		if txID, hasTx, parseErr := parseOptionalUUID(action.Payload, "transaction_id", true); parseErr == nil && hasTx {
			idx.categorizations[txID] = struct{}{}
			idx.dedupKeys[(CategorizeTransactionProposal{TransactionID: txID}).DeduplicationKey()] = struct{}{}
		}
	case ActionTypeReconcileTransfer:
		outID, hasOut, outErr := parseOptionalUUID(action.Payload, "outflow_transaction_id", true)
		if outErr == nil && hasOut {
			idx.transferLegs[outID] = struct{}{}
		}
		inID, hasIn, inErr := parseOptionalUUID(action.Payload, "inflow_transaction_id", true)
		if inErr == nil && hasIn {
			idx.transferLegs[inID] = struct{}{}
		}
		if outErr == nil && hasOut && inErr == nil && hasIn {
			idx.dedupKeys[(ReconcileTransferProposal{
				OutflowTransactionID: outID,
				InflowTransactionID:  inID,
			}).DeduplicationKey()] = struct{}{}
		}
	}
}

// HasCategorizationFor reports whether an active PENDING categorization exists for txID.
func (idx *PendingActionIndex) HasCategorizationFor(txID uuid.UUID) bool {
	if idx == nil || txID == uuid.Nil {
		return false
	}
	_, exists := idx.categorizations[txID]
	return exists
}

// HasTransferLeg reports whether txID is already referenced as an outflow or inflow leg in a PENDING transfer proposal.
func (idx *PendingActionIndex) HasTransferLeg(txID uuid.UUID) bool {
	if idx == nil || txID == uuid.Nil {
		return false
	}
	_, exists := idx.transferLegs[txID]
	return exists
}

// HasDeduplicationKey reports whether an active PENDING action is indexed under key.
func (idx *PendingActionIndex) HasDeduplicationKey(key string) bool {
	if idx == nil {
		return false
	}
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}
	_, exists := idx.dedupKeys[trimmed]
	return exists
}

// PendingActionSelector configures deterministic filtering for batch pending action decisions.
type PendingActionSelector struct {
	ActionIDs  []uuid.UUID
	All        bool
	ActionType *PendingActionType
}

// SelectPendingActions filters PENDING actions preserving input order, enforcing optional ActionType,
// matching All or ActionIDs membership, and deduplicating by action ID.
func SelectPendingActions(actions []*PendingAction, selector PendingActionSelector) []*PendingAction {
	selectedSet := make(map[uuid.UUID]struct{}, len(selector.ActionIDs))
	if !selector.All {
		for _, id := range selector.ActionIDs {
			if id != uuid.Nil {
				selectedSet[id] = struct{}{}
			}
		}
		if len(selectedSet) == 0 {
			return []*PendingAction{}
		}
	}

	seen := make(map[uuid.UUID]struct{}, len(actions))
	selected := make([]*PendingAction, 0)

	for _, act := range actions {
		if act == nil || act.Status != ActionStatusPending {
			continue
		}
		if selector.ActionType != nil && act.ActionType != *selector.ActionType {
			continue
		}
		if !selector.All {
			if _, wanted := selectedSet[act.ID]; !wanted {
				continue
			}
		}
		if _, dup := seen[act.ID]; dup {
			continue
		}
		seen[act.ID] = struct{}{}
		selected = append(selected, act)
	}

	return selected
}
