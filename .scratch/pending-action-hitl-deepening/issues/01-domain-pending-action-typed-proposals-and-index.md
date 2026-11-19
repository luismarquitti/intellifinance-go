# 01: Domain Pending Action Typed Proposals, Codecs, and Deduplication Index (Expand)

**What to build:** Enrich the pure in-process `Pending Action` domain module (`internal/domain/pending_action.go`) with strongly typed proposal models (`CategorizeTransactionProposal`, `ReconcileTransferProposal`), entity-to-proposal constructors, fail-closed JSON payload decoders, `PendingActionIndex` deduplication, `SelectPendingActions` batch filtering, and `Transaction.ApplyApprovedCategorization`.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Define sentinel errors (`ErrInvalidPendingActionPayload`, `ErrPendingActionNotPending`, `ErrPendingActionNotApproved`, `ErrDuplicatePendingAction`, `ErrUnsupportedPendingActionType`) and `ReconciliationType` constants (`INTERNAL_TRANSFER`, `BILL_PAYMENT`)
- [ ] Implement `CategorizeTransactionProposal` and `ReconcileTransferProposal` with `decimal.Decimal` precision validation (`IsRepresentableInCents`, confidence in `[0, 1]`), entity builders (`NewCategorizeProposalFromEntities`, `NewReconcileTransferProposalFromEntities`), typed constructors (`NewCategorizePendingAction`, `NewReconcileTransferPendingAction`), and decoders (`DecodeCategorizePayload`, `DecodeReconcileTransferPayload`) preserving 100% frontend JSON key compatibility
- [ ] Implement pure in-process `PendingActionIndex` (`NewPendingActionIndex`, `HasCategorizationFor`, `HasTransferLeg`, `HasDeduplicationKey`, `Record`) and `SelectPendingActions` (`PendingActionSelector`)
- [ ] Add table-driven unit tests in `internal/domain/pending_action_test.go` covering validation, round-trip encoding/decoding, deduplication indexing, and batch selection with zero mocks and zero skipped tests
