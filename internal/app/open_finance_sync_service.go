package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

var _ ports.OpenFinanceSyncService = (*OpenFinanceSyncService)(nil)

// OpenFinanceSyncService orchestrates Open Finance account synchronization, Provider Balance
// Snapshot persistence, Invoice cycle mapping, Ledger Balance anchoring, Invoice Settlement
// reconciliation, and post-sync Pending Action generation.
type OpenFinanceSyncService struct {
	connector          ports.OpenFinanceConnector
	accountRepo        ports.AccountRepository
	txRepo             ports.TransactionRepository
	invRepo            ports.InvoiceRepository
	snapRepo           ports.ProviderBalanceSnapshotRepository
	transferReconciler ports.TransferReconciliationService
	categoryMatcher    ports.CategoryRuleService
	logger             *slog.Logger
}

// NewOpenFinanceSyncService constructs a deep OpenFinanceSyncService module.
func NewOpenFinanceSyncService(
	connector ports.OpenFinanceConnector,
	accountRepo ports.AccountRepository,
	txRepo ports.TransactionRepository,
	invRepo ports.InvoiceRepository,
	snapRepo ports.ProviderBalanceSnapshotRepository,
	transferReconciler ports.TransferReconciliationService,
	categoryMatcher ports.CategoryRuleService,
	logger *slog.Logger,
) *OpenFinanceSyncService {
	if logger == nil {
		logger = slog.Default()
	}
	return &OpenFinanceSyncService{
		connector:          connector,
		accountRepo:        accountRepo,
		txRepo:             txRepo,
		invRepo:            invRepo,
		snapRepo:           snapRepo,
		transferReconciler: transferReconciler,
		categoryMatcher:    categoryMatcher,
		logger:             logger,
	}
}

type accountSyncState struct {
	account        *domain.Account
	isNew          bool
	extAccount     ports.ExternalAccount
	invoiceIDMap   map[string]uuid.UUID
	invoiceByMonth map[string]*domain.CreditCardInvoice
	closingDay     int
	dueDay         int
	cardPayments   []domain.CardPaymentObservation
	rawTxInputs    []domain.ProviderTxInput
}

// SyncItem executes a complete idempotent Open Finance synchronization cycle for one item.
func (s *OpenFinanceSyncService) SyncItem(
	ctx context.Context,
	req ports.SyncItemRequest,
) (*ports.SyncItemResult, error) {
	if req.TenantID == uuid.Nil || strings.TrimSpace(req.ItemID) == "" {
		return nil, domain.ErrInvalidID
	}
	if s.connector == nil {
		return nil, errors.New("open finance connector is required")
	}

	refNow := req.ReferenceTime
	if refNow.IsZero() {
		refNow = time.Now().UTC()
	}

	extAccounts, err := s.connector.FetchAccounts(ctx, req.ItemID)
	if err != nil {
		return nil, fmt.Errorf("failed fetching external accounts: %w", err)
	}

	existingAccounts, err := s.accountRepo.List(ctx, req.TenantID, ports.AccountFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed listing existing accounts: %w", err)
	}
	accountMap := indexAccountsByLookupKeys(existingAccounts)

	result := &ports.SyncItemResult{}
	var creditCardIDs []uuid.UUID
	cardPaymentsByAccount := make(map[uuid.UUID][]domain.CardPaymentObservation)

	for _, extAcc := range extAccounts {
		state, accErr := s.syncAccount(ctx, req, extAcc, accountMap, refNow, result)
		if accErr != nil {
			return nil, accErr
		}
		if state == nil {
			continue
		}
		if extAcc.Type == ports.ExternalAccountCreditCard {
			creditCardIDs = append(creditCardIDs, state.account.ID)
			cardPaymentsByAccount[state.account.ID] = state.cardPayments
		}
	}

	if err := s.reconcileCreditCardSettlements(ctx, req.TenantID, creditCardIDs, cardPaymentsByAccount, result); err != nil {
		return nil, err
	}

	if err := s.runPostSyncScans(ctx, req.TenantID, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *OpenFinanceSyncService) syncAccount(
	ctx context.Context,
	req ports.SyncItemRequest,
	extAcc ports.ExternalAccount,
	accountMap map[string]*domain.Account,
	refNow time.Time,
	result *ports.SyncItemResult,
) (*accountSyncState, error) {
	internalAcc, isNew, err := s.resolveOrCreateAccount(ctx, req, extAcc, accountMap)
	if err != nil {
		return nil, err
	}
	result.AccountsSynced++

	if err := s.saveProviderSnapshot(ctx, req, extAcc, internalAcc, refNow); err != nil {
		return nil, err
	}
	result.SnapshotsSaved++

	state := &accountSyncState{
		account:        internalAcc,
		isNew:          isNew,
		extAccount:     extAcc,
		invoiceIDMap:   make(map[string]uuid.UUID),
		invoiceByMonth: make(map[string]*domain.CreditCardInvoice),
		closingDay:     8,
		dueDay:         15,
	}

	if extAcc.Type == ports.ExternalAccountCreditCard {
		upserted, err := s.syncCreditCardInvoices(ctx, req.TenantID, state)
		if err != nil {
			return nil, err
		}
		result.InvoicesUpserted += upserted
	}

	if err := s.ingestAccountTransactions(ctx, req, state, refNow, result); err != nil {
		return nil, err
	}

	if isNew && internalAcc.IsLiquidAccount() {
		if err := s.anchorInitialLiquidBalance(ctx, req.TenantID, state, refNow); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func (s *OpenFinanceSyncService) resolveOrCreateAccount(
	ctx context.Context,
	req ports.SyncItemRequest,
	extAcc ports.ExternalAccount,
	accountMap map[string]*domain.Account,
) (*domain.Account, bool, error) {
	accName := fmt.Sprintf("Pluggy %s", extAcc.Name)
	if extAcc.Number != "" {
		if existing, ok := accountMap[extAcc.Number]; ok {
			return existing, false, nil
		}
	}
	if existing, ok := accountMap[accName]; ok {
		return existing, false, nil
	}

	newAcc, err := domain.NewAccount(
		req.TenantID,
		req.UserID,
		accName,
		mapExternalAccountType(extAcc.Type),
		extAcc.CurrencyCode,
		domain.VisibilityShared,
	)
	if err != nil {
		return nil, false, fmt.Errorf("failed constructing account %q: %w", accName, err)
	}
	newAcc.AccountNumberMasked = extAcc.Number
	newAcc.InstitutionID = extAcc.ID
	if strings.TrimSpace(extAcc.BankName) != "" {
		newAcc.InstitutionName = strings.TrimSpace(extAcc.BankName)
	} else {
		newAcc.InstitutionName = "Pluggy Bank Sandbox"
	}

	if err := s.accountRepo.Create(ctx, newAcc); err != nil {
		return nil, false, fmt.Errorf("failed saving new account %q: %w", accName, err)
	}
	if extAcc.Number != "" {
		accountMap[extAcc.Number] = newAcc
	}
	accountMap[newAcc.Name] = newAcc
	return newAcc, true, nil
}

func (s *OpenFinanceSyncService) saveProviderSnapshot(
	ctx context.Context,
	req ports.SyncItemRequest,
	extAcc ports.ExternalAccount,
	internalAcc *domain.Account,
	refNow time.Time,
) error {
	if s.snapRepo == nil {
		return nil
	}
	snap, err := domain.NewProviderBalanceSnapshot(
		req.TenantID,
		internalAcc.ID,
		"PLUGGY",
		extAcc.Balance,
		extAcc.CurrencyCode,
		refNow,
	)
	if err != nil {
		return fmt.Errorf("failed constructing provider balance snapshot: %w", err)
	}
	snap.Metadata["provider_account_id"] = extAcc.ID
	snap.Metadata["item_id"] = req.ItemID
	if err := s.snapRepo.Save(ctx, snap); err != nil {
		return fmt.Errorf("failed saving provider balance snapshot for account %s: %w", internalAcc.ID, err)
	}
	return nil
}

func (s *OpenFinanceSyncService) syncCreditCardInvoices(
	ctx context.Context,
	tenantID uuid.UUID,
	state *accountSyncState,
) (int, error) {
	existingInvs, err := s.invRepo.ListByAccount(ctx, tenantID, state.account.ID)
	if err != nil {
		return 0, fmt.Errorf("failed listing existing invoices for account %s: %w", state.account.ID, err)
	}
	existingByExtID := make(map[string]*domain.CreditCardInvoice, len(existingInvs))
	for _, ei := range existingInvs {
		if ei.ExternalID != nil && *ei.ExternalID != "" {
			existingByExtID[*ei.ExternalID] = ei
		}
		state.invoiceByMonth[ei.DueDate.Format("2006-01")] = ei
	}

	bills, err := s.connector.FetchBills(ctx, state.extAccount.ID)
	if err != nil {
		return 0, fmt.Errorf("failed fetching external bills for account %s: %w", state.extAccount.ID, err)
	}
	if len(bills) == 0 {
		return 0, nil
	}

	if bills[0].ClosingDate != nil {
		state.closingDay = domain.ExtractBrazilCalendarDay(*bills[0].ClosingDate, 8)
	}
	state.dueDay = domain.ExtractBrazilCalendarDay(bills[0].DueDate, 15)

	upserted := 0
	for _, b := range bills {
		var existingInv *domain.CreditCardInvoice
		if ei, ok := existingByExtID[b.ID]; ok {
			existingInv = ei
		} else if ei, ok := state.invoiceByMonth[b.DueDate.Format("2006-01")]; ok {
			existingInv = ei
		}

		status := domain.ResolveProviderInvoiceStatus(b.Status, b.PaymentsCount, existingInv)
		inv, iErr := domain.NewCreditCardInvoice(tenantID, state.account.ID, b.DueDate, b.TotalAmount, status)
		if iErr != nil {
			return upserted, fmt.Errorf("failed building invoice %q: %w", b.ID, iErr)
		}
		if existingInv != nil {
			inv.ID = existingInv.ID
			inv.Metadata = existingInv.Metadata
		}
		extIDCopy := b.ID
		inv.ExternalID = &extIDCopy
		inv.ClosingDate = b.ClosingDate
		if b.MinimumPaymentAmount != nil {
			inv.MinimumPayment = *b.MinimumPaymentAmount
		}
		inv.AllowsInstallments = b.AllowsInstallments

		if err := s.invRepo.Save(ctx, inv); err != nil {
			return upserted, fmt.Errorf("failed saving credit card invoice %q: %w", b.ID, err)
		}
		upserted++

		savedInv, findErr := s.invRepo.FindByExternalID(ctx, tenantID, b.ID)
		if findErr != nil && !errors.Is(findErr, domain.ErrNotFound) {
			return upserted, fmt.Errorf("failed reloading credit card invoice %q: %w", b.ID, findErr)
		}
		if savedInv != nil {
			inv = savedInv
		}
		state.invoiceIDMap[b.ID] = inv.ID
		state.invoiceByMonth[inv.DueDate.Format("2006-01")] = inv
	}

	return upserted, nil
}

type preparedProviderTx struct {
	domainTx   *domain.Transaction
	externalID string
}

func (s *OpenFinanceSyncService) ingestAccountTransactions(
	ctx context.Context,
	req ports.SyncItemRequest,
	state *accountSyncState,
	refNow time.Time,
	result *ports.SyncItemResult,
) error {
	extTxs, err := s.connector.FetchTransactions(ctx, state.extAccount.ID, time.Time{}, time.Time{})
	if err != nil {
		return fmt.Errorf("failed fetching transactions for account %s: %w", state.extAccount.ID, err)
	}

	syntheticTotals := make(map[uuid.UUID]decimal.Decimal)
	prepared := make([]preparedProviderTx, 0, len(extTxs))
	fingerprints := make([]string, 0, len(extTxs))
	domainAccType := state.account.Type

	var createdBy *uuid.UUID
	if req.UserID != uuid.Nil {
		createdBy = &req.UserID
	}

	for _, extTx := range extTxs {
		rawInput := domain.ProviderTxInput{
			RawAmount:    extTx.Amount,
			Description:  extTx.Description,
			CategoryHint: extTx.CategoryHint,
			Date:         extTx.Date,
		}
		state.rawTxInputs = append(state.rawTxInputs, rawInput)

		norm := domain.NormalizeProviderTransaction(domainAccType, rawInput, refNow)

		txMeta := map[string]any{
			"provider":         "PLUGGY",
			"payment_method":   extTx.PaymentMethod,
			"category_hint":    extTx.CategoryHint,
			"installment_no":   extTx.InstallmentNumber,
			"installments_tot": extTx.TotalInstallments,
		}
		if norm.IsInvoiceSettlement {
			txMeta["is_bill_payment"] = true
		}
		if norm.IsFutureScheduled {
			txMeta["is_future"] = true
		}

		// Validate domain invariants before creating synthetic Invoices so invalid rows never orphan cycles.
		newTx, txErr := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:        req.TenantID,
			AccountID:       state.account.ID,
			CreatedByUserID: createdBy,
			TransactedAt:    extTx.Date,
			Date:            extTx.Date,
			Amount:          norm.Amount,
			Description:     extTx.Description,
			Type:            norm.Type,
			Status:          norm.Status,
			Visibility:      domain.VisibilityShared,
			Source:          domain.SourceTypePluggy,
			ExternalID:      extTx.ID,
			Metadata:        txMeta,
		})
		if txErr != nil {
			s.logger.Warn("skipping invalid provider transaction", "err", txErr, "external_id", extTx.ID)
			continue
		}

		if norm.IsInvoiceSettlement && domainAccType == domain.AccountTypeCreditCard {
			state.cardPayments = append(state.cardPayments, domain.CardPaymentObservation{
				ExternalBillID: extTx.BillID,
				Date:           extTx.Date,
				Amount:         extTx.Amount,
			})
		}

		internalInvoiceID, invErr := s.resolveTransactionInvoice(ctx, req.TenantID, state, extTx, norm, syntheticTotals)
		if invErr != nil {
			return invErr
		}
		newTx.BillID = internalInvoiceID

		prepared = append(prepared, preparedProviderTx{domainTx: newTx, externalID: newTx.ExternalID})
		fingerprints = append(fingerprints, newTx.Fingerprint)
	}

	if len(prepared) > 0 {
		existingMap, fpErr := s.txRepo.FindExistingFingerprints(ctx, req.TenantID, fingerprints)
		if fpErr != nil {
			return fmt.Errorf("failed checking existing fingerprints: %w", fpErr)
		}

		for _, item := range prepared {
			if existingMap[item.domainTx.Fingerprint] {
				if recErr := s.reconcileDuplicateTx(ctx, req.TenantID, item, result); recErr != nil {
					return recErr
				}
				continue
			}

			if cErr := s.txRepo.Create(ctx, item.domainTx); cErr != nil {
				if isDuplicateTxError(cErr) {
					if recErr := s.reconcileDuplicateTx(ctx, req.TenantID, item, result); recErr != nil {
						return recErr
					}
					continue
				}
				return fmt.Errorf("failed creating transaction %q: %w", item.externalID, cErr)
			}

			existingMap[item.domainTx.Fingerprint] = true
			result.TransactionsCreated++
		}
	}

	for synID, totalAmt := range syntheticTotals {
		synInv, fErr := s.invRepo.FindByID(ctx, req.TenantID, synID)
		if fErr != nil {
			return fmt.Errorf("failed loading synthetic invoice %s: %w", synID, fErr)
		}
		if synInv == nil || synInv.ExternalID != nil {
			continue
		}
		synInv.TotalAmount = totalAmt.Round(2)
		if err := s.invRepo.Save(ctx, synInv); err != nil {
			return fmt.Errorf("failed updating synthetic invoice %s total: %w", synID, err)
		}
	}

	return nil
}

func (s *OpenFinanceSyncService) reconcileDuplicateTx(
	ctx context.Context,
	tenantID uuid.UUID,
	item preparedProviderTx,
	result *ports.SyncItemResult,
) error {
	result.TransactionsDeduplicated++
	if err := s.txRepo.ReconcileExternalTransaction(ctx, tenantID, item.externalID, item.domainTx.BillID, item.domainTx.Status); err != nil {
		return fmt.Errorf("failed reconciling duplicate transaction %q: %w", item.externalID, err)
	}
	return nil
}

func (s *OpenFinanceSyncService) resolveTransactionInvoice(
	ctx context.Context,
	tenantID uuid.UUID,
	state *accountSyncState,
	extTx ports.ExternalTransaction,
	norm domain.NormalizedProviderTx,
	syntheticTotals map[uuid.UUID]decimal.Decimal,
) (*uuid.UUID, error) {
	if state.account.Type != domain.AccountTypeCreditCard || norm.IsInvoiceSettlement {
		return nil, nil
	}
	if extTx.BillID != "" {
		if invID, ok := state.invoiceIDMap[extTx.BillID]; ok {
			return &invID, nil
		}
	}

	cycle := domain.ResolveInvoiceCycle(extTx.Date, state.closingDay, state.dueDay)
	if existingCycleInv, ok := state.invoiceByMonth[cycle.CycleKey]; ok {
		if existingCycleInv.ExternalID == nil {
			syntheticTotals[existingCycleInv.ID] = syntheticTotals[existingCycleInv.ID].Add(norm.Amount.Neg())
		}
		return &existingCycleInv.ID, nil
	}

	newInv, invErr := domain.NewCreditCardInvoice(tenantID, state.account.ID, cycle.DueDate, decimal.Zero, domain.InvoiceStatusOpen)
	if invErr != nil {
		return nil, fmt.Errorf("failed building synthetic invoice for cycle %s: %w", cycle.CycleKey, invErr)
	}
	closingCopy := cycle.ClosingDate
	newInv.ClosingDate = &closingCopy
	if saveErr := s.invRepo.Save(ctx, newInv); saveErr != nil {
		return nil, fmt.Errorf("failed saving synthetic invoice for cycle %s: %w", cycle.CycleKey, saveErr)
	}

	state.invoiceByMonth[cycle.CycleKey] = newInv
	syntheticTotals[newInv.ID] = syntheticTotals[newInv.ID].Add(norm.Amount.Neg())
	return &newInv.ID, nil
}

func (s *OpenFinanceSyncService) anchorInitialLiquidBalance(
	ctx context.Context,
	tenantID uuid.UUID,
	state *accountSyncState,
	refNow time.Time,
) error {
	currentAcc, err := s.accountRepo.GetByID(ctx, tenantID, state.account.ID)
	if err != nil {
		return fmt.Errorf("failed reading liquid account %s for balance anchor: %w", state.account.ID, err)
	}
	delta := domain.ComputeLiquidAccountAnchorDelta(currentAcc.Balance, state.extAccount.Balance, state.rawTxInputs, refNow)
	if delta.IsZero() {
		return nil
	}
	if err := s.accountRepo.UpdateBalance(ctx, tenantID, state.account.ID, delta); err != nil {
		return fmt.Errorf("failed anchoring liquid account %s balance: %w", state.account.ID, err)
	}
	return nil
}

func (s *OpenFinanceSyncService) reconcileCreditCardSettlements(
	ctx context.Context,
	tenantID uuid.UUID,
	creditCardIDs []uuid.UUID,
	cardPaymentsByAccount map[uuid.UUID][]domain.CardPaymentObservation,
	result *ports.SyncItemResult,
) error {
	if len(creditCardIDs) == 0 {
		return nil
	}

	allAccounts, err := s.accountRepo.List(ctx, tenantID, ports.AccountFilter{})
	if err != nil {
		return fmt.Errorf("failed listing accounts for settlement reconciliation: %w", err)
	}

	var liquidAccounts []*domain.Account
	liquidTxs := make(map[uuid.UUID][]*domain.Transaction)
	for _, acc := range allAccounts {
		if !acc.IsLiquidAccount() {
			continue
		}
		liquidAccounts = append(liquidAccounts, acc)
		txs, _, listErr := s.txRepo.List(ctx, tenantID, ports.TransactionFilter{AccountID: &acc.ID, Limit: 1000})
		if listErr != nil {
			return fmt.Errorf("failed listing transactions for liquid account %s: %w", acc.ID, listErr)
		}
		liquidTxs[acc.ID] = txs
	}

	for _, cardID := range creditCardIDs {
		invoices, err := s.invRepo.ListByAccount(ctx, tenantID, cardID)
		if err != nil {
			return fmt.Errorf("failed listing invoices for credit card %s: %w", cardID, err)
		}

		for _, inv := range invoices {
			if inv.Status == domain.InvoiceStatusPaid {
				continue
			}
			match := domain.MatchInvoiceSettlement(
				inv,
				liquidAccounts,
				liquidTxs,
				cardPaymentsByAccount[cardID],
				domain.DefaultSettlementWindow(nil),
			)
			if !match.Matched {
				continue
			}
			inv.ApplySettlement(match)
			if saveErr := s.invRepo.Save(ctx, inv); saveErr != nil {
				return fmt.Errorf("failed marking synced invoice %s as paid: %w", inv.ID, saveErr)
			}
			result.InvoicesSettled++
		}

		cardAcc, getErr := s.accountRepo.GetByID(ctx, tenantID, cardID)
		if getErr != nil {
			return fmt.Errorf("failed reloading credit card %s: %w", cardID, getErr)
		}
		delta := domain.ComputeCreditCardLiabilityDelta(cardAcc.Balance, invoices)
		if !delta.IsZero() {
			if updErr := s.accountRepo.UpdateBalance(ctx, tenantID, cardID, delta); updErr != nil {
				return fmt.Errorf("failed updating credit card %s liability balance: %w", cardID, updErr)
			}
		}
	}

	return nil
}

func (s *OpenFinanceSyncService) runPostSyncScans(
	ctx context.Context,
	tenantID uuid.UUID,
	result *ports.SyncItemResult,
) error {
	if s.transferReconciler != nil {
		scanRes, err := s.transferReconciler.ScanAndPropose(ctx, tenantID)
		if err != nil {
			return fmt.Errorf("transfer reconciliation scan failed: %w", err)
		}
		if scanRes != nil {
			result.TransferActionsProposed = scanRes.ProposedActionsCount
			result.ProposedActions = append(result.ProposedActions, scanRes.ProposedActions...)
		}
	}

	if s.categoryMatcher != nil {
		catRes, err := s.categoryMatcher.ScanAndPropose(ctx, tenantID)
		if err != nil {
			return fmt.Errorf("category matching scan failed: %w", err)
		}
		if catRes != nil {
			result.CategoryActionsProposed = catRes.ProposedActionsCount
		}
	}

	return nil
}

func indexAccountsByLookupKeys(accounts []*domain.Account) map[string]*domain.Account {
	idx := make(map[string]*domain.Account, len(accounts)*2)
	for _, acc := range accounts {
		if acc.AccountNumberMasked != "" {
			idx[acc.AccountNumberMasked] = acc
		}
		idx[acc.Name] = acc
	}
	return idx
}

func mapExternalAccountType(extType ports.ExternalAccountType) domain.AccountType {
	switch extType {
	case ports.ExternalAccountCreditCard:
		return domain.AccountTypeCreditCard
	case ports.ExternalAccountSavings:
		return domain.AccountTypeSavings
	case ports.ExternalAccountInvestment:
		return domain.AccountTypeInvestment
	default:
		return domain.AccountTypeChecking
	}
}

func isDuplicateTxError(err error) bool {
	if errors.Is(err, domain.ErrDuplicateTransaction) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "uq_tenant_transaction_fingerprint")
}
