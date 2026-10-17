# ADR-009: Frontend Architecture — SPA Consuming the REST API

- **Status**: Proposed
- **Deciders**: Luis Eduardo Angelini Marquitti (Product Owner & Tech Lead), Software Architect
- **Date**: 2026-10-17
- **Technical Story**: Issue #36 — Frontend definition, specification & MVP architecture
- **Supersedes**: none
- **Related**: ADR-002 (API Architecture), ADR-008 (Public Showcase & Zero-Leak Security), `docs/specs/frontend.md`, `docs/specs/api.md`

---

## 1. Context & Problem Statement

IntelliFinance ships a production-grade backend: Hexagonal Go 1.22+, Chi v5 REST API, spec-first OpenAPI 3.0, RFC 7807 problem details, PostgreSQL 16 with Row-Level Security (RLS), and a dual-target runtime (Homelab `lm-claw` bare-metal and GCP Cloud Run serverless).

However, the repository currently lacks a graphical user interface, which blocks three explicit product deliverables:

1. **PRD §6.3 (Interactive Portfolio Showcase)**: An external evaluator, recruiter, or engineer running `make dev-up` must be able to launch the application locally, connect an account, and experience real-time ingestion, deduplication, and AI categorization. Without a UI, this demonstration degrades to raw terminal `curl` outputs.
2. **PRD §4.5 (Human-in-the-Loop AI Categorization)**: The system flags low-confidence transaction categorizations (`ai_confidence < 0.85`) for human verification. This review queue requires an interactive visual interface with optimistic category overrides.
3. **PRD §2 (Personas) & §4.7 (Financial Analytics)**: Household budget partners require an aggregated view of account balances, monthly cash flow, category distributions, and business expense segregation.

### Architectural Constraints:
- **Dual-Target Execution & $0 Cost**: The platform must execute seamlessly in Homelab mode (`lm-claw` Debian 13) and scale-to-zero in production (GCP Cloud Run), maintaining a monthly idle cost of exactly **$0.00** (`PRD §5.5`, `NFR-REL-01`).
- **Single-Artifact Distroless Container**: Production containers deploy onto `gcr.io/distroless/static-debian12` or `scratch`, with an image size budget of **< 35 Megabytes** and no shell utilities (`NFR-REL-03`). A separate Node.js server in production is impermissible.
- **Spec-First Contract Adherence**: The frontend must consume the authoritative OpenAPI 3.0.3 contract (`docs/specs/api.md` / `api/openapi/openapi.yaml`). Client-side types must be strictly derived from this schema to eliminate interface drift.
- **Financial Precision & Security**: Float arithmetic is strictly prohibited on monetary amounts (`PRD §7.2`). JWT access tokens must reside in volatile memory only (`NFR-SEC-05`), and tenant privacy must never be entrusted to client-side filtering.

**The decision required:** What architectural paradigm, stack, component system, and delivery strategy should the frontend adopt to optimize developer velocity, contract safety, and runtime simplicity based strictly on contemporary technical merit?

---

## 2. Decision Drivers

| # | Driver | Weight | Source |
|---|---|---|---|
| **D1** | **Single-Artifact Deployment (`embed.FS`) & Custo $0** — Assets must be embedded into the Go binary (`cmd/api`), maintaining a unified artifact without Node.js in production | Critical | PRD §5.5, NFR-REL-02/03 |
| **D2** | **End-to-End Type Safety & Anti-Drift** — Types strictly generated from `openapi.yaml`; contract drift breaks CI compilation | Critical | ADR-002, SPEC-002-API |
| **D3** | **Build Determinism & Clean Toolchain** — Hermetic, clean-clone reproducible build with locked dependencies | High | AGENTS.md §5 |
| **D4** | **Rich Reactivity & Asynchronous State Handling** — Optimistic UI for AI review queue, progress bars, and resilient polling for background ingestion | High | PRD §4.5, §6.3 |
| **D5** | **Developer Velocity for Solo Maintainer** — Ergonomic DX, mature headless component ecosystem, minimal boilerplate | High | Project Structure |
| **D6** | **Credential & Tenant Boundary Security** — Zero tokens in browser web storage (`localStorage`/`sessionStorage`), strict memory storage | High | NFR-SEC-05, ADR-008 |
| **D7** | **Zero Client-Side Float Arithmetic** — Absolute avoidance of IEEE 754 precision errors in currency rendering | High | PRD §7.2, AGENTS.md §2.1 |

---

## 3. Considered Options

The following architectural alternatives were evaluated strictly on technical merit:

1. **React 19 + Vite 6/7 + TypeScript + TanStack Query v5 + Tailwind CSS** (Static SPA embedded via Go `embed.FS`)
2. **Next.js 15 (App Router / SSR + RSC) + TypeScript** (Standalone Node.js Server or Static Export)
3. **Svelte 5 (Runes) + SvelteKit (Static Adapter) + TypeScript** (Static SPA embedded via Go `embed.FS`)
4. **Vue 3 + Vite + TypeScript + Pinia / TanStack Query** (Static SPA embedded via Go `embed.FS`)
5. **Angular 19 (Standalone Components + Signals)** (Static SPA embedded via Go `embed.FS`)
6. **Server-Rendered Go Templates (`html/template` + htmx + Tailwind CSS)** (Pure Go SSR, no JS framework)

---

## 4. Evaluation & Comparative Matrix

| Evaluation Criteria | React 19 + Vite (1) | Next.js 15 (2) | Svelte 5 Kit (3) | Vue 3 + Vite (4) | Angular 19 (5) | Go + htmx (6) |
|:---|:---:|:---:|:---:|:---:|:---:|:---:|
| **D1: Single Artifact & $0 Cost** | **5.0** (Static `embed.FS`) | 1.5 (Requires Node runtime) | **5.0** (Static `embed.FS`) | **5.0** (Static `embed.FS`) | 4.5 (Static `embed.FS`) | **5.0** (Native Go SSR) |
| **D2: OpenAPI Type Safety** | **5.0** (`openapi-typescript`) | 4.5 (Typed client) | 4.0 (Community tooling) | 4.2 (Typed client) | 3.8 (Generated SDK) | 2.0 (Untyped templates) |
| **D3: Build Determinism** | **4.5** (Vite bundler) | 3.0 (SSR/CSR boundaries) | 4.8 (Static adapter) | 4.7 (Vite bundler) | 4.0 (Angular CLI) | **5.0** (No JS bundler) |
| **D4: Reactivity & Polling** | **4.8** (TanStack Query v5) | 4.5 (React Server Actions)| 4.6 (Svelte Runes) | 4.5 (TanStack/Pinia) | 4.2 (Signals) | 2.5 (Complex state) |
| **D5: Solo Developer DX** | **4.5** (Headless UI ecosyst.) | 3.5 (Hydration edge cases) | 4.3 (Concise syntax) | 4.4 (Composition API) | 3.0 (Heavy verbosity) | 3.5 (Limited UI libs) |
| **D6: Security & In-Memory Auth**| **5.0** (Memory + httpOnly) | 4.5 (Cookie sessions) | 5.0 (Memory + httpOnly) | 5.0 (Memory + httpOnly) | 5.0 (Interceptors) | **5.0** (HttpOnly cookies)|
| **D7: Financial Precision** | **4.8** (String formatting) | 4.8 (String formatting) | 4.8 (String formatting) | 4.8 (String formatting) | 4.8 (Pipes) | **5.0** (Go decimal) |
| **Weighted Score (Total)** | **4.76** | **3.05** | **4.58** | **4.55** | **3.82** | **3.75** |
| **Verdict** | **Selected** | Rejected | Runner-Up | Viable | Rejected | Rejected |

---

## 5. Decision Outcome

**Adopt Option 1: React 19 + Vite 6/7 + TypeScript + TanStack Query v5 + Tailwind CSS, compiled into static production assets and embedded into the Go binary (`cmd/api`) via `embed.FS`.**

### Merit-Based Rationale:

1. **Optimal Artifact Unification (D1)**: Compiling to static HTML, JS, and CSS files allows Go's standard library `embed.FS` to bundle the entire frontend into `cmd/api`. This satisfies `NFR-REL-02` and `NFR-REL-03`:
   - A single static executable runs seamlessly on Homelab Linux (`lm-claw`) and Google Cloud Run.
   - The production container image is based on distroless (<35MB) with **zero Node.js runtime overhead** and zero shell attack surface.
   - Cloud Run cold start remains under 100ms with zero idle compute cost.
2. **Absolute Schema Consistency (D2)**: Utilizing `openapi-typescript` generates strictly typed TypeScript contracts directly from `api/openapi/openapi.yaml`. Any change to request schemas or response envelopes in the backend immediately surfaces as a compile-time type error in the frontend during CI execution.
3. **Advanced Server State Management (D4)**: TanStack Query v5 natively addresses the key interactive requirements of IntelliFinance:
   - Polling with exponential backoff for ingestion jobs (`GET /api/v1/ingestion/jobs/{id}`).
   - Optimistic mutations with automatic rollback on error for the AI transaction review queue (`PATCH /api/v1/transactions/{id}`).
   - Cache invalidation and background revalidation across accounts and transaction ledgers.
4. **Developer Ergonomics & Component Maturity (D5)**: Combining Tailwind CSS with unstyled headless primitives (such as Radix UI / shadcn/ui patterns) yields rapid development of accessible (WCAG AA), responsive, and visually refined interfaces without vendor lock-in.

### Why Alternative Options Were Rejected:
- **Option 2 (Next.js 15)** is rejected because its architectural model depends on a long-running Node.js process to support Server Components (RSC) and Server Actions. Running a separate Node.js container doubles the operational footprint and breaks the single-binary requirement. Using `output: 'export'` strips Next.js of its primary benefits, leaving unnecessary toolchain overhead.
- **Option 6 (Go templates + htmx)** is rejected because while excellent for static CRUD, it struggles with rich client-side states such as upload progress streams, interactive financial charts, and fine-grained optimistic rollbacks, requiring messy imperative script fallbacks.
- **Option 3 (Svelte 5 / SvelteKit)** is an outstanding runner-up with minimal bundle size, but lacks the comprehensive OpenAPI client ecosystem and breadth of headless financial UI primitives available in the React ecosystem.

---

## 6. Architectural Invariants & Detailed Design

### 6.1 Static Serving & Route Fallback
Static assets compiled into `web/dist` are embedded into Go via `internal/adapters/inbound/http/assets/assets.go` and served by the Chi router:
- `GET /assets/*`: Hashed static files served via `http.FileServer` with `Cache-Control: public, max-age=31536000, immutable`.
- `GET /*`: SPA fallback route serving `index.html` with `Cache-Control: no-cache` for client-side deep routing.
- The SPA fallback is registered **last** in Chi, ensuring API routes (`/api/v1/*`) and health probes (`/healthz`, `/readyz`) take absolute precedence.

### 6.2 Authentication & Credential Isolation
- **Access Token**: Short-lived JWT (TTL ≤ 1h) held **exclusively in volatile JavaScript memory**. Never stored in `localStorage`, `sessionStorage`, or readable cookies.
- **Refresh Token**: Issued by the backend as an `HttpOnly`, `Secure`, `SameSite=Strict` cookie scoped to `/api/v1/auth/refresh`.
- **Silent Refresh Mutex**: An HTTP client interceptor queues concurrent 401 responses, executing a single refresh handshake and replaying failed requests to eliminate race conditions.
- **Showcase Compatibility Mode**: In local development or preview environments where JWT auth is not yet provisioned, the client injects `X-Tenant-ID` directly via an in-memory tenant selector.

### 6.3 Financial Precision & Data Integrity
- Currency values arrive as formatted decimal strings (e.g., `"14250.75"`).
- The client formats values for display using `Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })`.
- Mathematical calculations and aggregations are performed **exclusively on the backend** (`PRD §7.2`). `parseFloat()` and `Number()` conversions on currency amounts are strictly prohibited.

### 6.4 Build Pipeline & Verification Guardrails

```
web/ (npm ci) ──► vite build ──► web/dist/ ──► internal/adapters/inbound/http/assets/dist/
                                                        │
                                                        ▼
                                       go build (embed.FS) ──► Single Distroless Binary
```

---

## 7. Implementation Phasing

- **Phase A (Specification & ADR Gate — PR #37)**: Complete formal specifications, merit evaluation, and EARS requirements.
- **Phase B1 (MVP Core — Existing Endpoints)**: Scaffold `web/`, wire `openapi-typescript`, implement Accounts (`/accounts`) and Transactions (`/transactions`) with inline category override, and configure Go `embed.FS` serving.
- **Phase B2 (Advanced Flows & Backend Extensions)**: Implement JWT Login/Refresh, Dashboard analytics, multipart Ingestion dropzone with polling, and Playwright E2E test suites.

---

## 8. References

- `docs/specs/api.md` (SPEC-002-API — Authoritative REST contracts)
- `docs/specs/frontend.md` (SPEC-007-FRONTEND — Detailed UI Specification)
- `.flow/tasks/gh-36/STACK_EVALUATION.md` (Comparative Merit Evaluation Matrix)
- `.flow/tasks/gh-36/DEVILS_ADVOCATE.md` (Adversarial Audit & Risk Mitigation)
- `docs/PRD.md` §4.5 (AI Review), §4.7 (Analytics), §6.3 (Showcase), §7.2 (Precision)
