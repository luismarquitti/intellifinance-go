# 02: Deep PendingActionService Application Module (Expand)

**What to build:** Introduce the `ports.PendingActionService` interface (`internal/ports/pending_action_service.go`) and its deep implementation `app.PendingActionService` (`internal/app/pending_action_service.go`) owning proposal validation, ID-only snapshot hydration, deduplication, fail-closed approval execution (`Category` assignment, `Internal Transfer` confirmation, `Invoice Settlement`, and credit card `Account` liability `Ledger Balance` sync), and single/batch decisions.

**Blocked by:** 01 (Domain Pending Action Typed Proposals, Codecs, and Deduplication Index)

**Status:** completed

- [x] Define `ports.PendingActionService` and `ports.BatchDecisionResult` in `internal/ports/pending_action_service.go`
- [x] Implement `app.PendingActionService` in `internal/app/pending_action_service.go` with `ListPending`, `LoadPendingIndex`, `ProposeCategorization` (including automatic `Transaction`/`Category` lookup and display hydration when snapshot fields are omitted), `ProposeTransferReconciliation`, `Approve`, `Reject`, `ApproveBatch`, and `RejectBatch`
- [x] Ensure `Approve` fails closed on any payload decode, lookup, transfer confirmation, or `Invoice` save error (leaving the `Pending Action` in `PENDING` status), applies `inv.ApplySettlement` + `ComputeCreditCardLiabilityDelta` on `Invoice Settlement` approvals, and rejects unsupported action types with `domain.ErrUnsupportedPendingActionType`
- [x] Write behavioral tests in `internal/app/pending_action_service_test.go` across the `ports.PendingActionService` seam using in-memory repository adapters (covering hydration, deduplication, categorization approval, transfer + invoice settlement approval, failure rollback/preservation of `PENDING` state, and batch decisions)
