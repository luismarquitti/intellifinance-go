# Spec: Deepen Pending Action HITL Governance

**Status:** completed

## Problem Statement

Human-in-the-Loop (`Pending Action`) governance is currently fragmented across three producers and one inbound HTTP consumer that manipulate untyped `map[string]any` payloads and talk directly to persistence repositories:
1. **Scattered Untyped Payload Construction & Deduplication**: The MCP server tool, the automated Category matcher, and the Internal Transfer reconciliation scanner each hand-assemble raw `map[string]any` payloads with divergent key sets, perform manual `float64`-to-decimal conversions, and duplicate `ListPending` loops with fragile string type assertions (`Payload["transaction_id"].(string)`) to prevent duplicate proposals. Because the MCP server only receives `transaction_id`, `category_id`, and `reason`, its persisted proposals lack `description`, `clean_description`, `amount`, `date`, and `category_name`, starving the frontend approval modal of display context.
2. **Hexagonal Seam Violation & Silent Error Swallowing on Approval**: The inbound HTTP `Pending Action` handler directly injects `PendingActionRepository` and `InvoiceRepository` alongside `TransactionService` and `TransferService`, embedding cross-aggregate approval execution and batch filtering inside the transport layer. Worse, its approval execution silently discards critical lookup and persistence errors (`_ = invoiceRepo.Save(...)`, or silently skipping missing/invalid transactions while still marking the `Pending Action` as `APPROVED` and `EXECUTED`) and fails to synchronize the credit card `Account` liability `Ledger Balance` when an `Invoice Settlement` proposal is approved.

## Solution

Deepen `Pending Action` governance across two clean seams:
1. **Pure In-Process Domain Core (`Category 1: In-process`)**: Enrich the `Pending Action` domain module with strongly typed proposal value objects (`CategorizeTransactionProposal` and `ReconcileTransferProposal`), entity-to-proposal constructors, bidirectional JSON payload/metadata codecs that preserve 100% frontend wire compatibility, an $O(1)$ in-memory deduplication index (`PendingActionIndex`), pure batch selection filtering (`SelectPendingActions`), and `Transaction.ApplyApprovedCategorization`.
2. **Deep Application Seam (`PendingActionService` — `Category 2/3: Local-substitutable / Ports & Adapters`)**: Introduce a single inbound application port (`PendingActionService`) implemented by a deep application module that owns proposal hydration (enriching ID-only MCP categorization proposals from `Transaction` and `Category` repositories), deterministic deduplication, single and batch human decision workflows (`Approve`, `Reject`, `ApproveBatch`, `RejectBatch`), fail-closed cross-aggregate execution (`Category` assignment, `Internal Transfer` confirmation, `Invoice Settlement` via `ApplySettlement`, and credit card `Account` liability `Ledger Balance` synchronization via `ComputeCreditCardLiabilityDelta`), and explicit error propagation. Migrate all three producers and the HTTP handler/router onto `PendingActionService`.

## User Stories

1. As a tenant operator reviewing AI suggestions in the approval modal, I want categorization proposals created by the MCP server to display the transaction description, clean description, date, formatted amount, and category name identically to scanner-generated proposals, so that I never have to approve blind UUIDs.
2. As a tenant operator, I want automated Category and Internal Transfer scanners to skip transactions that already have an active `PENDING` proposal in $O(1)$ time without manual map parsing, so that repeated scans are idempotent and free of duplicate proposals.
3. As a tenant operator, I want approving a categorization `Pending Action` to verify that the target `Transaction` exists, assign the proposed `Category`, mark the `Transaction` as AI-categorized and manually verified, and fail closed without marking the `Pending Action` as `EXECUTED` if the `Transaction` is missing or persistence fails.
4. As a tenant operator, I want approving an `Internal Transfer` reconciliation `Pending Action` to link the `Outflow Leg` and `Inflow Leg` atomically, and—when the proposal represents an `Invoice Settlement` (`invoice_id` present)—apply settlement metadata to the `Invoice`, transition it to `PAID`, and synchronize the credit card `Account` liability `Ledger Balance`, aborting the approval if any step fails.
5. As a tenant operator, I want batch approval and batch rejection (`All`, filtered by `ActionType`, or explicit `ActionIDs`) to execute deterministically through the application seam and report accurate `ProcessedCount`, `ApprovedCount`, `RejectedCount`, `FailedCount`, and per-action error messages.
6. As a maintainer, I want the inbound HTTP `Pending Action` handler to depend exclusively on `PendingActionService` with zero direct `*Repository` imports, and I want all proposal/decision invariants tested through the public `PendingActionService` and domain interfaces.

## Implementation Decisions

- **Pure In-Process Domain Module (`internal/domain`)**:
  - Define `CategorizeTransactionProposal` and `ReconcileTransferProposal` using `decimal.Decimal` for both monetary amounts and confidence scores (`[0.00, 1.00]`), eliminating `float64` from the proposal boundary.
  - Provide entity-based domain builders (`NewCategorizeProposalFromEntities` and `NewReconcileTransferProposalFromEntities`) that automatically derive fallback `Account` names (`"Conta de Origem"`, `"Conta de Destino"`), `INTERNAL_TRANSFER` vs `BILL_PAYMENT` reconciliation kind, Brazil timezone calendar dates (`StandardizeDateToBrazil`), and canonical Portuguese descriptions.
  - Provide typed constructors (`NewCategorizePendingAction`, `NewReconcileTransferPendingAction`) and fail-closed decoders (`DecodeCategorizePayload`, `DecodeReconcileTransferPayload`) that serialize and validate the exact JSON map keys consumed by the React frontend (`PendingActionsModal.tsx`) while stamping `Metadata["deduplication_key"]` (`categorize_<tx_id>` and `reconcile_<outflow_id>_<inflow_id>`).
  - Implement `PendingActionIndex` (`NewPendingActionIndex`, `HasCategorizationFor`, `HasTransferLeg`, `HasDeduplicationKey`, `Record`) and `SelectPendingActions` (`PendingActionSelector`) as pure in-process functions.
- **Hexagonal Application Seam (`ports.PendingActionService` / `app.PendingActionService`)**:
  - Expose 8 cohesive entry points on `ports.PendingActionService`: `ListPending`, `LoadPendingIndex`, `ProposeCategorization`, `ProposeTransferReconciliation`, `Approve`, `Reject`, `ApproveBatch`, and `RejectBatch`.
  - In `ProposeCategorization`: if display snapshot fields (`Description`, `CategoryName`, etc.) are omitted by an ID-only caller (such as `cmd/mcp`), load the `Transaction` and `Category` within the `Tenant`, reject missing or `VOID`/`TRANSFER` targets, and hydrate the proposal before persisting. Enforce deduplication against existing `PENDING` actions for the same `Transaction` or `Internal Transfer` legs.
  - In `Approve`: enforce **Execute-Before-MarkExecuted** ordering. Decode the typed payload first (returning `domain.ErrInvalidPendingActionPayload` on corrupt data); execute the target ledger mutation (`Transaction` category update or `TransferService.ConfirmTransfer` + `Invoice.ApplySettlement` + `InvoiceRepository.Save` + credit card `Account` liability `Ledger Balance` delta sync via `ComputeCreditCardLiabilityDelta`); only transition `PENDING -> APPROVED -> EXECUTED` after all ledger writes succeed.
  - Map unsupported or parked action types (`SPLIT_TRANSACTION`, `BALANCE_ADJUSTMENT`) to `domain.ErrUnsupportedPendingActionType` rather than silently marking them `EXECUTED`.
- **Caller Migration & Contraction (*Replace, Don't Layer*)**:
  - Migrate `cmd/mcp`, `CategoryMatcher`, and `TransferReconciliationService` to use `PendingActionService` (or typed domain proposals) and delete all manual `map[string]any` construction and manual `ListPending` type-assertion deduplication loops.
  - Refactor `PendingActionHandler` and `RouterConfig` so `PendingActionHandler` accepts only `ports.PendingActionService`. Update `cmd/api` and `cmd/sync-pluggy` wiring accordingly.

## Testing Decisions

- **Pure Domain Unit Tests (`Category 1: In-process` Seam)**: Table-driven tests covering `CategorizeTransactionProposal` and `ReconcileTransferProposal` validation (sub-cent rejection, confidence bounds, distinct leg IDs), entity-to-proposal builders, JSON payload round-tripping (`New*PendingAction` -> `Decode*Payload`), `PendingActionIndex` leg/transaction collision detection, and `SelectPendingActions` batch filtering with zero mocks.
- **Behavioral Application Seam Tests (`ports.PendingActionService`)**: Test `app.PendingActionService` through its public interface backed by in-memory repository adapters, covering:
  - ID-only categorization proposal hydration and duplicate suppression.
  - Transfer reconciliation proposal creation and leg-collision suppression.
  - `Approve` for `CATEGORIZE_TRANSACTION` (mutates `Transaction.CategoryID`, `IsAICategorized`, `IsManuallyVerified`; fails closed if `Transaction` is missing).
  - `Approve` for `RECONCILE_TRANSFER` with and without `InvoiceID` (confirms `Internal Transfer`, applies `Invoice Settlement` marking `Invoice` as `PAID`, updates credit card `Account` liability `Ledger Balance`, and leaves `Pending Action` in `PENDING` if transfer confirmation or invoice save fails).
  - `Reject`, `ApproveBatch` (with partial failure isolation), and `RejectBatch`.
- **HTTP Transport Adapter Tests (`PendingActionHandler`)**: Replace the old 4-dependency handler test setup (which tested `Approve` with a nil payload and nil `TransactionService`, masking silent no-op bugs) with clean HTTP status/RFC 7807 tests across the single `ports.PendingActionService` seam.

## Out of Scope

- Implementing execution logic for parked action types (`SPLIT_TRANSACTION`, `BALANCE_ADJUSTMENT`) documented in `.out-of-scope/README.md`.
- Changing HTTP endpoint paths, request/response DTO JSON keys, or `PendingActionsModal.tsx` frontend code.
- Adding a dynamic runtime plugin/executor registry (`RegisterExecutor`).

## Further Notes

- Enforces `ADR-010` (Confirmed Internal Transfers & Decimal Precision) and `ADR-012` (HITL Pending Action Governance & Provider Balance Snapshot Segregation).
- Follows Workflow 3 Stage 6 (`Expand → Migrate → Contract` and *Replace, don't layer*).
