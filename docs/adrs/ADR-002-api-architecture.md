# ADR-002: API Architecture (RESTful API with Chi v5, OpenAPI 3.0, and RFC 7807)

- **Status**: Accepted
- **Deciders**: Software Architect (Worker M3), Principal Engineer, API Security Lead
- **Date**: 2026-09-24
- **Technical Story**: Milestone 3 - Architecture Decision Records

---

## 1. Context & Problem Statement

The legacy IntelliFinance prototype utilized an Apollo GraphQL server built on Express and TypeGraphQL. While GraphQL offers client-driven query flexibility, its adoption in IntelliFinance introduced substantial architectural and operational liabilities:

1. **Query Complexity & Resource Starvation**: Unrestricted nested queries allowed clients to execute costly relational traversals (e.g. `User -> Accounts -> Transactions -> Categories -> Transactions`), requiring complex runtime query depth and complexity limiters that added CPU latency to every request.
2. **File Upload Incompatibility**: Standard GraphQL specifications do not natively support multipart/form-data streaming. Uploading dense CSVs, OFX files, and multi-megabyte PDF bank statements required non-standard multipart request specifications (`graphql-upload`), which proved unstable and prone to memory leaks.
3. **Lack of Native HTTP Semantics & Caching**: All GraphQL requests route through `POST /graphql` with HTTP 200 OK responses, even on application errors. This bypassed standard HTTP caching mechanisms, proxy CDN layers, and Google Cloud Run request metrics.
4. **Security Exposure**: Unauthenticated schema introspection exposed internal domain entities and unfinished mutations to external scanners.
5. **Tooling Overhead**: Maintaining GraphQL schema definitions, TypeScript resolvers, codegen pipelines, and client query fragments created significant maintenance overhead for a family financial platform.

IntelliFinance requires a clean, high-performance API transport layer adhering to industry standards, featuring compile-time type safety, standard `net/http` compatibility, strict contract enforcement, and predictable error handling.

---

## 2. Decision Drivers

- **Standard `net/http` Compatibility**: 100% interoperability with the Go standard library, enabling seamless integration with Google Cloud Run, OpenTelemetry HTTP instrumentation, and standard Go middleware ecosystems.
- **Spec-First Contract Governance**: Single source of truth via OpenAPI 3.0, generating compile-time Go server interfaces and client SDKs without manual synchronization drift.
- **Concurrency & Memory Safety**: Zero request-context memory pooling hazards when dispatching asynchronous tasks or logging in background goroutines.
- **Predictable Error Semantics**: Universal compliance with **RFC 7807 Problem Details** for all client and server error responses.
- **Low Overhead & Sub-Millisecond Routing**: Routing overhead under 10 microseconds per request with sub-tree route grouping and declarative middleware stacking.

---

## 3. Considered Options

1. **GraphQL (Legacy Apollo / gqlgen in Go)**
2. **Fiber v3 (`gofiber/fiber`) based on `valyala/fasthttp`**
3. **Chi v5 (`go-chi/chi/v5`) based on standard `net/http` (Selected)**
4. **Go 1.22 Standard Library `http.ServeMux`**
5. **gRPC / ConnectRPC**

---

## 4. Evaluation & Comparative Matrix

| Evaluation Criteria | Chi v5 (Selected) | Fiber v3 | Go 1.22 `ServeMux` | GraphQL (`gqlgen`) | ConnectRPC / gRPC |
|---|---|---|---|---|---|
| **Underlying HTTP Engine** | Standard `net/http` | `valyala/fasthttp` | Standard `net/http` | Standard `net/http` | HTTP/2 & HTTP/1.1 |
| **Context & Concurrency Safety** | **100% Safe** (Immutable `context.Context`) | **Hazardous** (Pooled `fiber.Ctx` reused across requests) | **100% Safe** (Immutable `context.Context`) | **100% Safe** | **100% Safe** |
| **OpenAPI 3.0 Code Generation** | **First-Class** (`oapi-codegen` native `chi-server`) | Poor (Custom wrappers required) | Partial (Basic `net/http` wrapper) | None (Uses GraphQL SDL) | None (Uses Protocol Buffers) |
| **Standard Middleware Ecosystem** | **Universal** (Standard `http.Handler`) | Incompatible (Requires `adaptor` bridge) | Universal | Universal | gRPC Interceptors only |
| **Route Grouping & Middleware Scoping** | **Superior** (`r.Route` with isolated stacks) | Good (Express-style groups) | **Limited** (No per-route middleware stacks) | N/A (Single endpoint) | Package/service level |
| **Streaming File Uploads** | Native streaming `io.Reader` | Buffers payload in memory | Native streaming `io.Reader` | Fragile multipart hack | Streaming chunks via Protobuf |
| **HTTP Status & Caching** | Native HTTP semantics | Native HTTP semantics | Native HTTP semantics | Broken (Always 200 OK) | Trailer-based status |
| **Public Portfolio & Web Accessibility** | Universal (cURL, Swagger, Web UI) | Universal | Universal | Requires GraphQL Client | Requires Protobuf / Connect |

---

## 5. Decision Outcome

**Adopt RESTful API Architecture with Chi v5 (`go-chi/chi/v5`)**, enforced by a **Spec-First OpenAPI 3.0 contract** and **RFC 7807 Problem Details**.

### Rationale:
1. **Elimination of Fiber Concurrency Hazards**: Fiber relies on `valyala/fasthttp`, which recycles request contexts (`fiber.Ctx`) to achieve synthetic benchmark throughput. In financial systems where transactions trigger asynchronous background tasks, audit logging, or Goroutine dispatches, referencing a pooled context causes severe race conditions, data corruption, or panics unless explicitly cloned. Chi uses the Go standard library's `context.Context`, guaranteeing absolute thread safety.
2. **Superior OpenAPI Tooling**: `oapi-codegen` provides direct generation targets for `chi-server` and `strict-server`. This compiles `api/openapi/openapi.yaml` into strict Go interface types. If an HTTP handler signature deviates from the OpenAPI specification, compilation fails immediately.
3. **Rejection of Standard `http.ServeMux` for Routing**: While Go 1.22 enhanced `ServeMux` with pattern matching (`GET /accounts/{id}`), it lacks native support for hierarchical middleware chaining per subroute tree (e.g. applying `TenantAuth` middleware strictly to `/api/v1/*` while leaving `/healthz` and `/metrics` public). Chi provides this cleanly with zero reflection.
4. **Rejection of GraphQL**: Replaced by REST to eliminate query depth vulnerabilities, enable native browser streaming file uploads (multipart/form-data for OFX/CSV/PDF), and provide standard HTTP status caching.
5. **Rejection of gRPC**: While performant, gRPC requires binary Protobuf serialization and specialized client tooling, adding unnecessary friction for frontend browser consumption and public portfolio demonstrations.

---

## 6. API Design Principles & Contract Specifications

### 6.1 Spec-First Workflow via `oapi-codegen`

The single source of truth for the API contract is `api/openapi/openapi.yaml`. Handlers are generated via automated tooling:

```bash
oapi-codegen -config api/openapi/codegen-config.yaml api/openapi/openapi.yaml
```

Configuration (`api/openapi/codegen-config.yaml`):
```yaml
package: openapi
output: internal/adapters/inbound/http/generated.go
generate:
  chi-server: true
  strict-server: true
  types: true
  spec: true
```

The `strict-server` option generates type-safe request/response Go types, unmarshaling JSON bodies and path parameters automatically before invoking the application use cases.

### 6.2 Standardized Response Envelopes

All successful responses return JSON wrapped in a predictable top-level structure:

#### Single Resource:
```json
{
  "data": {
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "name": "Itaú Conta Corrente",
    "type": "CHECKING",
    "balance": 5420.50,
    "currency": "BRL",
    "visibility": "HOUSEHOLD_SHARED",
    "created_at": "2026-09-24T12:00:00Z"
  }
}
```

#### Paginated Collection:
```json
{
  "data": [
    {
      "id": "a1b2c3d4-e5f6-4a5b-8c9d-0e1f2a3b4c5d",
      "date": "2026-09-24",
      "amount": -142.80,
      "description": "Supermercado Pão de Ouro",
      "category": {
        "id": "f5e4d3c2-b1a0-4987-6543-210fedcba987",
        "name": "Groceries"
      },
      "is_ai_categorized": true,
      "ai_confidence": 0.94
    }
  ],
  "meta": {
    "page": 1,
    "limit": 50,
    "total_records": 1248,
    "total_pages": 25
  }
}
```

---

## 7. Error Handling Standard: RFC 7807 Problem Details

All error responses from the API strictly follow **RFC 7807 (Problem Details for HTTP APIs)** with the `Content-Type: application/problem+json` header:

```json
{
  "type": "https://api.intellifinance.app/errors/validation-failed",
  "title": "Validation Failed",
  "status": 422,
  "detail": "The transaction amount cannot be zero and must have at most 2 decimal places.",
  "instance": "/api/v1/transactions",
  "invalid_params": [
    {
      "name": "amount",
      "reason": "Value must be non-zero"
    }
  ]
}
```

### Go Implementation of RFC 7807 Problem Representation:
```go
package httpresponse

import (
	"encoding/json"
	"net/http"
)

type ProblemDetail struct {
	Type          string          `json:"type"`
	Title         string          `json:"title"`
	Status        int             `json:"status"`
	Detail        string          `json:"detail"`
	Instance      string          `json:"instance,omitempty"`
	InvalidParams []InvalidParam  `json:"invalid_params,omitempty"`
}

type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

func RenderProblem(w http.ResponseWriter, status int, prob ProblemDetail) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(prob)
}
```

---

## 8. Middleware Architecture & Routing Hierarchy

Chi routers are assembled hierarchically to enforce security and observability:

```
[ Incoming Request ]
         │
         ▼
┌───────────────────────────────────────────────┐
│ Global Middleware Chain                      │
│ - RequestID (UUIDv4 tracking)                 │
│ - RealIP (Extract client IP behind CDN/Proxy) │
│ - SlogLogger (Structured latency & status)    │
│ - Recoverer (Panic capture with stack trace)  │
│ - Timeout (30s context cancellation)          │
└──────────────────────┬────────────────────────┘
                       │
         ┌─────────────┴─────────────┐
         ▼                           ▼
┌──────────────────┐        ┌───────────────────────────────────┐
│ Public Subrouter │        │ Protected Subrouter (/api/v1/*)   │
│ - GET /healthz   │        │ - AuthMiddleware (JWT Extraction) │
│ - GET /readyz    │        │ - TenantMiddleware (RLS Context)  │
│ - GET /metrics   │        ├───────────────────────────────────┤
│ - GET /docs      │        │ Handlers:                         │
└──────────────────┘        │ - /accounts                       │
                            │ - /transactions                   │
                            │ - /categories                     │
                            │ - /ingestion                      │
                            └───────────────────────────────────┘
```

---

## 9. Core Route Registry

| Method | Path | Description | Auth Required |
|---|---|---|---|
| `GET` | `/healthz` | Liveness health check | No |
| `GET` | `/readyz` | Readiness check (DB & Redis ping) | No |
| `GET` | `/docs/openapi.yaml` | Raw OpenAPI 3.0 specification | No |
| `POST` | `/api/v1/auth/login` | Authenticate user & issue JWT | No |
| `GET` | `/api/v1/accounts` | List financial accounts for tenant | Yes |
| `POST` | `/api/v1/accounts` | Create a new financial account | Yes |
| `GET` | `/api/v1/transactions` | Query paginated, filtered transactions | Yes |
| `POST` | `/api/v1/transactions` | Create a manual transaction record | Yes |
| `PATCH`| `/api/v1/transactions/{id}/category`| Override transaction category | Yes |
| `POST` | `/api/v1/ingestion/upload` | Multipart upload (CSV/OFX/PDF) | Yes |
| `GET` | `/api/v1/ingestion/jobs/{id}` | Ingestion job status & statistics | Yes |
| `POST` | `/internal/tasks/ingest` | Cloud Tasks worker webhook (GCP Prod) | OIDC (GCP) |
| `POST` | `/internal/tasks/categorize`| Cloud Tasks batch categorize webhook | OIDC (GCP) |

---

## 10. Consequences

### Positive Consequences
- **Absolute Concurrency Safety**: Standard `net/http` context prevents dangerous memory leaks and race conditions present in fasthttp/Fiber.
- **Spec-Driven Consistency**: Strict compiler checking ensures handlers match OpenAPI contracts at all times.
- **Client & Developer Ergonomics**: Predictable RFC 7807 errors make debugging immediate and intuitive for frontend applications.
- **Effortless Observability**: Native OpenTelemetry and `log/slog` integration with zero middleware bridge overhead.

### Negative Consequences
- **Manual Spec Maintenance**: Developers must update `openapi.yaml` and run `make generate` when altering API endpoints.
- **Slightly Lower Synthetic Benchmarks than Fiber**: Microsecond routing differences are negligible in real-world workloads dominated by database and network I/O.

### Neutral Consequences
- All HTTP handlers must conform to generated strict server interfaces.

---

## 11. Implementation & Verification Plan

1. **Verify Contract File**:
   - Ensure `api/openapi/openapi.yaml` compiles cleanly with Spectral linter:
     ```bash
     npx @stoplight/spectral-cli lint api/openapi/openapi.yaml
     ```
2. **Verify Code Generation**:
   - Run `oapi-codegen` and verify generated code compiles without syntax errors:
     ```bash
     go generate ./api/openapi/...
     go test ./internal/adapters/inbound/http/...
     ```
3. **Verify RFC 7807 Compliance**:
   - Write integration tests verifying that 400, 401, 404, 422, and 500 responses return `Content-Type: application/problem+json` with all required fields.
