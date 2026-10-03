# Legacy Architecture Post-Mortem & Comprehensive Security Audit

**Document Status:** Final / Authoritative  
**Date:** 2026-09-24  
**Classification:** Confidential — Internal Security Audit & Greenfield Architectural Mandate  
**Target Systems:** IntelliFinance Legacy Monorepo (TypeScript/Node.js) & Greenfield Platform (Go 1.22+)  
**Author:** Technical Documentation Author & Security Analyst (`teamwork_preview_worker_m1`)  

---

## Executive Summary

### 1. Purpose of the Legacy Platform
IntelliFinance was originally conceived as an all-in-one personal and household financial intelligence platform tailored to the Brazilian banking ecosystem. Its primary design objectives were to automate bank statement ingestion (spanning checking accounts, credit card invoices, and investment summaries), reconcile transactions across disparate banking institutions (principally Itaú, Nubank, and Banco Inter), automatically classify transactions via Large Language Models (LLMs), and provide real-time dashboard analytics tracking monthly burn rates, categorical distributions, and liquidity trends.

### 2. Legacy Technology Stack
The platform was architected as a full-stack TypeScript monorepo managed via Yarn Workspaces (Yarn v1.22.22) and orchestrated locally with Docker Compose:
- **API Backend (`apps/backend`)**: Express.js with Apollo Server 4 mounted at `/graphql`, implementing a schema-first GraphQL API, dual-token JWT authentication, and rate limiting.
- **Frontend SPA (`apps/frontend`)**: React 18, Vite 5/7, Material UI (MUI v5), Apollo Client 4, and Recharts.
- **Asynchronous Worker (`apps/worker`)**: BullMQ consumer backed by Redis 7, utilizing `PapaParse` for CSV processing and `pdf-parse` combined with LangChain (`@langchain/openai` and `@langchain/google-genai`) for LLM-driven transaction extraction and categorization.
- **Data Tier (`packages/database`)**: PostgreSQL 15 accessed via Prisma ORM v5, with multi-target binary engines (`native`, `linux-musl-openssl-3.0.x`).
- **Shared Libraries**: `packages/types` (TypeScript interfaces and Zod validation schemas), `packages/jobs` (BullMQ queue definitions and payload typings), `packages/logger` (Pino logger with correlation tracking).
- **Documentation (`apps/docs`)**: Docusaurus v3 documentation portal.

### 3. Project Ambition vs. Operational Reality
While the repository accumulated substantial architectural ambition and meta-governance documentation—including detailed AI agent workflows, Conductor tracks, and design tokens—the foundational software engineering execution stalled. The platform proved incapable of running reliably in its containerized target environment. Critical flaws in Docker multi-stage image packaging, broken workspace package resolution, container filesystem isolation, mathematical errors in financial aggregation, and naive ingestion parsers rendered the application inoperable for day-to-day use.

### 4. Core Outcome & Strategic Mandate
During the transition toward an open-source, public GitHub portfolio project, a deep forensic audit revealed a catastrophic data leakage event: **more than 90 real, highly confidential personal financial PDFs** (including Wipro salary payslips, Itaú bank statements, federal income tax returns, auto loan contracts, and medical patient records) and **over 70 CSV/XLSX files containing real personal names, CPFs, debts, and transaction histories** were committed to Git and pushed to the remote repository (`origin/dev`).

Additionally, the database seed script was hardcoded to inject real personal financial records into demo databases, and authentication endpoints defaulted to hardcoded, insecure secrets.

**Strategic Mandate:** The legacy repository **cannot be open-sourced or exhibited publicly under any circumstances**. Remediation via history rewriting tools (`git filter-repo` / BFG) is deemed unacceptably hazardous due to GitHub's dangling blob caches and fork propagation risks. The project must execute a **Clean Greenfield Strategy**: initializing a completely virgin Git repository at `v1.0.0-scaffold` in **Golang 1.22+**, enforcing automated Gitleaks CI/CD barriers, utilizing deterministic synthetic test fixtures, and implementing database-enforced Row-Level Security (RLS).

---

## Architecture & Component Inventory

```
+----------------------------------------------------------------------------------------------------+
|                                    INTELLIFINANCE MONOREPO TOPOLOGY                                 |
+----------------------------------------------------------------------------------------------------+
|                                                                                                    |
|   apps/frontend (React 18 + Vite + Apollo Client + MUI v5)                                         |
|         │                                                                                          |
|         │ GraphQL Queries / Mutations (HTTP :3000/graphql)                                         |
|         ▼                                                                                          |
|   apps/backend (Express + Apollo Server 4)                                                         |
|         │                                                                                          |
|         ├── Writes uploaded file to local container disk: /app/uploads/                            |
|         ├── Enqueues job in Redis (BullMQ queue: 'ingestion')                                      |
|         └── Prisma ORM Client ───► PostgreSQL 15 (Single 'User' scoping, No RLS)                   |
|                                                                                                    |
|   apps/worker (BullMQ Consumer + Node.js)                                                          |
|         │                                                                                          |
|         ├── Dequeues job from Redis                                                                |
|         ├── Attempts to read local /app/uploads/  ◄─── [CRITICAL FAILURE: ENOENT (No Shared Vol)]  |
|         ├── Parses CSV (PapaParse) or PDF (pdf-parse)                                              |
|         ├── LLM Extraction (LangChain + GPT-4o-mini / Gemini-1.5-flash with 25k char truncation)   |
|         └── Prisma ORM Client ───► PostgreSQL 15                                                   |
|                                                                                                    |
|   packages/                                                                                        |
|         ├── packages/database (Prisma schema & migrations; seed.ts reading real CSV)               |
|         ├── packages/types (Shared Zod schemas & TypeScript contracts)                             |
|         ├── packages/jobs (BullMQ queue constants: INGEST_PDF_JOB, INGEST_CSV_JOB)                 |
|         └── packages/logger (Pino structured logger with correlation IDs)                          |
+----------------------------------------------------------------------------------------------------+
```

### 1. Detailed Review of Applications and Packages

#### 1.1 `apps/backend` (`@intellifinance/backend`)
- **Runtime & Framework**: Node.js, Express.js, Apollo Server 4 mounted at `/graphql`.
- **GraphQL Engine**: Schema-first architecture. GraphQL SDL files are distributed across `src/graphql/schemas/` (`auth.graphql`, `accounts.graphql`, `transactions.graphql`, `ingestion.graphql`, `dashboard.graphql`) and concatenated at runtime via `fs.readFileSync`.
- **Security & Middleware**:
  - `express-rate-limit` configured globally.
  - Custom JWT context handler (`src/graphql/context.ts`) extracting bearer tokens.
  - Apollo query complexity analysis (`graphql-query-complexity`) configured with a maximum complexity limit of 100.
- **Defects & Limitations**:
  - Tightly coupled module instantiation: `queueService` initializes an `IORedis` connection globally upon module evaluation rather than inside a managed lifecycle or dependency injection container.
  - GraphQL SDL concatenation relies on synchronous filesystem reads (`readFileSync`), creating runtime startup hazards when directory paths diverge across development and Docker environments.

#### 1.2 `apps/worker` (`@intellifinance/worker`)
- **Runtime & Responsibilities**: Standalone Node.js process consuming jobs from the BullMQ queue (`ingestion`).
- **Processing Pipelines**:
  - `csv-import.processor.ts`: Parses CSV files using `PapaParse`, attempts to infer categories, and writes transactions to the database via Prisma transactions.
  - `pdf-extract.processor.ts`: Ingests bank statement PDFs, extracts raw text via `pdf-parse`, and forwards the textual stream to `llm.service.ts` for structured extraction.
- **Defects & Limitations**:
  - Filesystem isolation crash: Assumes file paths enqueued by `backend` exist on the local worker filesystem. Because containers do not share volumes, all non-local file imports crash with `ENOENT`.
  - Zero OCR capability: Scanned documents, image-only PDFs, and mobile camera receipts yield empty strings from `pdf-parse`, aborting the pipeline.

#### 1.3 `packages/database` (`@intellifinance/database`)
- **ORM & Database**: PostgreSQL 15 managed via Prisma ORM v5 (`schema.prisma`).
- **Binary Targets**: Configured for `native` and `linux-musl-openssl-3.0.x`.
- **Core Models**:
  - `User`: Primary account owner (`id`, `email`, `passwordHash`, `name`, `createdAt`, `updatedAt`).
  - `Account`: Bank or card account (`id`, `name`, `type`, `balance`, `currency`, `userId`).
  - `Category`: Transaction classification (`id`, `name`, `type`, `icon`, `color`, `userId`).
  - `Transaction`: Individual ledger record (`id`, `accountId`, `categoryId`, `amount`, `date`, `description`, `type`, `status`, `sourceFileUrl`).
  - `IngestionJob`: Asynchronous tracking entity (`id`, `accountId`, `fileUrl`, `status`, `resultSummary`, `errorDetails`).
- **Defects & Limitations**:
  - **Severe Schema Blindspot**: `Transaction` has no direct foreign key to `User` or `Tenant`. Scoping transactions to a user requires joining across `Account`. Any direct query on `Transaction` (e.g., global category aggregates) risks leaking data across users.
  - Single-user paradigm: Completely lacks models for households, multi-tenant workspaces, split expenses, or role-based access.

#### 1.4 `packages/jobs` (`@intellifinance/jobs`)
- **Queue Definitions**: Standardizes BullMQ queue names (`INGESTION_QUEUE = 'ingestion'`), job identifiers (`INGEST_CSV_JOB = 'ingest-csv'`, `INGEST_PDF_JOB = 'ingest-pdf'`), and payload TypeScript interfaces (`IngestionJobPayload`).
- **Defects**: Monorepo symlinking errors prevented this package from being resolved inside Docker runner images.

#### 1.5 `packages/types` & `packages/logger`
- **`@intellifinance/types`**: Exports Zod schemas (`ExtractedTransactionSchema`, `CategorizedTransactionSchema`) and DTO interfaces.
- **`@intellifinance/logger`**: Wraps Pino with structured JSON formatting and async correlation ID tracking via Node.js `AsyncLocalStorage`.

#### 1.6 Frontend Application (`apps/frontend`)
- **Stack**: Single Page Application (SPA) built with React 18, Vite (v5/v7), and Apollo Client 4.
- **UI Components**: Material UI (MUI v5) and Recharts for data visualization.
- **Implemented Views**:
  - Authentication (Login / Register).
  - Dashboard with summary KPIs (`balance`, `income`, `expense`).
  - Categorical spending breakdown (`CategoryChart` pie chart).
  - Monthly trendline (`TrendChart` bar chart).
  - Review queue table for unconfirmed transactions.
  - File upload dialog for CSV/PDF bank statements.

#### 1.7 Documentation (`apps/docs`)
- Docusaurus v3 static documentation site containing architecture overviews, developer onboarding guides, and an OpenAPI viewer plugin stub.

---

### 2. GraphQL API Inventory

| Operation | Type | Signature | Auth Required | Tenant Scoped | Implemented Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `register` | Mutation | `(input: RegisterInput!): AuthPayload!` | No | Public | Implemented |
| `login` | Mutation | `(input: LoginInput!): AuthPayload!` | No | Public | Implemented |
| `refreshToken` | Mutation | `(refreshToken: String!): AuthPayload!` | No | Public | Implemented |
| `accounts` | Query | `(): [Account!]!` | **Yes** | Yes (via `context.user`) | Implemented |
| `account` | Query | `(id: ID!): Account` | **Yes** | Yes (via `context.user`) | Implemented |
| `createAccount`| Mutation | `(input: CreateAccountInput!): Account!` | **Yes** | Yes (via `context.user`) | Implemented |
| `transactions` | Query | `(accountId: ID, from: Date, to: Date): [Transaction!]!` | **Yes** | Yes (via `Account.userId` join) | Implemented |
| `financialSummary` | Query | `(accountId: ID): FinancialSummary!` | **Yes** | Yes (via `Account.userId` join) | Implemented (Buggy math) |
| `uploadStatement` | Mutation | `(file: Upload!, accountId: ID!): IngestionJob!` | **NO (Vulnerability)** | **NO (Vulnerability)** | Insecure Implementation |
| `ingestionJob` | Query | `(id: ID!): IngestionJob` | **NO (Vulnerability)** | **NO (Vulnerability)** | Insecure Implementation (IDOR) |
| `spendingByCategory` | Query | `(timeframe: String): [CategorySpending!]!` | **Yes** | Yes | Implemented |
| `monthlyTrends` | Query | `(months: Int): [MonthlyTrend!]!` | **Yes** | Yes | Implemented |
| `reviewQueue` | Query | `(): [Transaction!]!` | **Yes** | Yes | Implemented |

---

### 3. End-to-End Data Ingestion Pipeline & Execution Trace

The legacy data ingestion workflow was designed as follows:
1. **Client Submission**: The client sends an `uploadStatement` multipart GraphQL mutation attaching a `.csv` or `.pdf` file alongside a target `accountId`.
2. **File Storage**: The backend `UploadService` resolves the upload stream, generates a pseudo-random filename (`crypto.randomUUID()`), and streams the bytes to a local directory:
   ```typescript
   // apps/backend/src/services/upload.service.ts:18
   this.uploadDir = join(process.cwd(), 'uploads');
   ```
3. **Job Scheduling**: The mutation creates a database row in `IngestionJob` with status `PENDING`, and enqueues a payload onto the Redis BullMQ queue containing `{ jobId, fileUrl, accountId }`.
4. **Worker Processing**:
   - The worker picks up the job.
   - For CSVs: `PapaParse` reads the file, parses rows, creates or looks up categories, and inserts transactions into PostgreSQL using Prisma.
   - For PDFs: `pdf-parse` reads raw textual streams. The text is passed to LangChain (`ChatGoogleGenerativeAI` or `ChatOpenAI`) using `withStructuredOutput(ExtractedTransactionSchema)` to extract date, description, amount, and suggested category.
5. **State Finalization**: The worker updates `IngestionJob` to `COMPLETED` or `FAILED`.

---

## Root Causes of Project Stall & Failure Modes

The failure of the legacy TypeScript platform was caused by systemic architectural, operational, and methodological friction points.

### 1. Container & Build Friction

#### 1.1 Node.js Version & ABI Incompatibility
Both `apps/backend/Dockerfile` and `apps/worker/Dockerfile` specified mismatched Node.js environments across multi-stage build targets:
```dockerfile
# apps/backend/Dockerfile
FROM node:22-alpine AS builder
...
FROM node:20-alpine AS runner
```
Building native C++ modules, OpenSSL hooks (`prisma-fmt`, `libssl3`), and bcrypt dependencies under Node 22 (V8 engine v12.4) and copying those compiled artifacts into a Node 20 runtime (V8 engine v11.3) introduced binary ABI mismatches, causing silent memory errors and container initialization aborts.

#### 1.2 Broken Monorepo Package Symlinks (`MODULE_NOT_FOUND`)
Yarn Workspaces manages internal packages (`@intellifinance/database`, `@intellifinance/jobs`, `@intellifinance/logger`, `@intellifinance/types`) by establishing symbolic links inside the root `node_modules` directory pointing to the respective local package subdirectories.

When Docker multi-stage builds executed:
```dockerfile
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/packages ./packages
COPY --from=builder /app/apps/backend/dist ./apps/backend/dist
```
The Docker filesystem copy operation dereferenced or severed the Yarn workspace symlinks. When the runtime container executed `node apps/backend/dist/index.js`, Node's CJS module resolver could not locate `@intellifinance/database`:

```
backend-1  | node:internal/modules/cjs/loader:1210
backend-1  |   throw err;
backend-1  |   ^
backend-1  | Error: Cannot find module '@intellifinance/database'
backend-1  | Require stack:
backend-1  | - /app/apps/backend/dist/graphql/resolvers/ingestion.resolver.js
backend-1  | - /app/apps/backend/dist/index.js
backend-1  |   code: 'MODULE_NOT_FOUND'
backend-1  | Node.js v20.20.0
```

Identical symlink failures crippled the background worker:
```
worker-1   | Error: Cannot find module '@intellifinance/jobs'
worker-1   | Require stack: [ '/app/apps/worker/dist/index.js' ]
worker-1   |   code: 'MODULE_NOT_FOUND'
backend-1  | Node.js v20.20.0
```

#### 1.3 Frontend Rollup / ESM Resolution Failures
The frontend build toolchain suffered from major package version mismatches:
- `apps/frontend/package.json` declared React Router DOM v7 alongside `@types/react-router-dom` v5.3.3.
- Under Vite 5/Rollup bundling, `@apollo/client` v4 core threw unresolved module dependency errors:
  ```
  [vite]: Rollup failed to resolve import "rxjs" from ".../node_modules/@apollo/client/core/ApolloClient.js"
  ```
Because peer dependencies were not pinned or installed, building the frontend container consistently failed.

---

### 2. File Handling & Storage Isolation: The "Upload Black Hole"

In `docker-compose.yml`, the backend and worker run as two completely separate container instances with isolated root filesystems:
- `backend` runs in container namespace A.
- `worker` runs in container namespace B.

When a user uploaded a file:
1. `backend` saved the file to `/app/uploads/extrato-123.csv` on **container A's ephemeral disk**.
2. `backend` enqueued the absolute string `/app/uploads/extrato-123.csv` into Redis.
3. `worker` (container B) popped the job from Redis and called `fs.readFileSync('/app/uploads/extrato-123.csv')`.
4. `docker-compose.yml` configured **no shared volume** for `/app/uploads`. Container B had an empty `/app/uploads/` directory.
5. The worker immediately threw `ENOENT: no such file or directory, open '/app/uploads/extrato-123.csv'`, crashing the ingestion job.

```
+--------------------------+                         +--------------------------+
|    CONTAINER: backend    |                         |    CONTAINER: worker     |
|                          |                         |                          |
|  Writes /app/uploads/    |                         |  Attempts to read:       |
|  statement-xyz.csv       |                         |  /app/uploads/           |
|            │             |                         |  statement-xyz.csv       |
+------------│-------------+                         +------------▲-------------+
             │                                                    │
             │ Enqueues: { fileUrl: "/app/uploads/..." }          │
             ▼                                                    │
      +──────────────+                                            │
      |    REDIS     | ───────────────────────────────────────────┘
      +──────────────+               Pulls Job Path
                                     Worker File System: EMPTY!
                                     CRASH: ENOENT
```

Furthermore, in `apps/backend/src/services/queue.service.ts`:
```typescript
export class QueueService {
  private ingestionQueue: Queue;
  constructor() {
    this.ingestionQueue = new Queue('ingestion', {
      connection: new IORedis(process.env.REDIS_URL)
    });
  }
}
export const queueService = new QueueService();
```
`new IORedis()` was invoked at the top-level module scope during file evaluation. If `REDIS_URL` was unavailable or invalid during testing, running migrations, or offline builds, importing any resolver threw an unhandled connection exception, halting the entire Node runtime.

---

### 3. Data Modeling Limitations

1. **Absence of Direct Tenant / User Foreign Key on Transactions**:
   In `packages/database/prisma/schema.prisma` (lines 74–96):
   ```prisma
   model Transaction {
     id            String            @id @default(uuid())
     accountId     String
     account       Account           @relation(fields: [accountId], references: [id])
     categoryId    String?
     category      Category?         @relation(fields: [categoryId], references: [id])
     amount        Decimal
     date          DateTime
     description   String
     type          TransactionType
     status        TransactionStatus @default(COMPLETED)
     sourceFileUrl String?
     createdAt     DateTime          @default(now())
     updatedAt     DateTime          @updatedAt
   }
   ```
   `Transaction` does not possess a `userId` or `tenantId` column. Data access scoping relies entirely on joining across `Account`. If an application developer queries `prisma.transaction.findMany({ where: { categoryId } })` or runs aggregate analytics without an explicit account join, transactions belonging to different users leak across tenant boundaries.

2. **Total Absence of Household / Co-Budgeting Concepts**:
   The legacy schema assumes every user is an isolated individual. It contains no domain entities for:
   - Households / Workspaces.
   - Shared accounts with co-ownership.
   - Private vs. shared transaction visibility.
   - Expense splitting (e.g., 50/50 household splits).
   - Multi-tenant isolation for invited friends/family.

3. **Zero Database-Level Row-Level Security (RLS)**:
   All security filtering was handled strictly at the Node.js application layer. Any missing `where: { userId }` clause in a GraphQL resolver directly resulted in an unauthorized data leak.

---

### 4. Processing & Ingestion Brittleness

#### 4.1 Balance Calculation Sign Arithmetic Bug
A severe mathematical inconsistency exists between how transaction amounts were seeded, how the CSV worker stored them, and how the GraphQL resolver aggregated balances:
- **Seed Script (`packages/database/prisma/seed.ts:183-212`)**: Expenses were stored as **negative numbers** (`amount: -3660.00`).
- **CSV Worker (`apps/worker/src/processors/csv-import.processor.ts:94`)**: Stored amounts as **positive numbers** (`amount: Math.abs(amount)`).
- **Backend Summary Resolver (`apps/backend/src/graphql/resolvers/transactions.resolver.ts:72-77`)**:
  ```typescript
  aggregations.forEach(agg => {
    const amount = agg._sum.amount ? parseFloat(agg._sum.amount.toString()) : 0;
    if (agg.type === 'INCOME') income += amount;
    if (agg.type === 'EXPENSE') expense += amount;
  });
  const balance = income - expense;
  ```
**The Arithmetic Failure Mode:**  
If a user had R$ 5,000 in income and a negative expense of `-R$ 3,660` (as produced by the seed script or signed bank exports):
$$\text{expense} = -3660$$
$$\text{balance} = \text{income} - \text{expense} = 5000 - (-3660) = 5000 + 3660 = 8660$$
The user's net balance inflated to R$ 8,660 instead of decreasing to R$ 1,340. The platform was fundamentally incapable of reliable financial accounting.

#### 4.2 Brittleness of `pdf-parse` & Complete Lack of OCR
In `apps/worker/src/services/pdf.service.ts`:
```typescript
export class PdfService {
  async extractText(filePath: string): Promise<string> {
    const dataBuffer = readFileSync(filePath);
    const data = await pdf(dataBuffer);
    return data.text;
  }
}
```
`pdf-parse` operates solely by extracting embedded PDF text objects (`/Text` streams). In real-world Brazilian banking:
- Itaú and Nubank PDF statements use complex multi-column grids. `pdf-parse` reads raw stream offsets, concatenating left-column dates with right-column balance numbers from unrelated rows into an unparseable text blob.
- Scanned receipts, boletos (`BOLETO+LUIS.PDF`), auto financing CET disclosures (`ADITIVO_CET.pdf`), and doctor consultation receipts (`Fluxo_Pacientes_Dra_Eluma.pdf`) consist of raster images without embedded text layers. `pdf-parse` returned empty strings (`""`), causing immediate extraction failure.

#### 4.3 Hardcoded 25k Character LLM Truncation
In `apps/worker/src/services/llm.service.ts` (lines 74–75):
```typescript
const result = await structuredModel.invoke([
  ["system", "You are a financial data extraction expert. Extract transactions from the text."],
  ["user", `Extract transactions from this text:\n\n${text.substring(0, 25000)}`]
]);
```
The text extracted from the PDF was hardcoded to a substring of the first 25,000 characters. For monthly credit card bills or extensive checking statements spanning 5 to 10 pages, transactions appearing after page 3 were silently dropped from ingestion without user notification.

---

### 5. Test Suite Fragility & Process Coupling

In `tests/integration/docker_bootstrap.test.ts`:
```typescript
beforeAll(() => {
  execSync('docker compose up -d --build --wait', { stdio: 'inherit' });
});
```
The Jest integration test suite synchronously spawned child processes executing `docker compose up --build --wait`. When run on host machines where Docker Desktop was offline or WSL2 named pipes were unavailable, the Jest test runner hung until timing out or threw unhandled pipe exceptions (`The system cannot find the file specified`), causing the entire integration test gate to abort.

---

### 6. Meta-Governance Overhead vs. Implementation Stall

A striking discovery of this post-mortem is the disparity between **meta-governance documentation** and **working software execution**.

The repository contained extensive directories dedicated to:
- `.ai/` and `.agent/`: Prompts, role definitions, and workflow protocols.
- `conductor/`: Complex track specifications, agile workstream epics, and handoff rituals.
- `.specify/`: Spec-mining frameworks.
- `design-stitch/` and `design-system/`: Design system tokens and styling specifications.

While hundreds of hours were invested authoring documents *about* how AI agents should build software, core engineering prerequisites were left unfulfilled:
- The containers did not boot.
- The monorepo packages did not resolve.
- The uploads were lost between containers.
- The balance calculations produced erroneous arithmetic.
- The Git repository was repeatedly polluted with confidential personal data.

**Key Engineering Lesson:** Process ceremony and planning documentation cannot substitute for running software, working integration tests, and rigorous automated security verification.

---

## Comprehensive Privacy & Security Audit

The security audit revealed multiple critical-severity vulnerabilities (CVSS 9.0–9.8) and pervasive exposure of personally identifiable information (PII) and sensitive personal data under the Brazilian General Data Protection Law (**LGPD - Lei 13.709/2018**).

---

### 1. Forensic Inventory of Leaked Real Personal & Financial Data

A thorough examination of the Git object database and working tree cataloged **96 tracked PDF documents**, **74 CSV files**, **17 XLSX spreadsheets**, **5 XLS files**, and **7 OFX bank exports** containing real personal data.

#### Comprehensive Category Inventory

| Category | File Paths / Patterns | Tracked Count | Sensitivity & Legal Classification | Exposed Data Elements |
| :--- | :--- | :--- | :--- | :--- |
| **Salary Payslips (Holerites)** | `data-sources/Google_Drive_Files/Finance/Holerites/*.pdf` | 65+ tracked PDFs (+ 5 untracked) | **Strictly Confidential / LGPD Art. 5** | Employer: Wipro; Full employee name (`Luis Paulo Angelini Marquitti`), CPF, monthly base salary, adiantamentos quinzenais, 13º salário, INSS/IRRF tax deductions, net remuneration (2024–2026). |
| **Federal Income Tax (DIRPF)** | `data-sources/Google_Drive_Files/Finance/Informes/*.pdf`, `Guia_DIRPF_2026_Luis_Marquitti.docx` | 4 tracked PDFs + Word doc | **Strictly Confidential / Fiscal Secrecy** | Complete Brazilian federal tax income reports (Informes de Rendimentos) from Itaú and Wipro for fiscal years 2025–2026. Complete CNPJs, taxable income, health deductions, banking account numbers. |
| **Personal Bank Statements** | `extrato-luis-2026.pdf` (1.04 MB, root), `extrato_transacoes.csv`, `data-sources/Google_Drive_Files/Finance/Extratos/**` | 5 tracked PDFs, 2 tracked CSVs (+ 3 untracked) | **Confidential Financial Banking Data** | Complete transaction records, account numbers, PIX transfer keys and recipient names, utility payments, school tuitions (`Escola LH`), debit transactions. |
| **Credit Card Invoices (Faturas)** | `data-sources/Google_Drive_Files/Finance/Faturas/Itau_Uniclass/*.pdf`, `fatura_mastercard_*.pdf`, `fatura_mercadopago_*.pdf` | 17 tracked PDFs | **Confidential Financial / Commercial** | Line-item purchases, merchant names, physical shopping locations, installment balances, cardholder names (`Luis`, `Eluma`). |
| **Normalized Financial Statements** | `data-sources/Google_Drive_Files/Finance/Normalizado/*.csv` | 30+ tracked CSVs | **Confidential Financial Structured Data** | Normalized tabular records containing personal names `luis` and `eluma`, account types, categories, and exact monetary amounts. |
| **Medical / Clinical Patient Records** | `data-sources/Google_Drive_Files/Negocios/Clinica_Eluma/Fluxo_Pacientes_Dra_Eluma.pdf` & `.xlsx` | 1 tracked PDF, 1 tracked XLSX | **Sensitive Personal Health Data (LGPD Art. 5, II)** | Full patient names, consultation dates, medical/clinical treatments, doctor identity (`Dra. Eluma`), consultation fee amounts. |
| **Auto Financing Contracts & Boletos** | `data-sources/Google_Drive_Files/Financiamentos/Onix/ADITIVO_CET.pdf`, `BOLETO+LUIS.PDF`, `financiamento_hb20s.xlsx`, `simulacao_itau.xlsx` | 2 tracked PDFs, 2 tracked XLSX | **Confidential Legal & Credit Contracts** | Vehicle financing agreements, CET disclosures, banking boletos with active barcodes, borrower identification, debt amounts. |
| **Real Estate & Business Spreadsheets** | `data-sources/Google_Drive_Files/Pessoal/Imoveis/Apartamento_44.xlsx`, `Negocios/Motoel/*.xlsx`, `Planilha Geral/financeiro_luis_e_eluma.xlsx` | 14 tracked XLSX | **Confidential Personal & Corporate Financials** | Apartment acquisition contracts, commercial venture debt statements (`Motoel`), family budget reconciliation sheets. |
| **Historical OFX Bank Exports** | `data-sources/Google_Drive_Files/Legado_Historico/Historico_Organizze_2022/itau_*.ofx` | 7 tracked OFX | **Confidential Financial Banking Exports** | Direct XML/SGML banking data from Itaú checking accounts for 2022, including `<FITID>` identifiers and transaction memos. |
| **Personal Loan Image Receipt** | `data-sources/Google_Drive_Files/Pessoal/Emprestimos/comprovante_emprestimo_li.jpg` | 1 tracked JPEG | **Confidential Financial** | Photographic image capture of private loan transfer receipt. |

---

### 2. Git History & Remote Repository Exposure

The audit traced the commits responsible for tracking confidential data and checked their distribution across remote branches:

```
Commit 53afbc49 ──► Pushed to origin/dev (Tracked data-sources/transactions-0126.csv)
Commit d61c186f ──► Pushed to origin/dev (Tracked extrato-luis-2026.pdf - 1.04 MB)
Commit 8f57fb76 ──► Committed locally on dev (Tracked 96 PDFs, Google Drive tree)
```

1. **Commit `d61c186f` ("chore(planning): unify and consolidate project governance in conductor/")**:
   - Committed `extrato-luis-2026.pdf` (1,038,417 bytes, adding 70,392 lines to the Git packfile).
   - Confirmed present on remote branch: `git branch -r --contains d61c186` returns `origin/dev`.
2. **Commit `53afbc49` ("feat(worker): Implement CsvDataSourceAdapter")**:
   - Committed `data-sources/transactions-0126.csv`.
   - Confirmed present on remote branch: `git branch -r --contains 53afbc4` returns `origin/dev`.
3. **Commit `8f57fb76` ("feat(docs-migration): import and organize Google Drive data sources and normalization scripts")**:
   - Added over 150 personal files from Google Drive into Git, including holerites, clinic patient flow, and tax reports.
4. **Gitignore Omissions**:
   The root `.gitignore` file (lines 1–30) had no entries for:
   - `data-sources/`
   - `*.pdf`
   - `*.ofx`
   - `*.csv`
   - `*.xlsx` / `*.xls`
   - `uploads/`
   - Sensitive logs (`worker_logs.txt`, `backend_logs.txt`, `setup_log.txt`) were initially committed to Git before being added to `.gitignore`, leaving them permanently tracked in Git history.

---

### 3. Database Seed Weaponization

In `packages/database/prisma/seed.ts`:
- **Line 102** hardcodes the ingestion of a real personal CSV file:
  ```typescript
  const seedDataPath = path.join(__dirname, "../../../data-sources/transactions-0126.csv");
  ```
- **Lines 126–222** iterate across all 126 real personal transaction records and insert them into the PostgreSQL database under the demo account (`demo@intellifinance.app`).
- **Verbatim Data Extract from `transactions-0126.csv`**:
  - Row 3: `02.01.2026,Eluma Alves dos Santos,Salário,"R$ 3.000,00"`
  - Row 4: `02.01.2026,Luis Paulo Angelini Marquitti,Dívidas e empréstimos,"-R$ 3.660,00"`
  - Row 9: `05.01.2026,Condominio Residencial Athenas,Condomínio ,"-R$ 1.046,15"`
- **Lines 63–64** set a hardcoded administrative password:
  ```typescript
  password: "rootpassword123"
  ```
**Consequence:** Running standard database initialization commands (`yarn db:seed` or `yarn db:reset`) immediately seeded real personal names, private family debts, school tuitions, and income transactions into any local or cloud deployment.

---

### 4. Hardcoded Secrets & Token Verification Failure (CWE-798)

1. **Authentication Token Verification Bypass**:
   In `apps/backend/src/services/auth.service.ts` (lines 7–8):
   ```typescript
   private static ACCESS_TOKEN_SECRET = process.env.ACCESS_TOKEN_SECRET || 'access_secret';
   private static REFRESH_TOKEN_SECRET = process.env.REFRESH_TOKEN_SECRET || 'refresh_secret';
   ```
   In `apps/backend/src/graphql/context.ts` (line 11):
   ```typescript
   const decoded = jwt.verify(token, process.env.ACCESS_TOKEN_SECRET || 'access_secret');
   ```
   However, `.env.dev.example` and `.env.prod.example` only define `JWT_SECRET`:
   ```bash
   # .env.dev.example:20
   JWT_SECRET="development_jwt_secret_key_change_me"
   ```
   Because `ACCESS_TOKEN_SECRET` is never defined in any configuration file, both token generation and verification **silently defaulted to the hardcoded string `'access_secret'`**. Any external party who reads the source code can forge valid JWT access tokens for any administrative or user account ID.

2. **Infrastructure IP Exposure**:
   `.env.dev.example` (line 9) publicly exposed an internal LAN IP and database credentials:
   ```bash
   DATABASE_URL="postgresql://if_dev:senha_de_desenvolvimento_segura@192.168.3.20:5432/intellifinance_dev?schema=public"
   ```

---

### 5. API Authorization Vulnerabilities & IDOR (CWE-639 / CWE-306)

In `apps/backend/src/graphql/resolvers/ingestion.resolver.ts`:

1. **Unauthenticated File Upload & Arbitrary Account Binding (CWE-306)**:
   ```typescript
   // apps/backend/src/graphql/resolvers/ingestion.resolver.ts:7
   uploadStatement: async (_: any, { file, accountId }: { file: Promise<GraphQLUploadFile>, accountId: string }) => {
     const filePath = await uploadService.saveFile(file);
     const job = await prisma.ingestionJob.create({
       data: {
         accountId,
         fileUrl: filePath,
         status: IngestionStatus.PENDING,
       },
       include: { account: true },
     });
     ...
   }
   ```
   The `uploadStatement` mutation lacks authentication guards (it never inspects `context.user`). An unauthenticated attacker can upload arbitrary files and trigger background worker jobs bound to any `accountId` in the database.

2. **Insecure Direct Object Reference (IDOR) (CWE-639)**:
   ```typescript
   // apps/backend/src/graphql/resolvers/ingestion.resolver.ts:29
   ingestionJob: async (_: any, { id }: { id: string }) => {
     return prisma.ingestionJob.findUnique({
       where: { id },
       include: { account: true },
     });
   }
   ```
   Any client can query any `IngestionJob` record by UUID without verifying whether the requesting user owns the underlying account, exposing file paths, ingestion status summaries, and account metadata.

---

## Clean Greenfield Repository Strategy

### 1. Formal Mandate for a Clean-Slate Repository
Under no circumstances should the existing Git repository or its commit history be transitioned to a public GitHub repository.

The project must establish a **Clean Greenfield Repository** initialized with a completely fresh Git history:
```bash
git init
git checkout -b main
git tag -a v1.0.0-scaffold -m "release: initial clean-slate greenfield architecture"
```
The legacy repository, along with the `data-sources/Google_Drive_Files` directory, must be archived into an encrypted, offline cold storage volume completely segregated from version-controlled source code.

---

### 2. Forensic Analysis: Why `git filter-repo` / BFG is Unacceptable for Public Showcase

Attempting to "clean" the existing repository using BFG Repo-Cleaner or `git filter-repo` presents unacceptable security risks:

```
+-----------------------------------------------------------------------------------------------+
|                       WHY HISTORY REWRITING IS HAZARDOUS FOR PUBLIC SHOWCASE                   |
+-----------------------------------------------------------------------------------------------+
| 1. Remote GitHub Blob Caches: Commits pushed to origin/dev (d61c186, 53afbc4) remain cached   |
|    in GitHub's internal commit cache. Anyone with the historical SHA-1 hash can directly view |
|    the raw 1.04 MB bank statement or transactions CSV via GitHub's API or web interface.      |
|                                                                                               |
| 2. Dangling Ref Blunders: BFG and filter-repo frequently leave unreferenced loose objects in   |
|    .git/objects/pack/ unless aggressive, multi-step git gc --prune=now commands are executed  |
|    flawlessly across all developer clones.                                                    |
|                                                                                               |
| 3. High Blast Radius of Failure: If 1 out of 96 personal PDFs escapes the scrubbing filter,  |
|    confidential salary holerites or patient clinical data become permanently public on GitHub.|
|                                                                                               |
| 4. Clean Showcase Presentation: A public engineering portfolio must exhibit clean, structured|
|    commits following Conventional Commits, not messy force-push rewritten histories with      |
|    broken signatures and synthetic merge artifacts.                                           |
+-----------------------------------------------------------------------------------------------+
```

---

### 3. Data Hygiene Rules for the Greenfield Go Platform

1. **Strict `.gitignore` Invariants**:
   The greenfield `.gitignore` must categorically block all document, financial export, and secret file extensions from the initial commit:
   ```gitignore
   # Confidential Personal & Banking Documents
   *.pdf
   *.ofx
   *.csv
   *.xlsx
   *.xls
   *.jpg
   *.jpeg
   *.png
   *.docx
   
   # Data and Ingestion Directories
   data-sources/
   uploads/
   tmp/
   
   # Secrets & Local Configurations
   .env
   .env.*
   !.env.example
   *.pem
   *.key
   
   # Build & OS Artifacts
   bin/
   dist/
   *.log
   .DS_Store
   ```

2. **Automated Gitleaks CI/CD & Pre-Commit Gates**:
   - **Local Pre-Commit Hook**: Enforced via `.husky` or native git hooks:
     ```bash
     gitleaks protect --staged --verbose --config=.gitleaks.toml
     ```
   - **Automated GitHub Actions CI**: Every push and pull request must execute `zricethezav/gitleaks-action@v8` using a custom `.gitleaks.toml` that blocks:
     - Brazilian CPFs (`\b\d{3}\.\d{3}\.\d{3}-\d{2}\b`).
     - Brazilian CNPJs (`\b\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}\b`).
     - Bank account formats and tokens.
     - Pluggy API credentials and JWT secret keys.

3. **Deterministic Synthetic Test Fixtures**:
   No real transaction data will ever be used in development, integration tests, or portfolio demonstrations. All test datasets must be generated dynamically via a dedicated CLI tool:
   - Location: `cmd/fixturegen/main.go`.
   - Output: `test/fixtures/` containing synthetic OFX, CSV, Excel, and PDF files.
   - Personas: Fictitious Brazilian literary names ("Machado de Assis", "Clarice Lispector"), mod11-validated test CPFs (`000.xxx.xxx-xx`), and standardized merchant categories.

4. **Secret Management Architecture**:
   - **Homelab Dev**: Environment variables loaded strictly from a local, gitignored `.env` file; `.env.example` in version control contains non-sensitive dummy placeholders.
   - **GCP Serverless Prod**: Cloud Run service accounts injecting secrets directly from **Google Cloud Secret Manager** with least-privilege IAM bindings.

---

### 4. Migration of Sanitized Business Logic into Go

The greenfield Go platform will re-implement and elevate the valid business concepts from the legacy codebase while discarding its architectural flaws:

```
+------------------------------------+          +------------------------------------+
|       LEGACY TYPESCRIPT/NODE       |          |         GREENFIELD GOLANG          |
+------------------------------------+          +------------------------------------+
| Yarn Monorepo (Symlink Failures)   | ───────► | Go 1.22+ Standard Project Layout   |
| Apollo GraphQL (Missing Auth)      | ───────► | Chi v5 REST API + OpenAPI 3.0 Specs|
| Prisma ORM (No RLS, Missing User)  | ───────► | PostgreSQL + sqlc + pgx + Native RLS|
| Local /app/uploads (ENOENT Crash)  | ───────► | StoragePort (Disk vs Google GCS)   |
| pdf-parse (Fails on Scanned/Tables)| ───────► | Vertex AI Gemini 2.0 Multimodal OCR|
| Real Data in seed.ts & Git         | ───────► | cmd/fixturegen Synthetic Fixtures  |
| Single User Model                  | ───────► | Household Multi-Tenancy & Splits   |
+------------------------------------+          +------------------------------------+
```

1. **Hexagonal Architecture (Ports and Adapters)**:
   Pure domain models (`Account`, `Transaction`, `Category`, `Household`, `AuditLog`) in `internal/domain/` with zero third-party framework dependencies.
2. **Dual-Target Execution**:
   - **Homelab Dev Target**: Self-hosted on local hardware (`lm-claw` / Debian 13 Trixie) utilizing Redis/Asynq, LiteLLM/Ollama fallback, and local filesystem storage.
   - **GCP Serverless Prod Target**: Cloud Run, Cloud Tasks push queues, Vertex AI SDK (Gemini 2.0 Flash consuming GenAI App Builder credits), and Cloud Storage.
3. **Database-Enforced Multi-Tenancy (Row-Level Security)**:
   Mandatory `tenant_id` on all relational tables, enforced natively in PostgreSQL via `ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;` and scoped via transactional session variables:
   ```sql
   SET LOCAL app.current_tenant_id = '...';
   ```
4. **Deterministic Deduplication Engine**:
   Idempotent transaction ingestion utilizing SHA-256 fingerprint hashing:
   $$\text{Fingerprint} = \text{SHA256}(\text{tenant\_id} \parallel \text{account\_id} \parallel \text{date} \parallel \text{amount} \parallel \text{clean\_description} \parallel \text{external\_id})$$
   Guaranteed by a unique index `(tenant_id, fingerprint)` with `ON CONFLICT DO NOTHING`.
5. **Robust Multimodal Statement Ingestion**:
   Replacing `pdf-parse` with Vertex AI Gemini 2.0 Flash structured outputs, enabling visual reconstruction of complex multi-column bank statements and image-scanned documents with zero truncation limitations.

---

## Conclusion & Architectural Sign-Off

The legacy IntelliFinance TypeScript/Node.js codebase reached an unrecoverable state characterized by operational failure and severe personal privacy compromises. Attempting to incrementally patch the existing codebase or rewrite Git history carries severe operational risk and leaves lingering privacy liabilities.

The strategic transition to a **Clean Greenfield Go Platform** resolves all identified failure modes:
1. Eliminates PII leakage via a virgin Git repository, Gitleaks automation, and synthetic test fixtures.
2. Eliminates monorepo packaging and container failures by compiling standalone Go binaries.
3. Eliminates file sharing black holes through unified storage ports.
4. Enforces ironclad privacy and multi-tenancy using PostgreSQL Row-Level Security.
5. Delivers an open-source portfolio project that demonstrates elite software engineering practices.

**Audit Status:** APPROVED FOR GREENFIELD RE-ARCHITECTURE  
**Next Milestone:** Greenfield Product Requirements Document (PRD) in `docs/PRD.md`.
