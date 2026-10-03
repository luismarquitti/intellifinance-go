# Product Requirements Document (PRD)
# Project: IntelliFinance — High-Performance Financial Management Platform

**Document Version**: 1.0.0-scaffold (Greenfield Specification)  
**Author**: Product Architecture Team (Worker M2)  
**Status**: Authoritative Product Requirements Document  
**Date**: 2026-09-24  
**Target Runtime**: Go 1.22+ Standard Project Layout  
**Target Infrastructure**: Dual-Target — Homelab Bare-Metal (`lm-claw` Debian 13) & GCP Serverless Production (Cloud Run, Cloud Tasks, Vertex AI)  
**Classification**: Public Portfolio Showcase & Personal/Household Production  

---

## 1. Executive Summary & Product Vision

### 1.1 Product Vision & North Star
IntelliFinance is an enterprise-grade, high-performance financial management platform designed from the ground up in **Golang 1.22+** following **Hexagonal Architecture (Ports and Adapters)**. The system transforms personal and household financial intelligence by unifying automated Brazilian Open Finance banking synchronization with a resilient, zero-cost offline ingestion pipeline, machine-learning-driven transaction categorization, and rigorous household co-budgeting.

IntelliFinance eliminates manual bookkeeping toil through deterministic transaction deduplication, real-time banking webhooks, and intelligent visual document extraction powered by Google Vertex AI Gemini 2.0 Flash. At the same time, it strictly protects personal privacy through PostgreSQL Row-Level Security (RLS) and full compliance with the Brazilian General Data Protection Law (LGPD).

### 1.2 Strategic Goals & Invariants
The platform is governed by three uncompromised strategic pillars:

1. **Zero-Cost Personal & Household Operation**:
   Operates continuously with $0.00 incremental cloud billing for personal and family budgeting. The production infrastructure harnesses Google Cloud Platform's perpetual free tiers (Cloud Run with scale-to-zero, Cloud Tasks push dispatches, Artifact Registry) and Google Cloud GenAI App Builder startup credits. When cloud credits are depleted or when running offline in the Homelab, the system automatically falls back to local containerized infrastructure (PostgreSQL 15+, Asynq/Redis queueing, LiteLLM proxy, and local Ollama models on node `lm-claw`).

2. **Public Engineering Portfolio Showcase Excellence**:
   Engineered as an open-source-grade software showcase on GitHub that demonstrates modern Go design patterns, spec-first OpenAPI 3.0 REST contracts, compile-time type-safe persistence (`sqlc` + `pgx/v5`), asynchronous task processing, automated Gitleaks CI/CD security hygiene, and clean greenfield git history with zero historical data leaks. Recruiters, engineering peers, and tech leads can launch interactive Pluggy Sandbox bank syncs and test automated AI ingestion directly with synthetic datasets.

3. **Multi-Tenancy with Ironclad Privacy & Household Co-Budgeting**:
   Provides shared co-budgeting workflows for Luis and Eluma (joint account visibility, discretionary private purchase shielding, 50/50 and proportional expense splitting, and medical clinic business record isolation), while enforcing cryptographically and relationally isolated tenant boundaries for invited friends and family via database-enforced PostgreSQL Row-Level Security.

### 1.3 Dual-Target Operating Paradigm
IntelliFinance is architected with a decoupled dual-target runtime topology:

| Dimension | Homelab Development Target (`lm-claw`) | GCP Serverless Production Target |
|---|---|---|
| **Host Environment** | Physical Bare-Metal Dell OptiPlex 3070 (`192.168.3.10`, Debian 13 Trixie) | Google Cloud Run (Containerized, Fully Managed) |
| **API Server Entrypoint** | `cmd/api` binary (HTTP REST via Chi v5) | `cmd/api` binary deployed as Cloud Run service |
| **Worker Processing** | `cmd/worker` continuous daemon via `hibiken/asynq` + Redis 7 | Serverless Cloud Tasks HTTP push webhooks to `/internal/tasks/*` |
| **Database Persistence** | PostgreSQL 16 on `lm-claw:5432` | Managed PostgreSQL (Cloud SQL or Serverless Supabase) |
| **Queue Mechanism** | Redis 7 streaming queues (`lm-claw:6379`) | Google Cloud Tasks (1M free dispatches/month) |
| **AI Categorization** | LiteLLM proxy (`:4000`) & Ollama (`:11434`, `llama3.2` / `qwen2.5`) | Vertex AI Gemini 2.0 Flash (`google.golang.org/genai`) |
| **Secret Management** | Local `.env` file (0600 permissions, strictly gitignored) | Google Cloud Secret Manager via Service Account IAM |
| **Cost Profile** | $0.00 (Self-hosted on existing home hardware) | $0.00 idle cost (Perpetual free tier + GenAI credits) |

---

## 2. Target Personas & Multi-Tenancy Use Cases

### 2.1 Persona 1: Luis Marquitti — Household Financial Lead & Software Architect
- **Role**: Household Owner, Primary System Administrator, and Lead Developer.
- **Background**: Senior Software Engineer / Solutions Architect managing family finances, investments, credit cards, and technology infrastructure.
- **Pain Points**:
  - Fragmented banking apps (Itaú, Nubank, Inter) requiring manual daily logins and disjointed exports.
  - Repetitive, error-prone manual transaction categorization.
  - Risk of leaking confidential personal assets (bank statements, tax declarations, paystubs) when publishing open-source projects on GitHub.
  - Difficulty maintaining a clear split between joint household expenses and personal discretionary spending.
- **Core Objectives**:
  - Automatically sync checking and credit card transactions via Open Finance.
  - Maintain a unified household ledger with automated AI categorization (>90% accuracy).
  - Operate the entire platform at zero personal financial cost using Homelab and GCP serverless free tiers.
  - Showcase an elite, production-grade Go codebase on public GitHub.

### 2.2 Persona 2: Dra. Eluma Marquitti — Co-Budgeting Partner & Medical Clinic Owner
- **Role**: Household Admin, Co-Budgeting Partner, and Healthcare Entrepreneur.
- **Background**: Practicing Physician managing personal finances, joint household budgets, and professional revenue/expenses for her medical clinic (`Clínica Eluma`) and business ventures (`Motoel`).
- **Pain Points**:
  - Accidental commingling of medical clinic expenses with personal groceries and household utility bills.
  - Lack of transparency into shared family expenses, requiring cumbersome end-of-month manual spreadsheet reconciliations.
  - Complex multi-format bank statements (credit card invoices, scanned payment receipts, insurance reimbursements).
- **Core Objectives**:
  - Review joint household burn rate and monthly category budgets with zero manual bookkeeping.
  - Automatically split shared expenses (50/50 or custom proportional splits) with Luis with single-click reconciliation.
  - Maintain strict segregation of `Clínica Eluma` commercial cash flows from household personal budgets under a separate business account category.

### 2.3 Persona 3: Invited Friends & Extended Family (e.g., Edson) — Isolated Tenant
- **Role**: Invited Tenant / External User.
- **Background**: Friend or family member invited to use IntelliFinance for personal money management.
- **Pain Points**:
  - Distrust of commercial budgeting apps that monetize, sell, or profile private banking data.
  - Needs reliable bank sync and statement ingestion without subscription paywalls.
- **Core Objectives**:
  - Manage personal accounts, categories, and budgets in an independent, sovereign workspace.
  - Guarantee absolute privacy: zero visibility into Luis & Eluma's household ledger, and zero possibility of Luis or any other user viewing their private transactions.
  - Retain full data sovereignty under LGPD: export all data on demand and trigger cryptographic erasure upon account deletion.

### 2.4 Multi-Tenancy & Co-Budgeting Architecture Matrix

```
+---------------------------------------------------------------------------------------------------+
|                                  INTELLIFINANCE MULTI-TENANCY TOPOLOGY                            |
+---------------------------------------------------------------------------------------------------+
|                                                                                                   |
|  [ TENANT 1: "Família Marquitti" (Household) ]            [ TENANT 2: "Edson Silva" (Personal) ]  |
|                                                                                                   |
|  Members:                                                 Members:                                |
|  ├── Luis Marquitti (Role: OWNER)                         └── Edson Silva (Role: OWNER)           |
|  └── Dra. Eluma Marquitti (Role: ADMIN)                                                           |
|                                                           Accounts:                               |
|  Accounts & Visibility Boundaries:                        ├── Nubank Conta (PERSONAL - Private)   |
|  ├── Itaú Conta Conjunta     [Visibility: SHARED]         └── XP Investimentos (PERSONAL - Private)|
|  │   └── Shared Household: Groceries, Utilities, Rent                                             |
|  │                                                        Transactions:                           |
|  ├── Nubank Luis             [Visibility: PERSONAL]       └── Strictly isolated via PostgreSQL    |
|  │   └── Private Discretionary Spending                       Row-Level Security (RLS).           |
|  │                                                            Zero cross-tenant leakage.          |
|  ├── Nubank Eluma            [Visibility: PERSONAL]                                               |
|  │   └── Private Discretionary Spending                                                           |
|  │                                                                                                |
|  └── Clínica Eluma (PJ)      [Visibility: RESTRICTED]                                             |
|      └── Medical Clinic Operations: Eluma (Admin),                                                |
|          Luis (Read-Only Auditor / System Admin)                                                  |
|                                                                                                   |
|  Expense Splitting & Tagging Engine:                                                              |
|  ├── Tag #casa: 100% Joint Household Allocation                                                   |
|  ├── Tag #split-50-50: Automatically debts R$ 250 to Luis / R$ 250 to Eluma                       |
|  └── Tag #clinica: Flagged as professional expense; excluded from family burn rate                |
+---------------------------------------------------------------------------------------------------+
```

---

## 3. Product Architecture & Operating Model

### 3.1 Hexagonal Decoupling & Invariants
IntelliFinance enforces strict Hexagonal Architecture (Ports and Adapters). The business logic resides exclusively within pure Go packages (`internal/domain` and `internal/app`), containing zero imports of external web frameworks (Chi), database drivers (`pgx`), message brokers (Asynq/Redis), or cloud provider SDKs (Google Cloud / Vertex AI).

All infrastructure dependencies are accessed across interfaces defined in `internal/ports`:
- **Inbound (Driving) Ports**: `TransactionService`, `AccountService`, `IngestionService`, `CategorizationService`, `AnalyticsService`.
- **Outbound (Driven) Ports**: `Repository` (Persistence), `TaskQueue` (Messaging), `CategorizationEngine` (AI), `DocumentParser` (File extraction), `OpenFinanceConnector` (Banking sync), `SecretManager` (Key resolution).

### 3.2 Dual-Driver Asynchronous Processing Model
To reconcile bare-metal Homelab continuous queueing with Google Cloud Run scale-to-zero economics, the platform implements an abstract `ports.TaskQueue`:

```
                           [ Application Use Case: IngestFile ]
                                            |
                                            v
                                   [ ports.TaskQueue ]
                                            |
                     +----------------------+----------------------+
                     |                                             |
          (ENVIRONMENT = development)                   (ENVIRONMENT = production)
                     |                                             |
                     v                                             v
           [ AsynqQueueAdapter ]                        [ CloudTasksQueueAdapter ]
                     |                                             |
              (Redis LPUSH/ZADD)                              (Cloud Tasks API)
                     |                                             |
                     v                                             v
             [ Redis 7 Server ]                          [ GCP Cloud Tasks ]
             (lm-claw:6379)                                        |
                     |                                      (OIDC HTTP Webhook)
                     v                                             v
         [ cmd/worker (Daemon) ]                        [ cmd/api (Cloud Run) ]
      (Asynq Worker Server on lm-claw)                  (POST /internal/tasks/*)
                     |                                             |
                     +----------------------+----------------------+
                                            |
                                            v
                                 [ Task Handler Dispatcher ]
                                            |
                                 - Parse Statements (OFX/CSV/PDF)
                                 - Calculate SHA-256 Fingerprints
                                 - Vertex AI Batch Categorization
                                 - Atomic PostgreSQL Commit
```

### 3.3 AI Categorization & Document Extraction Strategy
IntelliFinance deploys Google's **Gemini 2.0 Flash** model via the unified `google.golang.org/genai` Go SDK, utilizing Vertex AI Enterprise endpoints to consume GenAI App Builder credits. Gemini 2.0 Flash is selected for its sub-second reasoning latency, multimodal native PDF understanding, and strict JSON Schema output enforcement.

When executing in local Homelab mode or when cloud network connectivity is disabled, the system dynamically switches to the `LiteLLMAdapter`, directing HTTP requests to `http://192.168.3.10:4000/v1/chat/completions` where LiteLLM distributes queries across local Ollama instances (`llama3.2-vision` for document OCR, `qwen2.5-coder` for categorization) at zero financial cost.

### 3.4 Brazilian Open Finance Strategy
The platform adopts **Pluggy API** as its Open Finance provider. Pluggy natively covers all core financial institutions operating in Brazil: Itaú Unibanco, Nubank, Banco do Brasil, Bradesco, Santander, Banco Inter, C6 Bank, BTG Pactual, XP Investimentos, and Mercado Pago.
- **Sandbox Mode**: Default for local development, CI automated testing, and the public GitHub portfolio showcase. Uses perpetual free mock credentials (`user-ok`, MFA `123456`) at $0.00 cost.
- **Production Mode**: Enabled via configuration for live personal bank account synchronization.
- **Universal Fallback**: A resilient, zero-cost manual file ingestion pipeline supporting Brazilian bank OFX exports, multi-dialect CSV files, Excel spreadsheets, and multimodal PDF statements/paystubs.

---

## 4. Comprehensive Functional Requirements & User Stories

### 4.1 Module 1: Account & Asset Management

#### Functional Requirements
- **FR-1.1**: The system shall support five distinct account types: `CHECKING` (Conta Corrente), `CREDIT_CARD` (Cartão de Crédito), `INVESTMENT` (Investimentos / Renda Fixa / Ações), `CASH` (Dinheiro / Carteira), and `BUSINESS` (Conta PJ / Clínica).
- **FR-1.2**: Every account must be associated with an owning user (`owner_user_id`) and a tenant (`tenant_id`), with an explicit visibility attribute:
  - `HOUSEHOLD_SHARED`: Visible and editable by all authorized household members.
  - `PERSONAL_PRIVATE`: Visible only to the account owner; concealed from other household members.
  - `RESTRICTED_BUSINESS`: Reserved for business operations (e.g., `Clínica Eluma`); accessible with granular permissions (Admin vs Viewer).
- **FR-1.3**: The system shall maintain real-time computed balances and reconciled balances per account, calculated using high-precision fixed-point math (`shopspring/decimal.Decimal` in Go, `NUMERIC(15, 2)` in PostgreSQL).
- **FR-1.4**: The system shall support manual balance adjustment entries with mandatory audit notes to account for unrecorded cash expenditures or historical discrepancies.

#### User Stories

##### US-ACC-01: Create Shared Household Checking Account
- **Given** Luis is logged into IntelliFinance as a household `OWNER` under the "Família Marquitti" tenant,
- **When** Luis submits a request to create a new account with name `"Itaú Conjunta"`, type `CHECKING`, currency `"BRL"`, initial balance `1500.00`, and visibility `HOUSEHOLD_SHARED`,
- **Then** the system creates the account record, associates it with the household tenant, records the initial balance transaction, and makes the account immediately visible on both Luis's and Eluma's dashboards.

##### US-ACC-02: Create Private Personal Discretionary Account
- **Given** Eluma is logged in as an `ADMIN` under the "Família Marquitti" household tenant,
- **When** Eluma creates an account named `"Nubank Pessoal Eluma"`, type `CREDIT_CARD`, and visibility `PERSONAL_PRIVATE`,
- **Then** the account is successfully created,
- **And** the account and its subsequent transactions are exclusively visible to Eluma,
- **And** the account is completely excluded from Luis's views, queries, and household net worth aggregations.

##### US-ACC-03: Real-Time Account Balance Recalculation
- **Given** a checking account has a verified ledger balance of R$ 5,000.00,
- **When** an asynchronous batch ingestion inserts 10 new debit transactions totaling R$ 1,250.00 and 1 credit transaction of R$ 3,000.00,
- **Then** the system atomically updates the account's current balance to R$ 6,750.00 within the same database transaction block,
- **And** emits a balance updated domain event.

##### US-ACC-04: Multi-Tenant Account Isolation Enforcement
- **Given** Edson is authenticated under tenant `"Tenant-Edson"`,
- **When** Edson attempts to query `GET /api/v1/accounts/{id}` where `{id}` belongs to Luis's `"Itaú Conjunta"` in tenant `"Família Marquitti"`,
- **Then** the PostgreSQL Row-Level Security policy denies the read at the database engine level,
- **And** the API returns HTTP 404 Not Found (RFC 7807 problem details) with zero leakage of the account's existence.

---

### 4.2 Module 2: Ingestion Pipeline — Open Finance (Pluggy API)

#### Functional Requirements
- **FR-2.1**: The system shall integrate the Pluggy Connect Widget by generating short-lived (30-minute TTL) Connect Tokens via backend endpoint `POST /api/v1/openfinance/connect-token`.
- **FR-2.2**: The backend shall authenticate with the Pluggy API using server-to-server API Keys obtained via `POST https://api.pluggy.ai/auth`, automatically cached in-memory and refreshed before their 2-hour TTL expiration.
- **FR-2.3**: The system shall support a dual-mode configuration:
  - `PLUGGY_ENVIRONMENT=sandbox`: Free perpetual sandbox mode utilizing predefined mock bank connectors and test credentials.
  - `PLUGGY_ENVIRONMENT=production`: Live Open Finance sync for personal accounts.
- **FR-2.4**: The system shall expose an HTTPS webhook endpoint `POST /api/v1/webhooks/pluggy` to ingest real-time events (`item/created`, `item/updated`, `item/error`, `transactions/deleted`).
- **FR-2.5**: The webhook handler must verify the incoming request using a pre-shared secret header (`X-IntelliFinance-Webhook-Secret`) with constant-time string comparison (`crypto/subtle.ConstantTimeCompare`) before accepting payloads.
- **FR-2.6**: Upon receiving a valid webhook event, the handler must immediately return HTTP 202 Accepted and enqueue an asynchronous synchronization job (`sync_open_finance_item`) to prevent HTTP connection timeouts.

#### User Stories

##### US-OF-01: Connect Banking Account via Pluggy Sandbox (Portfolio Showcase Demo)
- **Given** a reviewer or recruiter is viewing the public IntelliFinance web demo,
- **When** the user clicks "Connect Bank" and selects "Itaú Sandbox",
- **And** inputs the sandbox credentials (`user-ok` / `password-ok`),
- **Then** Pluggy creates a sandbox item with status `UPDATED`,
- **And** the IntelliFinance backend links the item to the demo tenant,
- **And** enqueues a background sync task that populates the dashboard with 30 days of realistic mock transactions within 3 seconds.

##### US-OF-02: Handle Multi-Factor Authentication (MFA) Challenge
- **Given** a user connects an institution requiring two-step verification,
- **When** Pluggy transitions the item status to `WAITING_USER_INPUT` with parameter `{ name: "token", type: "number" }`,
- **Then** the IntelliFinance UI displays a secure MFA prompt requesting the code,
- **When** the user submits the code (or sandbox test token `123456`),
- **Then** the backend forwards the token to Pluggy, and the item transitions to `UPDATED`.

##### US-OF-03: Webhook Verification and Tamper Protection
- **Given** the IntelliFinance webhook receiver is listening at `/api/v1/webhooks/pluggy`,
- **When** an external caller sends a webhook payload with an invalid or missing `X-IntelliFinance-Webhook-Secret` header,
- **Then** the system immediately rejects the request with HTTP 401 Unauthorized,
- **And** logs a security warning with the source IP address,
- **And** dispatches zero background worker tasks.

##### US-OF-04: Graceful Degradation on Inactive Sandbox Items
- **Given** an automated test suite or user accesses an inactive sandbox item older than 30 days,
- **When** the synchronization worker encounters an HTTP 404 `ITEM_NOT_FOUND` from the Pluggy API,
- **Then** the system updates the item status in PostgreSQL to `OUTDATED`,
- **And** displays an informative UI banner inviting the user to reconnect the bank without crashing the application or emitting unhandled exceptions.

---

### 4.3 Module 3: Ingestion Pipeline — Zero-Cost Multi-Format Fallback

#### Functional Requirements
- **FR-3.1**: The system shall implement native Go parsers for four offline statement formats: OFX (1.02 SGML and 2.x XML), CSV (multi-dialect), Excel (.xlsx via `excelize`), and PDF (digital and scanned).
- **FR-3.2 (OFX Parser)**:
  - Must automatically detect character encoding from headers (`CHARSET:1252`, `ENCODING:USASCII`, or `UTF-8`) and decode ISO-8859-1 / Windows-1252 streams into UTF-8 using `golang.org/x/text/encoding/charmap`.
  - Must extract banking metadata: `<BANKID>`, `<ACCTID>`, `<DTPOSTED>`, `<TRNAMT>`, `<FITID>`, and `<MEMO>`.
- **FR-3.3 (CSV Sniffer & Parser)**:
  - Must auto-detect delimiters (`;`, `,`, `\t`) by sampling the initial 5 rows of the file.
  - Must identify Brazilian bank export dialects:
    - *Itaú*: `Data;Lançamento;Detalhes;Valor;Saldo`
    - *Nubank*: `date,category,title,amount`
    - *Banco Inter*: `Data Lançamento;Histórico;Descrição;Valor;Saldo`
    - *IntelliFinance Standard*: `date,description,amount,currency,account_name,category,type,external_id`
  - Must parse Brazilian decimal numbers (`"1.250,50"` -> `1250.50`) and invert Nubank credit card purchase signs so expenses are strictly negative.
- **FR-3.4 (Excel XLSX Parser)**:
  - Must use `github.com/qax-os/excelize/v2` with streaming row iteration (`Rows()`) to minimize memory footprint.
  - Must process multi-sheet workbooks, allowing the user to select specific sheets or auto-detecting the primary ledger tab.
- **FR-3.5 (Multimodal PDF OCR via Gemini 2.0 Flash)**:
  - Scanned PDF statements, photographic receipts, and Brazilian paystubs (holerites) shall be transmitted directly to Vertex AI Gemini 2.0 Flash as raw `application/pdf` binary buffers.
  - Gemini 2.0 Flash must extract multi-column transaction tables into structured JSON matching a strict schema.
  - For paystubs (holerites), the extractor must record gross pay, itemized deductions (INSS, IRRF, benefits), and net pay, creating one primary income transaction for the net credit.
  - Must fallback to local LiteLLM/Ollama (`llama3.2-vision`) when operating in Homelab offline mode.

#### User Stories

##### US-ING-01: Ingest Itaú Latin-1 OFX Bank Statement
- **Given** Luis exports an OFX statement from Itaú containing Latin-1 encoded special characters ("Transferência PIX", "Cartão Alimentação"),
- **When** Luis uploads the `.ofx` file via the web interface to his `"Itaú Conjunta"` account,
- **Then** the Go OFX parser decodes the stream to UTF-8 without character corruption,
- **And** extracts all `<STMTTRN>` blocks, mapping `<FITID>` to `external_id`,
- **And** enqueues transactions for deduplication and AI categorization.

##### US-ING-02: Auto-Detect Nubank CSV Inverted Credit Card Statement
- **Given** Eluma uploads a Nubank credit card bill CSV where purchases are formatted as positive numbers (e.g., `54.90`) and bill payments as negative (e.g., `-2500.00`),
- **When** the ingestion engine processes the file,
- **Then** the CSV sniffer recognizes the Nubank dialect,
- **And** inverts the numeric signs so purchases become negative outflows (`-54.90`) and payments become positive inflows (`+2500.00`),
- **And** assigns category hints from Nubank's native category column.

##### US-ING-03: Stream Ingest Large Historical Multi-Sheet Excel Budget
- **Given** Luis uploads a historical family spreadsheet `financeiro_luis_e_eluma.xlsx` containing 5,000 transaction rows across multiple annual tabs,
- **When** the Excel ingestion adapter processes the file,
- **Then** it reads the rows using a streaming memory buffer (< 15MB heap allocation),
- **And** successfully extracts valid dates, descriptions, and amounts while skipping non-data header blocks and formula error cells,
- **And** logs a summary report: `4,982 records extracted, 18 invalid rows skipped`.

##### US-ING-04: Multimodal PDF Extraction of Scanned Hospital Paystub
- **Given** Eluma uploads a scanned, photographic PDF paystub (`Recibo de Pagamento.pdf`) with skewed columns and watermark stamps,
- **When** the PDF ingestion worker sends the binary to Vertex AI Gemini 2.0 Flash,
- **Then** Gemini 2.0 parses the document layout visually and returns structured JSON:
  - Gross Income: R$ 12,000.00
  - Deductions: INSS R$ 900.00, IRRF R$ 2,100.00
  - Net Credit: R$ 9,000.00
- **And** the system creates an income transaction for R$ 9,000.00 with the itemized tax breakdown attached in transaction metadata JSON.

##### US-ING-05: Offline Homelab Ingestion Fallback
- **Given** the Homelab node `lm-claw` loses external internet connectivity,
- **When** a user uploads a statement PDF,
- **Then** the system detects the Vertex AI outage,
- **And** routes the document to the local LiteLLM proxy pointing to Ollama `llama3.2-vision` on `lm-claw:11434`,
- **And** successfully extracts the transaction rows without failing the user's upload.

---

### 4.4 Module 4: Transaction Processing & Deduplication Engine

#### Functional Requirements
- **FR-4.1**: Every transaction ingested into the system must be evaluated by a deterministic deduplication engine prior to database persistence.
- **FR-4.2 (Canonical Fingerprint Algorithm)**: The system shall calculate a 64-character hexadecimal SHA-256 fingerprint for every candidate transaction using the canonical pipe-delimited formula:
  $$\text{Fingerprint} = \text{SHA256}(\text{tenant\_id} \parallel \text{"\|"} \parallel \text{account\_id} \parallel \text{"\|"} \parallel \text{TO\_CHAR(date, 'YYYY-MM-DD')} \parallel \text{"\|"} \parallel \text{amount\_cents} \parallel \text{"\|"} \parallel \text{UPPER(TRIM(clean\_description))} \parallel \text{"\|"} \parallel \text{COALESCE(external\_id, '')} \parallel \text{"\|"} \parallel \text{COALESCE(sequence\_index, '0')})$$
  Where:
  - `date`: Formatted as ISO-8601 string `YYYY-MM-DD` anchored to Brazilian Official Time (`America/Sao_Paulo`).
  - `amount_cents`: Signed integer cents string (e.g. `"-12450"` for -R$ 124.50, `"4500"` for +R$ 45.00), preventing decimal formatting divergences.
  - `clean_description`: Uppercase string stripped of leading/trailing whitespace and common bank noise prefixes (`"COMPRA ELO "`, `"PIX TRANSF "`, `"DOC/TED "`, `"PAGTO ELETRO "`).
  - `external_id`: The bank's unique transaction identifier when present (e.g. OFX `<FITID>`, Pluggy transaction ID); empty string `""` when unavailable.
  - `sequence_index`: Monotonic integer index (stringified, e.g. `'0'`, `'1'`, `'2'`) representing occurrence position or statement row ordinal within the imported batch when `external_id` is empty. This prevents legitimate repeat same-day transactions (such as two R$ 20.00 Uber trips or repeated subway fares from CSV) from colliding.
- **FR-4.3**: The database schema must enforce a composite unique constraint: `CONSTRAINT uq_tenant_transaction_fingerprint UNIQUE (tenant_id, fingerprint)`.
- **FR-4.4**: Ingestion batch inserts must execute an idempotent upsert supporting pending-to-posted status progression:
  ```sql
  INSERT INTO transactions (...) VALUES (...)
  ON CONFLICT (tenant_id, fingerprint) DO UPDATE
  SET status = EXCLUDED.status, updated_at = NOW()
  WHERE transactions.status = 'PENDING' AND EXCLUDED.status != 'PENDING';
  ```
  The ingestion service must report exact counts of `inserted_count`, `updated_count`, and `duplicate_count`.
- **FR-4.5**: The system shall track transaction lifecycle statuses: `PENDING` (pre-authorization / unconfirmed), `COMPLETED` (posted / settled), `RECONCILED` (verified against monthly bank statement), and `VOID`. When a `PENDING` transaction transitions to `COMPLETED`, balance sync triggers automatically debit/credit the account balance.

#### User Stories

##### US-TX-01: Deterministic Deduplication on Overlapping Statement Uploads
- **Given** Luis uploads a January bank statement containing 150 transactions,
- **When** Luis accidentally re-uploads an overlapping statement covering January 15 to February 15 containing 75 duplicate January transactions and 75 new February transactions,
- **Then** the deduplication engine computes matching canonical SHA-256 fingerprints for the 75 duplicates and skips them,
- **And** inserts only the 75 new February transactions,
- **And** completes the ingestion job with summary: `75 inserted, 75 duplicates ignored, 0 errors`.

##### US-TX-02: Prevent Duplicate Webhook Replays from Pluggy
- **Given** Pluggy sends a webhook for a new credit card purchase,
- **And** due to a network glitch, Pluggy retries the identical webhook payload 10 seconds later,
- **When** both webhook tasks execute through the ingestion pipeline,
- **Then** the first task successfully inserts the transaction with its computed fingerprint,
- **And** the second task hits the unique constraint `uq_tenant_transaction_fingerprint`, skips insertion cleanly, and does not duplicate the expense on the dashboard.

##### US-TX-03: Handling Distinct Legitimate Same-Day Charges via FITID
- **Given** Luis buys coffee at "Café Central" twice in one morning for the exact same amount (R$ 8.50),
- **And** the bank assigns distinct `<FITID>` identifiers to each card swipe (`FITID-1001` and `FITID-1002`),
- **When** the OFX statement is ingested,
- **Then** the fingerprint algorithm incorporates the distinct `<FITID>` values,
- **And** generates two unique fingerprints, correctly recording both legitimate charges.

##### US-TX-04: Disambiguating Identical Same-Day Transactions in CSV Statements (The "Uber Ride" Case)
- **Given** Eluma takes two separate Uber rides on the same day, each costing R$ 20.00,
- **And** the exported CSV statement from Nubank provides no unique `<FITID>` or external transaction ID,
- **When** the CSV parser processes both rows in the statement batch,
- **Then** the ingestion engine assigns sequential `sequence_index` values (`0` and `1`) to the identical statement rows,
- **And** computes distinct canonical fingerprints: `SHA256(...|-2000|UBER TRIP||0)` and `SHA256(...|-2000|UBER TRIP||1)`,
- **And** successfully inserts both transactions into the database without collision or data loss, resulting in an accurate total deduction of R$ 40.00.

##### US-TX-05: Pending-to-Posted Transaction Lifecycle Settlement
- **Given** an authorized credit card purchase appears as `PENDING` with amount -R$ 150.00,
- **When** the transaction clears and settles 2 business days later, Pluggy sends an update with status `COMPLETED`,
- **Then** the ingestion upsert updates the existing row from `PENDING` to `COMPLETED`,
- **And** the database balance trigger executes, synchronizing the account balance accurately.

##### US-TX-06: Batch Insert Performance & Transaction Atomicity
- **Given** a parsed statement contains 2,000 transactions,
- **When** the ingestion service persists the batch to PostgreSQL,
- **Then** it utilizes an unlogged staging table with `pgx.CopyFrom` followed by atomic set-based `INSERT ... ON CONFLICT` inside a single database transaction,
- **And** completes the database write in under 150 milliseconds.

---

### 4.5 Module 5: AI-Driven Categorization & Feedback Loop

#### Functional Requirements
- **FR-5.1**: The system shall maintain a standardized two-tier category taxonomy:
  - *Income*: Salary (Salário/Holerite), Healthcare Clinic Revenue (Consultório/Clínica), Investments (Dividendos/Rendimentos), Transfers (Transferências Recebidas), Other Income.
  - *Expenses*: Housing (Aluguel/Condomínio), Utilities (Luz/Água/Gás/Internet), Groceries (Supermercado/Feira), Dining Out (Restaurantes/Delivery), Transportation (Combustível/Uber/Manutenção), Health & Wellness (Farmácia/Consultas), Education (Cursos/Escola), Leisure & Subscriptions (Streaming/Lazer), Discretionary Personal (Pessoal Luis / Pessoal Eluma), Taxes & Fees (Tributos/Tarifas).
- **FR-5.2**: The AI categorization service shall bundle unassigned transactions into batches (max 50 per batch) and transmit them to the `ports.CategorizationEngine`.
- **FR-5.3**: The prompt to Gemini 2.0 Flash must enforce a strict JSON Schema returning:
  `transaction_id`, `category_id`, `confidence` (float between 0.00 and 1.00), and `reasoning` (brief explanation).
- **FR-5.4 (Confidence Thresholding & Human-in-the-Loop)**:
  - If `confidence >= 0.85`: System automatically accepts the categorization, marks `is_ai_categorized = TRUE`, and sets `is_manually_verified = FALSE`.
  - If `confidence < 0.85`: System applies the predicted category as a tentative suggestion, but flags the transaction with a prominent "Needs Review" indicator in the UI.
- **FR-5.5 (User Override Learning Loop)**:
  - When a user manually changes or confirms a transaction's category via `PATCH /api/v1/transactions/{id}/category`, the system sets `is_manually_verified = TRUE` and records an immutable log in `category_correction_audit`.
  - Up to 15 recent user category corrections for that tenant must be injected into subsequent few-shot LLM prompts to continuously adapt to household spending nuances.

#### User Stories

##### US-AI-01: High-Confidence Auto-Categorization
- **Given** a transaction with description `"PÃO DE AÇÚCAR SÃO PAULO BR"` and amount `-R$ 342.10` is ingested,
- **When** the categorization engine analyzes the transaction,
- **Then** Gemini 2.0 Flash classifies it under `"Groceries / Supermercado"` with confidence `0.98` and reasoning `"Recognized Brazilian supermarket chain"`,
- **And** the transaction is marked as auto-categorized with no user intervention required.

##### US-AI-02: Low-Confidence Flagging for User Review
- **Given** an ambiguous transaction with description `"PAG*JoseSilva1293"` and amount `-R$ 150.00` is ingested,
- **When** the AI engine evaluates the transaction,
- **Then** it assigns category `"Services / Outros"` with confidence `0.65` and reasoning `"Generic peer-to-peer payment without merchant details"`,
- **And** the transaction appears in the "Review Needed" queue on the household dashboard.

##### US-AI-03: User Manual Override & Household Few-Shot Adaptation
- **Given** Eluma inspects the transaction `"PAG*JoseSilva1293"` flagged for review,
- **When** Eluma updates the category to `"Clínica / Manutenção Equipamento"`,
- **Then** the system updates the category, marks `is_manually_verified = TRUE`,
- **And** inserts an audit record into `category_correction_audit`,
- **And** when another payment to `"JoseSilva"` occurs next month, the LLM prompt includes this past correction as a few-shot example, correctly predicting `"Clínica / Manutenção Equipamento"` with high confidence.

##### US-AI-04: Resilient Fallback on AI Rate Limiting
- **Given** Vertex AI Gemini 2.0 returns an HTTP 429 Too Many Requests error during a large batch ingestion,
- **When** the categorization service catches the rate limit,
- **Then** it applies exponential backoff with jitter (initial retry 2s, up to 3 attempts),
- **And** if retries are exhausted, falls back to rule-based keyword matching without stalling the file ingestion job.

##### US-AI-05: Zero-Cost Homelab Categorization via LiteLLM
- **Given** the system is running in Homelab mode (`AI_PROVIDER=litellm`),
- **When** an ingestion worker dispatches a batch of 20 transactions for categorization,
- **Then** the request is forwarded to `http://192.168.3.10:4000/v1/chat/completions`,
- **And** LiteLLM invokes the local Ollama model (`qwen2.5-coder`), returning structured JSON categories without internet access or billable API costs.

---

### 4.6 Module 6: Household Co-Budgeting, Expense Splitting & Business Isolation

#### Functional Requirements
- **FR-6.1**: The system shall support expense splitting across household members with multiple split strategies:
  - `EQUAL_50_50`: Divides the expense equally (50% Luis, 50% Eluma).
  - `PROPORTIONAL_INCOME`: Divides the expense based on verified monthly income ratios (e.g., 60% Luis, 40% Eluma).
  - `CUSTOM_PERCENTAGE`: User specifies arbitrary percentages summing to 100%.
  - `EXACT_AMOUNT`: User allocates specific decimal amounts to each member.
- **FR-6.2**: Expense splits must be stored in the relational table `transaction_splits` referencing `transaction_id`, `user_id`, `amount`, and `percentage`.
- **FR-6.3**: Discretionary personal transactions flagged as `visibility = 'PERSONAL_PRIVATE'` must be hidden from other household members, but their net amount must be incorporated into account balance reconciliation so bank balances remain mathematically correct.
- **FR-6.4**: Business entities (such as `Clínica Eluma` or `Motoel`) must be partitioned under dedicated account tags or sub-tenants, completely isolating commercial revenue, operating expenses, and clinic payroll from the family living budget.

#### User Stories

##### US-BUD-01: Split Supermarket Expense 50/50 Between Household Partners
- **Given** Luis pays R$ 600.00 at the supermarket from the shared checking account,
- **When** Luis selects the transaction and applies the `"50/50 Household Split"` rule,
- **Then** the system creates two split records: R$ 300.00 assigned to Luis and R$ 300.00 assigned to Eluma,
- **And** the household monthly expense dashboard reflects each partner's accurate contribution.

##### US-BUD-02: Private Discretionary Purchase Shielding
- **Given** Luis buys a private anniversary gift for Eluma for R$ 450.00 using his personal Nubank card,
- **When** the transaction is ingested,
- **Then** Luis tags the transaction with `visibility = 'PERSONAL_PRIVATE'`,
- **And** when Eluma logs in, the merchant name `"Joalheria Central"` and line item details are completely concealed from her transaction list,
- **And** the joint cash flow summary displays only an aggregated entry: `"Private Discretionary Spending"`.

##### US-BUD-03: Segregate Medical Clinic Commercial Cash Flow
- **Given** Eluma receives a patient consultation payment of R$ 800.00 deposited into her `"Clínica Eluma (PJ)"` account,
- **When** the transaction is categorized under `"Consultório / Receitas"`,
- **Then** the system records it under the clinic business ledger,
- **And** excludes this revenue from the personal family living income, preventing distorted household savings rate calculations.

##### US-BUD-04: Monthly Household Split Reconciliation & Settle-Up
- **Given** at the end of the month, Luis has paid R$ 4,000.00 in joint expenses and Eluma has paid R$ 2,500.00 in joint expenses under an agreed 50/50 split policy,
- **When** Eluma views the `"Household Settlement"` dashboard,
- **Then** the system calculates the net imbalance: total joint spend is R$ 6,500.00 (R$ 3,250.00 each),
- **And** displays an actionable settlement instruction: `"Eluma owes Luis R$ 750.00 to balance monthly joint expenses"`,
- **And** allows one-click recording of the settling PIX transfer.

---

### 4.7 Module 7: Financial Analytics, Cash Flow & Reporting

#### Functional Requirements
- **FR-7.1**: The system shall compute real-time cash flow metrics: Total Monthly Inflow (Income), Total Monthly Outflow (Expenses), Net Monthly Savings, and Net Burn Rate.
- **FR-7.2**: The system shall aggregate expenses by primary category and subcategory over selectable time windows (Current Month, Last 3 Months, Year-to-Date, Custom Date Range).
- **FR-7.3**: The system shall compute budget variance reports comparing actual monthly spending against user-defined category budget targets, triggering visual warnings when spending exceeds 85% and 100% of budget.
- **FR-7.4**: All analytics queries must execute via indexed PostgreSQL aggregate queries running within the user's RLS tenant boundary, ensuring sub-50ms query execution without table-scanning across tenants.

#### User Stories

##### US-REP-01: View Monthly Household Burn Rate & Savings
- **Given** Luis and Eluma want to assess their financial trajectory for the current month,
- **When** they navigate to the `"Cash Flow Overview"` dashboard,
- **Then** the platform displays:
  - Total Income: R$ 24,500.00
  - Total Fixed Expenses: R$ 11,200.00
  - Total Variable Expenses: R$ 4,350.00
  - Net Savings: R$ 8,950.00 (Savings Rate: 36.5%)
- **And** the data renders in under 80 milliseconds.

##### US-REP-02: Category Spending Drift Warning
- **Given** the household has set a monthly dining out budget of R$ 1,500.00,
- **When** a new restaurant transaction of R$ 220.00 is ingested, bringing total dining spending to R$ 1,540.00,
- **Then** the analytics engine flags the `"Dining Out"` budget as exceeded (102.6%),
- **And** displays an amber warning badge on the dashboard.

##### US-REP-03: Export Complete Financial Ledger for Brazilian Tax Filing (DIRPF)
- **Given** Luis prepares the annual Brazilian Federal Tax Return (DIRPF),
- **When** Luis requests an export for tax year 2025,
- **Then** the system generates a validated CSV and Excel workbook containing:
  - Account balances as of 31/12/2024 and 31/12/2025.
  - Itemized deductible medical and educational expenses tagged with payee CNPJ/CPF.
  - Complete income records categorized by withholding entity.

---

## 5. Non-Functional Requirements (NFRs)

### 5.1 Performance & Latency
- **NFR-PERF-01 (API Latency)**: Core synchronous REST API read and write endpoints (`GET /api/v1/accounts`, `GET /api/v1/transactions`, `POST /api/v1/transactions`) must achieve a 95th percentile (p95) response time of **< 100 milliseconds** under a baseline load of 50 concurrent requests.
- **NFR-PERF-02 (Cold Start)**: The compiled Go binary deployed to Google Cloud Run must achieve an initial container cold-start latency of **< 250 milliseconds**, facilitated by small binary size (<25MB) and zero dynamic framework reflection.
- **NFR-PERF-03 (Batch Ingestion Throughput)**: The ingestion pipeline must process and store at least **1,000 transaction records per second** during bulk OFX, CSV, or database seeding operations using `pgx.CopyFrom`.
- **NFR-PERF-04 (File Upload Size)**: The API gateway must accept multipart file uploads up to **25 Megabytes** for digital and scanned PDFs, streaming the content directly to storage without buffering the full file in heap memory.

### 5.2 Security & Secret Management
- **NFR-SEC-01 (Zero-Leak Policy)**: No production credentials, API secrets, database passwords, private keys, or personal financial data shall ever be committed to the git repository.
- **NFR-SEC-02 (Automated Secret Scanning)**: Gitleaks must be integrated at two mandatory verification gates:
  1. A local pre-commit hook scanning staged commits.
  2. An automated GitHub Actions CI workflow scanning all pushes and pull requests.
  The Gitleaks configuration must include custom regex rules detecting Brazilian CPFs, CNPJs, bank account numbers, and Pluggy API keys.
- **NFR-SEC-03 (Cloud Secret Management)**: In production (GCP Cloud Run), secrets must be retrieved exclusively from **Google Cloud Secret Manager** and injected into the container environment via Cloud Run IAM service account bindings with least-privilege `roles/secretmanager.secretAccessor`.
- **NFR-SEC-04 (Token Encryption at Rest)**: Sensitive external tokens (Pluggy API keys, refresh tokens, webhook signing keys) stored in the database must be encrypted using **AES-256-GCM** with a 256-bit encryption key sourced from Secret Manager.
- **NFR-SEC-05 (Authentication & Session Tokens)**: API authentication must utilize stateless, signed JSON Web Tokens (JWT) using the HMAC-SHA256 (HS256) or EdDSA algorithm with a maximum expiration TTL of 1 hour, paired with secure HTTP-only refresh cookies.

### 5.3 Multi-Tenancy & Data Isolation
- **NFR-TEN-01 (Engine-Level Row-Level Security)**: All tenant-scoped database tables (`accounts`, `categories`, `transactions`, `transaction_splits`, `ingestion_jobs`, `audit_logs`) must have PostgreSQL Row-Level Security explicitly enabled (`ALTER TABLE ... ENABLE ROW LEVEL SECURITY`) and forced (`FORCE ROW LEVEL SECURITY`).
- **NFR-TEN-02 (Transactional Session Isolation)**: Go database connection pooling (`pgxpool`) must guarantee that session-level tenant variables never leak across requests. Every query or transaction must execute inside an explicit transaction block setting `SET LOCAL app.current_tenant_id = $1` and `SET LOCAL app.current_user_id = $2`.
- **NFR-TEN-03 (Zero Cross-Tenant Leakage)**: Under no operational or error condition shall a query executed by one tenant return rows belonging to another tenant. Queries omitting the tenant session variable must return zero rows.

### 5.4 Privacy & LGPD Compliance
- **NFR-PRV-01 (Data Minimization - LGPD Art. 6, III)**: The system shall collect and retain only the financial metadata necessary for accounting and budgeting. Raw bank login credentials and passwords are never collected, processed, or stored.
- **NFR-PRV-02 (Right to Erasure / Crypto-Shredding - LGPD Art. 18, VI)**:
  - When an individual user requests account deletion, the system must permanently hard-delete all personal accounts, private transactions, and uploaded raw files within 24 hours.
  - In a shared household tenant where transactions contribute to historical joint balances, personal PII must be crypto-shredded: user identity is permanently anonymized to `"Ex-Member [SHA256]"`, credentials revoked, and personal tags purged, preserving joint balance integrity without retaining personal data.
- **NFR-PRV-03 (Immutable Audit Trails - LGPD Art. 37 & 46)**: Every access to sensitive financial data, export of financial reports, modification of tenant settings, and deletion of records must be permanently logged to an append-only `audit_logs` table containing timestamp, tenant ID, user ID, client IP address, action, and resource ID.
- **NFR-PRV-04 (PII Masking in Observability)**: Structured logging (`log/slog`) must implement a redaction handler that masks Brazilian CPFs (`\d{3}\.\d{3}\.\d{3}-\d{2}` -> `***.***.***-**`), bank account numbers, and Bearer tokens before emission to stdout or Google Cloud Logging.

### 5.5 Reliability, Portability & Cloud Economics
- **NFR-REL-01 (Zero-Cost Idle State)**: In production on Google Cloud Platform, the entire application stack must scale to zero instances when no traffic is present, resulting in **$0.00 idle operational cost**.
- **NFR-REL-02 (Dual-Target Deployment Portability)**: The application must compile to a single, self-contained static Go binary capable of executing without code changes on both bare-metal Linux (`lm-claw` Debian 13) and containerized Google Cloud Run. Switching environments must require only configuration via standard environment variables.
- **NFR-REL-03 (Container Footprint)**: Production Docker container images must utilize multi-stage builds deploying onto `gcr.io/distroless/static-debian12` or `scratch`, with a total image size of **< 35 Megabytes** and zero shell utilities (`sh`, `bash`) to eliminate container attack surface.
- **NFR-REL-04 (Ingestion Job Fault Tolerance)**: Ingestion tasks that fail due to transient network or external API errors must automatically retry using exponential backoff with jitter up to a maximum of 5 attempts before transitioning to `FAILED` status.

---

## 6. Public Portfolio Showcase & Repository Hygiene Strategy

### 6.1 Clean Greenfield Repository Initiation
To completely eliminate the risk of leaking personal financial documents committed in historical legacy git commits (such as commit `8f57fb7` which contained over 25 personal bank statements and tax guides), the public GitHub project shall be launched as a **clean greenfield repository**:
1. A fresh `git init` is executed at `v1.0.0-scaffold` containing only pristine Go code, specifications, and documentation.
2. The legacy repository containing historical personal files is permanently archived in private, encrypted cold storage and excluded from public remotes.
3. Every commit message follows the Conventional Commits specification (`feat:`, `fix:`, `docs:`, `chore:`).

### 6.2 Synthetic Test Fixture Architecture (`cmd/fixturegen`)
All automated testing, local development, and live public demonstrations rely exclusively on **Synthetic Datasets** generated by a dedicated Go CLI utility (`cmd/fixturegen`):
- **Anonymized Personas**: Uses fictional Brazilian literary pioneers ("Carlos Drummond", "Clarice Lispector", "Machado de Assis").
- **Valid Algorithmic CPFs**: Generates mathematically valid CPFs using the official Mod11 algorithm, but strictly prefixed with designated test ranges (`000.xxx.xxx-xx`).
- **Realistic Brazilian Merchants**: Populates transactions from typical merchants across Brazil (Padaria Central, Supermercado Pão de Ouro, Farmácia Popular, Assinatura Streaming).
- **Synthetic File Formats**: Generates realistic synthetic `.ofx`, `.csv`, `.xlsx`, and multimodal `.pdf` statements in `test/fixtures/` for integration testing.

```
test/fixtures/
├── ofx/
│   ├── itau_checking_synthetic.ofx
│   └── nubank_card_synthetic.ofx
├── csv/
│   ├── itau_dialect_synthetic.csv
│   ├── nubank_dialect_synthetic.csv
│   └── intellifinance_standard_synthetic.csv
├── excel/
│   └── monthly_budget_synthetic.xlsx
└── pdf/
    ├── synthetic_bank_statement.pdf
    └── synthetic_holerite.pdf
```

### 6.3 Interactive Portfolio Showcase Capabilities
When published on GitHub, IntelliFinance provides an interactive, live demonstration capability:
1. Reviewers can launch the application locally via a single command: `docker compose -f docker-compose.homelab.yml up`.
2. Reviewers can connect a mock bank using the integrated Pluggy Sandbox widget, seeing transactions ingested, deduplicated, and categorized in real time.
3. Reviewers can upload synthetic statement fixtures (`test/fixtures/*`) and watch the background worker process and categorize transactions via local LLM or Vertex AI.

---

## 7. Data Governance, Schema Contracts & Database Invariants

### 7.1 Relational Entity Relationship Overview
The system relies on six core relational entity domains:

```
+---------------------------------------------------------------------------------------------------+
|                                      RELATIONAL DOMAIN SCHEMA                                     |
+---------------------------------------------------------------------------------------------------+
|                                                                                                   |
|    +-------------------+           +-----------------------+           +---------------------+    |
|    |      tenants      | 1-------* |     tenant_members    | *-------1 |        users        |    |
|    +-------------------+           +-----------------------+           +---------------------+    |
|              |                                                                    |               |
|              | 1                                                                  | 1             |
|              |                                                                    |               |
|              *                                                                    *               |
|    +-------------------+                                               +---------------------+    |
|    |     accounts      | 1-------------------------------------------* |     audit_logs      |    |
|    +-------------------+                                               +---------------------+    |
|              |                                                                                    |
|              | 1                                                                                  |
|              |                                                                                    |
|              *                                                                                    |
|    +-------------------+           +-----------------------+           +---------------------+    |
|    |   transactions    | 1-------* |  transaction_splits   | *-------1 |     categories      |    |
|    +-------------------+           +-----------------------+           +---------------------+    |
|              |                                                                                    |
|              | *                                                                                  |
|              |                                                                                    |
|              1                                                                                    |
|    +-------------------+                                                                          |
|    |  ingestion_jobs   |                                                                          |
|    +-------------------+                                                                          |
|                                                                                                   |
+---------------------------------------------------------------------------------------------------+
```

### 7.2 Database Invariants & Constraints
1. **Tenant Integrity**: Every financial entity (`accounts`, `categories`, `transactions`, `ingestion_jobs`, `audit_logs`) must carry a non-null `tenant_id` foreign key referencing `tenants(id)` with cascading deletes.
2. **Deterministic Fingerprint Uniqueness**: `CONSTRAINT uq_tenant_fingerprint UNIQUE (tenant_id, fingerprint)` guarantees zero duplicate transaction records across any combination of manual file imports, webhook deliveries, and background syncs.
3. **Monetary Precision**: All monetary values are defined as PostgreSQL `NUMERIC(15, 2)` and mapped to `shopspring/decimal.Decimal` in Go. Floating-point arithmetic (`float32`, `float64`) is strictly prohibited in financial calculations.
4. **Row-Level Security Enforcement**: Tables containing tenant data must enforce RLS at the engine level. No query can execute without an active transactional session setting `SET LOCAL app.current_tenant_id = '...'`.
5. **Immutable Audit Logs**: The `audit_logs` table is strictly append-only. `UPDATE` and `DELETE` operations are revoked from the application database role.

---

## 8. Success Metrics & Forensic Verification Gates

### 8.1 Key Performance Indicators (KPIs)

| Metric | Target | Measurement Method |
|---|---|---|
| **Operational Cloud Cost** | **$0.00 / month** idle | Google Cloud Billing Console (Cloud Run, Cloud Tasks, Secret Manager free tiers) |
| **API Response Time (p95)** | **< 100 ms** | Prometheus / Cloud Monitoring metrics on `/api/v1/*` endpoints |
| **AI Categorization Accuracy** | **> 92%** | Ratio of approved categorizations vs manual overrides in `category_correction_audit` |
| **Ingestion Deduplication Rate** | **100%** | Zero duplicate transactions inserted across synthetic fixture replay test suites |
| **Test Coverage** | **> 85%** | Go statement coverage: `go test -coverprofile=coverage.out ./...` |
| **Security Leak Vulnerabilities** | **0** | Gitleaks exit code 0 across all git commits, branches, and PRs |

### 8.2 Forensic Audit & Acceptance Verification Gate
This Product Requirements Document serves as the authoritative specification against which Worker M6 and the independent Forensic Auditor validate project completion. Acceptance requires:
- [x] Complete coverage of Luis & Eluma household co-budgeting, personal discretionary spending, and clinic business separation.
- [x] Complete coverage of invited friend/family multi-tenant isolation via PostgreSQL Row-Level Security.
- [x] Formal user stories in Given/When/Then format for all six functional modules.
- [x] Detailed dual-target execution model: Homelab (`lm-claw`) and GCP Serverless Production ($0 idle cost).
- [x] Dual-mode Open Finance (Pluggy Sandbox/Live) and 4-format fallback ingestion (OFX, CSV, Excel, PDF OCR).
- [x] Strict non-functional requirements for latency, security, LGPD privacy, and zero-leak git hygiene.
- [x] Absolute absence of placeholder strings, unfinished markers, or TODO stubs.
