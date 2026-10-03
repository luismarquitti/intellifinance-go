# ADR-007: Open Finance Integration Strategy & Multi-Format Ingestion Fallback

- **Status**: Accepted
- **Deciders**: Software Architect (Worker M3), Principal Engineer, Integrations Specialist, Security Officer
- **Date**: 2026-09-24
- **Technical Story**: Milestone 3 - Architecture Decision Records

---

## 1. Context & Problem Statement

Automating transaction synchronization with Brazilian banking institutions (such as Itaú, Nubank, Banco do Brasil, Bradesco, Santander, Banco Inter, and C6 Bank) is critical to eliminate manual bookkeeping friction. However, integrating banking connectivity into an open-source, dual-target personal finance platform introduces significant architectural, financial, and operational challenges:

1. **Commercial SaaS Cost Barrier**: Commercial Open Finance aggregators (such as Pluggy, Belvo, or Klavi) charge ongoing monthly fees per active connected bank account in live production mode. Imposing mandatory paid live connections would violate the project's **Zero-Cost Personal Operation** invariant, prevent open-source self-hosting, and block public portfolio visitors from testing live banking flows.
2. **Third-Party Availability & Session Brittleness**: Brazilian Open Finance APIs frequently experience institution maintenance windows, expired bank consents (mandatory 12-month re-consent under Central Bank of Brazil rules), and multi-factor authentication (MFA) prompts. A system relying exclusively on live Open Finance sync becomes unusable during bank API outages.
3. **Legacy Historical Data Formats**: Historical personal finances are stored across diverse offline files:
   - **OFX Statements**: Brazilian banks frequently export Open Financial Exchange files using older SGML 1.02 syntax encoded in ISO-8859-1 (Latin-1) or Windows-1252 rather than UTF-8.
   - **Multi-Dialect CSVs**: Inconsistent column headers, semicolon (`;`) or comma (`,`) delimiters, inverted credit card signs (e.g. Nubank bills where expenses are positive), and Brazilian currency notation (`R$ 1.250,50`).
   - **Excel Spreadsheets**: Multi-sheet family ledgers (`.xlsx`) requiring streaming extraction without high memory consumption.
   - **PDF Extracts & Paystubs (*Holerites*)**: Digital and scanned documents requiring visual table reconstruction.
4. **Duplicate Ingestion Hazards**: Webhook retries, repeated manual file imports, and scheduled sync jobs frequently process overlapping transaction sets. Without deterministic deduplication, ledger balances, category totals, and budget analytics become corrupted.

---

## 2. Decision Drivers

- **Zero-Cost Public Showcase & Development**: Public portfolio viewers, automated CI/CD pipelines, and local developers must be able to experience full bank synchronization at **$0.00 cost** without requiring paid banking contracts.
- **Configurable Live Bank Connectivity**: Personal instances must support seamless configuration of live Pluggy API credentials for automated daily bank syncing.
- **Comprehensive Offline Fallback**: Universal support for manual file uploads (OFX, CSV, Excel, PDF) operating completely offline without cloud or Open Finance dependencies.
- **Deterministic Idempotency & Deduplication**: Universal SHA-256 transaction fingerprinting guaranteeing that duplicate imports are safely ignored at the database layer.
- **LGPD-Compliant Credential Handling**: Zero raw banking passwords stored on the server; delegating OAuth/MFA flows entirely to certified Open Finance infrastructure.

---

## 3. Considered Options

1. **Unofficial Web Scraping / Puppeteer Headless Bots**: Reverse engineering bank web portals (violates bank Terms of Service, extremely brittle, breaks on UI changes, massive security liability).
2. **Live Open Finance Aggregator Exclusively**: Relies 100% on paid third-party APIs; incurs recurring costs and fails during bank outages.
3. **Manual File Import Only**: Zero ongoing cost, but high manual bookkeeping friction; lacks modern automated Open Finance capabilities.
4. **Dual-Mode Pluggy Open Finance (Free Sandbox + Configurable Live) paired with Zero-Cost Multi-Format Fallback & SHA-256 Deduplication (Selected)**.

---

## 4. Evaluation & Comparative Matrix

| Evaluation Criteria | Dual-Mode Pluggy + Multi-Format Fallback (Selected) | Live Aggregator Only | Manual File Import Only | Unofficial Puppeteer Bots |
|---|---|---|---|---|
| **Operational Dollar Cost** | **$0.00** (Free Sandbox for demo/dev; optional live) | High ($5 - $15/mo ongoing per user) | **$0.00** | $0.00 (Self-hosted) |
| **Public Portfolio Showcase** | **Flawless**: Visitors authenticate with mock bank credentials | Broken: Requires paying per visitor connection | Static demo data only | Unusable |
| **Bank Outage Resilience** | **Complete**: Instantly upload OFX/CSV/PDF if API is down | Zero (System blocked) | Complete | Zero |
| **Historical Data Coverage** | **100%**: Ingests 10+ years of past CSV/OFX/XLSX | Limited (Banks limit API sync to 90–365 days)| 100% | Limited |
| **Maintenance Burden** | Moderate (Unified Hexagonal Port abstraction) | Low (Single vendor API) | Moderate (Parser maintenance)| Extreme (Constant bot maintenance) |
| **Bank Security / Compliance** | **Certified Central Bank Open Finance** | Certified | No bank credentials involved | Severe Security Risk / ToS violation |

---

## 5. Decision Outcome

**Adopt a Dual-Mode Open Finance Strategy using the Pluggy API (Free Sandbox by default + Configurable Live Production)**, tightly integrated with a **Zero-Cost Multi-Format Offline Ingestion Engine (OFX, CSV, Excel XLSX, and Multimodal PDF via Vertex AI / LiteLLM)**, governed by **Deterministic SHA-256 Fingerprint Deduplication**.

### Architectural Flow:

```
+─────────────────────────────────────────────────────────────────────────────+
|                        INGESTION SOURCES                                    |
|                                                                             |
|   [ Pluggy Open Finance ]                      [ Offline File Uploads ]     |
|   ├── Free Sandbox (Default / Portfolio / CI)  ├── OFX 1.02 / 2.x (Latin-1) |
|   └── Live Production (Configured Personal)    ├── Multi-Dialect CSV        |
|                                                ├── Excel XLSX (excelize)    |
|                                                └── Multimodal PDF (AI OCR)  |
+──────────────────────────────────────┬──────────────────────────────────────+
                                       │
                                       ▼
+─────────────────────────────────────────────────────────────────────────────+
|                      UNIFIED INGESTION PORT INTERFACE                       |
|                                                                             |
|   ports.IngestionConnector & ports.FileParser                               |
|   - Normalizes raw streams into []domain.RawTransaction                     |
|   - Normalizes BRL amounts (Negative = Expense, Positive = Income)          |
|   - Normalizes dates to UTC ISO 8601                                        |
+──────────────────────────────────────┬──────────────────────────────────────+
                                       │
                                       ▼
+─────────────────────────────────────────────────────────────────────────────+
|                   DETERMINISTIC DEDUPLICATION ENGINE                        |
|                                                                             |
|   Computes SHA-256 Fingerprint Hash:                                        |
|   SHA256(tenant_id || account_id || date || amount || desc || external_id) |
+──────────────────────────────────────┬──────────────────────────────────────+
                                       │
                                       ▼
+─────────────────────────────────────────────────────────────────────────────+
|                          PERSISTENCE & STORAGE                              |
|                                                                             |
|   PostgreSQL with Row-Level Security:                                       |
|   INSERT INTO transactions (...) VALUES (...)                               |
|   ON CONFLICT (tenant_id, fingerprint) DO NOTHING;                          |
+─────────────────────────────────────────────────────────────────────────────+
```

---

## 6. Pluggy API Architecture & Token Lifecycle

### 6.1 Free Sandbox Mode for Portfolio Showcase & Testing

By default, the platform runs with `PLUGGY_ENVIRONMENT=sandbox`. In this mode:
- All Brazilian financial institutions are simulated with realistic transaction streams.
- Reviewers and portfolio visitors can test the live Connect Widget using standardized credentials:
  - **Username / CPF**: `000.000.000-00`
  - **Password**: `password-ok`
  - **MFA Code**: `123456`
- Failure states (`password-invalid`, `password-mfa-invalid`, `password-outdated`) allow testing error boundaries without real bank risk.
- Incurred cost: **$0.00 USD / R$ 0,00**.

### 6.2 Token Security & Webhook Verification

1. **Server-to-Server API Key**:
   - Backend calls `POST https://api.pluggy.ai/auth` using `PLUGGY_CLIENT_ID` and `PLUGGY_CLIENT_SECRET`.
   - Returns an `apiKey` cached in memory with a TTL of 1 hour 50 minutes (Pluggy TTL is 2 hours).
   - Never transmitted to frontend clients.
2. **Client Connect Token**:
   - Backend exposes authenticated endpoint `POST /api/v1/openfinance/connect-token`.
   - Issues a scoped token (`clientUserId: tenant_id`, TTL 30 minutes) allowing the frontend widget to authenticate directly with the bank.
3. **Webhook Security**:
   - Pluggy dispatches webhooks (`item/updated`, `transactions/deleted`) to `/api/v1/webhooks/pluggy`.
   - The handler verifies a pre-shared secret header (`X-IntelliFinance-Webhook-Secret`) using constant-time comparison (`subtle.ConstantTimeCompare`) before enqueuing an ingestion job:

```go
func (h *WebhookHandler) HandlePluggyWebhook(w http.ResponseWriter, r *http.Request) {
	secretHeader := r.Header.Get("X-IntelliFinance-Webhook-Secret")
	if subtle.ConstantTimeCompare([]byte(secretHeader), []byte(h.expectedSecret)) != 1 {
		http.Error(w, "Unauthorized webhook", http.StatusUnauthorized)
		return
	}
	// Enqueue job to TaskQueue and acknowledge immediately
	w.WriteHeader(http.StatusAccepted)
}
```

---

## 7. Zero-Cost Multi-Format Fallback Parsers

### 7.1 OFX Parser (SGML 1.02 & XML 2.x)
- Uses `golang.org/x/text/encoding/charmap` to automatically decode ISO-8859-1 (Latin-1) and Windows-1252 into UTF-8.
- Extracts `<FITID>`, `<DTPOSTED>`, `<TRNAMT>`, and `<MEMO>`.
- Preserves unique `<FITID>` identifiers as primary external IDs.

### 7.2 Multi-Dialect CSV Parser
- Auto-detects delimiters (`,`, `;`, `\t`) by frequency scanning the first 5 lines.
- Normalizes Brazilian currency notation: converts `"1.250,50"` into `1250.50`.
- Inverts credit card signs where bank exports represent expenses as positive numbers (e.g. Nubank bill exports).

### 7.3 Excel (XLSX) Streaming Parser
- Implemented with `github.com/qax-os/excelize/v2`.
- Uses streaming row reader (`file.Rows(sheetName)`) to parse historical multi-year sheets with minimal memory allocations.

### 7.4 Multimodal PDF Statement OCR
- Direct transmission of PDF binary to Vertex AI Gemini 2.0 Flash (with local LiteLLM/Ollama fallback for development).
- Reconstructs multi-column tables visually, extracting dates, merchants, and signed values into structured JSON.

---

## 8. Deterministic Deduplication Fingerprint Formula

To guarantee absolute idempotency across webhooks, scheduled synchronizations, and accidental re-uploads, every transaction is assigned a 64-character hexadecimal SHA-256 fingerprint:

$$\text{Fingerprint} = \text{SHA256}(\text{tenant\_id} \parallel \text{account\_id} \parallel \text{date} \parallel \text{amount} \parallel \text{clean\_description} \parallel \text{external\_id})$$

### Normalization Rules for `clean_description`:
1. Convert to uppercase.
2. Strip non-alphanumeric noise, extra whitespace, and bank prefixes (e.g. `"COMPRA CARTAO 12/03 "`, `"PIX TRANSF "`).
3. If no external ID exists (e.g. in manual CSV imports), `external_id` defaults to an empty string, relying on the combination of account, date, amount, and normalized description.

### Database Insertion Invariant:
```sql
INSERT INTO transactions (
    id, tenant_id, account_id, category_id, date, amount,
    description, type, status, fingerprint, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW()
)
ON CONFLICT (tenant_id, fingerprint) DO NOTHING;
```

---

## 9. Consequences

### Positive Consequences
- **Zero Ongoing Cost**: Portfolio demonstration and local development incur $0.00 in third-party API fees using Pluggy Sandbox.
- **Universal Ingestion Coverage**: Users can ingest historical data from any bank via OFX, CSV, Excel, or PDF even when live Open Finance is disconnected.
- **Bulletproof Deduplication**: Re-running imports or re-syncing bank accounts produces zero duplicate transaction records.
- **High Security**: Bank credentials never touch IntelliFinance servers.

### Negative Consequences
- **Parser Maintenance**: Supporting varied CSV dialects and encoding formats requires robust unit test coverage.
- **Pluggy Sandbox Item Expiration**: Pluggy purges inactive Sandbox connections after 30 days, requiring automated test suites to create fresh test items.

### Neutral Consequences
- All ingested transactions are normalized to negative decimal values for expenses and positive decimal values for income.

---

## 10. Implementation & Verification Plan

1. **Verify Pluggy Sandbox Connectivity**:
   - Execute automated integration test exchanging sandbox credentials for an API key:
     ```bash
     go test -v ./internal/adapters/outbound/openfinance/ -run TestPluggySandboxAuth
     ```
2. **Verify Multi-Format Parsing**:
   - Run unit test suite against synthetic fixtures in `test/fixtures/` (`itau.ofx`, `nubank.csv`, `budget.xlsx`). Assert all rows extract with exact BRL amounts and valid dates.
3. **Verify Deduplication Idempotency**:
   - Ingest a synthetic CSV file containing 50 transactions.
   - Re-ingest the exact same CSV file into the same account.
   - Assert `inserted_count = 0` and `duplicate_count = 50` in the job result summary.
