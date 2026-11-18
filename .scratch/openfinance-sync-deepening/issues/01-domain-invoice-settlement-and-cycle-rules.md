# 01: Domain Invoice Settlement, Normalization, and Cycle Rules

**What to build:** Establish a pure in-process domain core covering Open Finance transaction normalization, Invoice billing-cycle routing, unified Invoice Settlement matching, and Liquid Account / credit card Ledger Balance delta calculation.

**Blocked by:** None (can start immediately)

**Status:** completed

- [x] Pure functions cover provider credit card sign inversion, `is_bill_payment` settlement detection, and Brazil-timezone `PENDING` status classification
- [x] Pure `ResolveInvoiceCycle` and `ResolveProviderInvoiceStatus` handle month-end day clamping and preserve prior `PAID` status
- [x] Unified `MatchInvoiceSettlement`, `ApplySettlement`, and `ClearSettlement` consolidate window tolerances, keyword priority, and card-payment fallback
- [x] Pure `ComputeLiquidAccountAnchorDelta` and `ComputeCreditCardLiabilityDelta` calculate balance adjustments without I/O
- [x] Table-driven domain unit tests verify all edge cases with zero mocks and zero skipped tests
