# 03: Migrate Producers & HTTP Handler and Contract Raw Map Plumbing (Migrate & Contract)

**What to build:** Migrate all three `Pending Action` producers (`cmd/mcp`, `CategoryMatcher`, `TransferReconciliationService`) and the inbound HTTP transport (`PendingActionHandler`, `RouterConfig`, `cmd/api`, `cmd/sync-pluggy`) onto `ports.PendingActionService` and the typed domain builders, deleting manual `map[string]any` construction, manual `ListPending` deduplication loops, and direct repository dependencies from `PendingActionHandler`.

**Blocked by:** 02 (Deep PendingActionService Application Module)

**Status:** completed

- [x] Migrate `CategoryMatcher` (`internal/app/category_matcher.go`) and `TransferReconciliationService` (`internal/app/transfer_reconciliation_service.go`) to use `PendingActionIndex` / `PendingActionService` and typed domain proposal builders, eliminating manual `map[string]any` assembly and `Payload[...].(string)` type-assertion loops
- [x] Migrate `cmd/mcp` (`cmd/mcp/main.go` and `cmd/mcp/mcp_test.go`) to use `ports.PendingActionService.ProposeCategorization` and `ListPending` so MCP proposals validate target entities and hydrate full UI display fields
- [x] Refactor `PendingActionHandler` (`internal/adapters/inbound/http/handlers/pending_action_handler.go`) to depend solely on `ports.PendingActionService`, deleting `executeApproval` and direct `PendingActionRepository`/`InvoiceRepository`/`TransactionService`/`TransferService` injections from the handler; update `RouterConfig` (`internal/adapters/inbound/http/router.go`), `cmd/api/main.go`, and `cmd/sync-pluggy/main.go`
- [x] Replace shallow repository-mock tests in `internal/adapters/inbound/http/handlers/pending_action_handler_test.go` with tests across the `ports.PendingActionService` seam (including error-propagation cases), and verify `go test`, `go vet ./...`, and `npm --prefix web run build` pass with zero skipped tests
