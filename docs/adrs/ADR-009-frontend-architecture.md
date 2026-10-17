# ADR-009: Frontend Architecture — SPA Consuming the REST API

- **Status**: Proposed
- **Deciders**: Luis Eduardo Angelini Marquitti (Product Owner & Tech Lead), Software Architect
- **Date**: 2026-10-17
- **Technical Story**: Frontend definition & development (see issue)
- **Supersedes**: none
- **Related**: ADR-002 (API Architecture), ADR-008 (Public Showcase & Zero-Leak Security), `docs/specs/frontend.md`

---

## 1. Context & Problem Statement

IntelliFinance currently ships only a backend: Hexagonal Go 1.22+, REST API with Chi v5, spec-first OpenAPI 3.0, RFC 7807 problem details, PostgreSQL 16 with Row-Level Security, and dual-target runtime (Homelab + GCP Cloud Run).

There is **no user interface**, which blocks three explicit product goals:

1. **PRD §6.3 (Interactive Portfolio Showcase)** promises that a reviewer can launch the app locally, connect a sandbox bank, and "see transactions ingested, deduplicated, and categorized in real time". Without a UI this is unverifiable — the showcase degrades to `curl` output.
2. **PRD §2 (Personas)** requires Luis and Eluma to review household burn rate, reconcile splits, and segregate clinic revenue. All of that is dashboard-shaped work performed by non-engineers.
3. **PRD §4.5 (Human-in-the-Loop Categorization)** requires a "Needs Review" queue where a user overrides low-confidence AI categorizations. That flow has no surface.

Additional constraints from the greenfield reboot:

- The legacy platform's frontend (React 18 + Vite + Apollo Client 4 + MUI v5, per `docs/legacy_analysis_postmortem.md` §1.6) failed to build: React Router DOM v7 was declared alongside `@types/react-router-dom` v5.3.3, and `@apollo/client` v4 core threw unresolved `rxjs` imports under Vite 5/Rollup (§1.3). The frontend was a primary contributor to the platform being inoperable.
- ADR-002 §5 explicitly rejected GraphQL in favor of REST, eliminating the legacy frontend's Apollo Client dependency strategy.
- ADR-008 mandates zero real PII in any artifact, including demo data rendered in the UI.
- An unreferenced design prototype exists at `design/stitch/transactions-page/` (HTML + screenshot) with established theme tokens (primary `#0D47A1`, light/dark palettes, Roboto + Material Symbols).

**The decision required:** what stack, integration model, and delivery/serving strategy should the frontend adopt, and how is the legacy build failure structurally prevented rather than merely avoided?

---

## 2. Decision Drivers

| # | Driver | Weight | Source |
|---|---|---|---|
| D1 | **Build determinism** — a clean clone must build without module-resolution or peer-dependency failures | Critical | Postmortem §1.3 |
| D2 | **REST integration** — consumes the existing OpenAPI 3.0 contract, typed against it | Critical | ADR-002 |
| D3 | **Single-artifact deployment** — must not break the dual-target (Homelab binary + Cloud Run distroless <35MB) story | High | PRD §5.5, NFR-REL-02/03 |
| D4 | **$0 operational cost** — no paid hosting, CDN, or SaaS build service | High | PRD §1.2 |
| D5 | **Time-to-first-screen** — the audience is a recruiter on a 5-minute budget | High | PRD §6.3 |
| D6 | **Type safety end-to-end** — types generated from `openapi.yaml`, drift breaks the build | High | ADR-002 §6.1 |
| D7 | **Token security** — no long-lived credential in `localStorage` | High | NFR-SEC-05 |
| D8 | **Maintainability by one developer** — minimum toolchain surface | Medium | Project shape |
| D9 | **Visual fidelity to the existing prototype** | Medium | `design/stitch/` |

---

## 3. Considered Options

1. **React 19 + Vite 7 + TypeScript + TanStack Query + Tailwind** (client-rendered SPA, static assets embedded in the Go binary via `embed.FS`)
2. **Next.js 15 (App Router) + TypeScript + TanStack Query**, deployed as a separate Node service
3. **Svelte 5 + SvelteKit (static adapter) + TypeScript**, assets embedded in the Go binary
4. **Server-rendered Go templates** (`html/template` + htmx), no JS framework
5. **Revive the legacy React 18 + Apollo Client + MUI v5 stack**

---

## 4. Evaluation & Comparative Matrix

| Criteria | React+Vite (1) | Next.js (2) | Svelte 5/SvelteKit (3) | Go templates+htmx (4) | Legacy React+Apollo (5) |
|---|---|---|---|---|---|
| **Build determinism (D1)** | **High** — single bundler, one lockfile, no SSR/CSR split | Medium — SSR/RSC boundaries, server-only vs client-only module resolution | **High** — static adapter, minimal surface | **Highest** — no JS dependency tree at all | **Low** — documented root cause of legacy failure |
| **REST integration (D2)** | **Native** — `openapi-typescript` + TanStack Query | Native | Native | Manual | Poor — Apollo is GraphQL-first |
| **Single artifact (D3)** | **Yes** — `embed.FS` into the existing Go binary | **No** — needs a second container/runtime | Yes | **Yes** — already server-rendered | Partial |
| **$0 cost (D4)** | Yes | Yes, but adds a paid-tier-shaped surface (build minutes, server runtime) | Yes | Yes | Yes |
| **Time-to-first-screen (D5)** | Fast | Medium — more concepts before first screen | Fast | **Fastest** for read-only screens; slow for interactive ones | Slow — must repair the build first |
| **End-to-end types (D6)** | **Yes** — generated from `openapi.yaml` | Yes | Yes | No — Go structs only, drift undetected | No |
| **Token security (D7)** | Yes — memory + httpOnly refresh cookie | Yes | Yes | **Best** — token server-side, never in JS | Risky — Apollo cache persistence defaults |
| **Maintainability (D8)** | Medium — large but familiar toolchain | Lower — SSR/RSC + server runtime + caching semantics | **High** — smallest toolchain of the SPA options | High | Low |
| **Visual fidelity (D9)** | **High** — prototype is Tailwind-based | High | Medium — needs token porting | Low — no component ecosystem | High — but MUI ≠ Tailwind prototype |

---

## 5. Decision Outcome

**Adopt Option 1: React 19 + Vite 7 + TypeScript + TanStack Query + Tailwind CSS, built as static assets and embedded into the Go binary via `embed.FS`, served by `cmd/api`.**

### Rationale

1. **D1 is the decisive criterion.** The legacy project did not fail architecturally — it failed at the build. Vite with a single lockfile, fixed versions, and no SSR/CSR module boundary is the option with the fewest ways to break. Next.js reproduces exactly the class of problem that killed the legacy frontend: modules that resolve in one rendering context and not the other.
2. **D3 preserves the dual-target story.** The deployment artifact remains a single Go binary. `embed.FS` already appears in the Phase 5 plan (ROADMAP §7.1) for Swagger UI, so this reuses an established, already-accepted mechanism instead of introducing a second runtime. A separate Node container would contradict NFR-REL-03 (distroless, <35MB, no shell).
3. **D2 and D6 reinforce each other.** `openapi-typescript` generates types from the same `api/openapi/openapi.yaml` that `oapi-codegen` compiles server-side. One contract, two generated artifacts, and drift becomes a build failure rather than a production bug.
4. **D9 is free.** The prototype at `design/stitch/transactions-page/code.html` is already Tailwind with an inline `tailwind.config` carrying the exact palette. Adopting Tailwind means porting tokens, not redesigning.
5. **D5 is satisfied by scope control, not by framework choice.** Five screens with TanStack Query against a documented REST API is a small, well-understood build.
6. **Option 5 is rejected outright.** It reintroduces the documented failure mode and depends on GraphQL, which ADR-002 rejected.
7. **Option 4 is rejected on interactivity.** The Review Queue, category override, multipart upload with job polling, and paginated filtering are genuinely interactive surfaces. htmx would work for the dashboard and fight everything else.
8. **Option 3 is a credible runner-up** and the strongest choice on D1/D8. It loses on D9 (token and component porting cost) and on hiring-portfolio legibility, which is an explicit product goal.

### Consequences

**Positive**
- The deployment artifact stays a single static Go binary; Cloud Run and Homelab behavior is unchanged.
- Frontend and backend share one API contract with generated types on both sides.
- Tailwind tokens port directly from the existing design prototype.
- No second runtime, no SSR cache semantics, no server-only/client-only module classes.
- A reviewer gets the promised PRD §6.3 experience from `make dev-up` alone.

**Negative**
- Client-rendered SPA: no SEO and a blank first paint before the bundle loads. Acceptable — this is an authenticated financial app, not a public marketing site. The README carries the discovery burden.
- `embed.FS` requires the frontend build to run *before* the Go build. This ordering must be encoded in `Makefile` and CI, or the binary ships stale assets.
- React's toolchain footprint is larger than Svelte's; contrived as it is, that cost is accepted in exchange for ecosystem maturity.
- Bundle size must be watched to keep the distroless image under the NFR-REL-03 budget.

**Neutral**
- All frontend work lives under `web/`, keeping the Go module tree untouched (`go build ./...` must not see it).
- Tailwind v4 CSS-first configuration may diverge from the prototype's `tailwind.config` object; tokens are normalized in `design/tokens.md`.

---

## 6. Detailed Design Decisions

### 6.1 Serving & Base Path

Static assets are compiled into `internal/adapters/inbound/http/assets/` and served by the existing Chi router:

| Route | Behavior |
|---|---|
| `GET /assets/*` | Hashed static assets (`http.FileServer` over `fs.Sub(embedFS, "dist/assets")`), long-lived immutable cache headers |
| `GET /*` (non-`/api`, non-`/healthz`, non-`/docs`) | SPA fallback — serves `index.html` so client-side routing works on deep links |
| `GET /api/v1/*` | Unchanged REST API |
| `GET /healthz`, `/readyz` | Unchanged liveness/readiness |

The SPA fallback must be registered **last** in the Chi route tree so it never shadows API routes. API routes are matched first by Chi's routing tree; the fallback applies only to unmatched paths.

### 6.2 Authentication & Token Strategy

Per NFR-SEC-05 and driver D7:

- Access token (JWT, ≤1h TTL) held **in memory only** — a module-level variable, never `localStorage`, never `sessionStorage`, never a non-httpOnly cookie.
- Refresh token delivered as an **httpOnly + Secure + SameSite=Strict cookie** by the backend.
- On app boot, the SPA issues a silent refresh to rehydrate the session; a failure routes to `/login`.
- Logout calls the backend to revoke the refresh cookie and clears in-memory state.
- A 401 from any endpoint triggers a single refresh-and-retry, then a redirect to `/login` on the second failure.

This requires backend support for refresh-token issuance, tracked as a prerequisite in `docs/specs/frontend.md` §3.

### 6.3 Error Handling

The API returns RFC 7807 problem details with `Content-Type: application/problem+json`. The client:

1. Parses `type`, `title`, `status`, `detail`, and `invalid_params`.
2. Renders `title` as the message and `detail` as the supporting text.
3. Maps `invalid_params[].name` to the corresponding form field and displays `reason` inline.
4. Falls back to a generic message when the response body is not parseable, preserving the HTTP status.

No error path may render a raw JSON blob to the user.

### 6.4 Financial Value Handling

Monetary values arrive as decimal strings. The client:

- **Never** parses amounts to `number` for arithmetic.
- Renders with `Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })`.
- Performs any aggregation server-side; if client-side summation is unavoidable, it uses a decimal library, not floats.

This mirrors PRD §7.2 and AGENTS.md §2.1.

### 6.5 Privacy & Tenant Boundaries

- The client renders exactly what the API returns. It **never** infers or reconstructs visibility.
- No client-side filtering is used as a privacy control — RLS plus tenant middleware are the only enforcement points.
- No real PII in dev/demo data: all fixtures originate from `cmd/fixturegen` (ADR-008 §7).

### 6.6 Build Ordering Invariant

```
web/ (npm ci) → vite build → web/dist/ → copied to internal/adapters/inbound/http/assets/dist/
                                          → go build (embed.FS) → single binary
```

Enforced by:
- `Makefile` targets ordering `build-web` before `build-api`.
- A CI stage that builds the frontend and fails if `web/dist` is absent before the Go build.
- A test asserting the embedded FS is non-empty, so a misordered build fails loudly instead of shipping a blank page.

---

## 7. Implementation & Verification Plan

### Phase A — Decision & Specification (this ADR's merge gate)
1. Merge `ADR-009` and `docs/specs/frontend.md` into `dev`.
2. Add `design/tokens.md` derived from `design/stitch/transactions-page/code.html`.
3. Update `README.md`'s documentation table to reference both new documents.

**Verification:** documents exist, are internally consistent with ADR-002/ADR-008, and are linked from the README. No implementation code in Phase A.

### Phase B — MVP Implementation
4. Scaffold `web/` with Vite + React 19 + TypeScript, fixed dependency versions, committed lockfile.
5. Generate API types from `api/openapi/openapi.yaml`; wire TanStack Query.
6. Implement the five screens per `docs/specs/frontend.md`: Login, Dashboard, Transactions, Accounts, Ingestion.
7. Integrate the build into `Makefile` and `.github/workflows/ci.yml`.
8. Add the embedded-FS non-empty test.

**Verification gates:**
```bash
# Build determinism (D1) — from a clean clone
rm -rf web/node_modules web/dist && npm --prefix web ci && npm --prefix web run build
test -f web/dist/index.html || exit 1

# Type drift detection (D6) — regeneration must produce no diff
npm --prefix web run gen:api && git diff --exit-code web/src/api/generated

# Go build with embedded assets (D3)
go build ./... && go test -race ./...

# Lint (AGENTS.md §5)
golangci-lint run
npm --prefix web run lint

# End-to-end against the real local stack (D5, acceptance criteria)
make dev-up
curl -fsS localhost:8080/healthz
curl -fsSI localhost:8080/ | grep -i content-type
curl -fsSI localhost:8080/transactions | grep -i content-type   # SPA fallback
```

**Completion criteria:**
- `npm ci && npm run build` succeeds from a clean clone with no module-resolution or peer-dependency error.
- The five screens are navigable against the local backend with synthetic data.
- All acceptance criteria in `docs/specs/frontend.md` pass.
- The Go binary serves the SPA and the API from a single artifact.
- Zero real PII anywhere in the repository or rendered output.

---

## 8. References

- `docs/PRD.md` §2 (Personas), §4.5 (AI Categorization Review), §6.3 (Portfolio Showcase), §7.2 (Precision)
- `docs/ROADMAP.md` §7 (Phase 5 Portfolio Polish)
- `docs/legacy_analysis_postmortem.md` §1.3 (Frontend build failures), §1.6 (Legacy frontend stack), §6 (Meta-governance vs. shipping)
- `docs/adrs/ADR-002-api-architecture.md` (REST over GraphQL, OpenAPI codegen)
- `docs/adrs/ADR-008-public-showcase-security.md` (Zero-leak, synthetic fixtures)
- `docs/specs/api.md` (endpoint contracts and envelopes)
- `docs/specs/frontend.md` (UI specification — see companion document)
- `design/stitch/transactions-page/code.html` (visual reference)
