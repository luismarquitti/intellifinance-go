# Plano de Implementação Técnica — Frontend da Aplicação (`gh-36`)

- **Task ID**: `gh-36` (Issue #36)
- **Fase**: `Phase 5 — Public Portfolio Polish & Showcase`
- **Autor**: Archon (`01-spec-architect`) & UI Architect (`frontend-architect`)
- **Status**: Ready for Implementation
- **Companion Specs**: `SPEC-007-FRONTEND` (`.flow/tasks/gh-36/SPEC.md`), `ADR-009`

---

## 1. Visão Geral do Plano e Estratégia de Entrega

O plano de desenvolvimento do frontend foi reestruturado para resolver os apontamentos da auditoria crítica (Devil's Advocate), dividindo a execução em fases que respeitam a disponibilidade real dos endpoints do backend Go:

```
┌────────────────────────────────────────────────────────────────────────┐
│ Fase A — Decisão Arquitetural, Especificação Formal & Governança      │
│ - ADR-009 reescrito por mérito contemporâneo (sem viés do legado)     │
│ - SPEC.md no padrão EARS (38 requisitos) e contratos TypeScript       │
│ - Merge da PR #37 (Docs & Specs)                                       │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Fase B1 — Scaffolding da SPA e Telas com API Existente (MVP Core)      │
│ - Setup do workspace /web (React 19 + Vite + TypeScript + Tailwind)    │
│ - Geração de tipos automáticos via openapi-typescript                  │
│ - Cliente HTTP com injeção de TenantContext (X-Tenant-ID / Showcase)   │
│ - Tela de Contas (/accounts) conectada à API real                      │
│ - Tela de Transações (/transactions) com paginação e edição inline     │
│ - Suporte a embed.FS no Chi router (/assets/* e SPA fallback /*)       │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Fase B2 — Telas Dependentes de Extensões do Backend                    │
│ - Integração com novos endpoints de Auth JWT (/auth/login e refresh)   │
│ - Integração com endpoints de Analytics (/analytics/cash-flow)        │
│ - Integração com upload multipart e polling de Ingestão (/ingestion)  │
│ - Testes E2E de ponta a ponta Playwright em container distroless       │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Fase A — Especificação e Refinamento de Governança (Concluída nesta Task)

### Tarefas da Fase A
- [x] **A.1**: Mapeamento do inventário real de endpoints no backend Go (`router.go`, handlers e DTOs).
- [x] **A.2**: Avaliação comparativa de stack por mérito puro em `STACK_EVALUATION.md` (6 opções ponderadas).
- [x] **A.3**: Elaboração da especificação formal `SPEC.md` no padrão EARS com 38 requisitos.
- [x] **A.4**: Auditoria crítica adversarial `DEVILS_ADVOCATE.md` e revisão de segurança `SECURITY_REVIEW.md`.
- [x] **A.5**: Veredito do comitê em `REVIEW.md` e reescrita de `ADR-009` e `docs/specs/frontend.md`.
- [x] **A.6**: Preparação do corpo amadurecido da Issue #36 em `ISSUE_UPDATE.md`.

---

## 3. Fase B1 — Scaffolding & MVP Core (Consumindo Endpoints Prontos)

### Tarefa B1.1: Inicialização do Workspace e Toolchain (`/web`)
- **Ações**:
  1. Inicializar projeto Vite no diretório `/web` utilizando template `react-ts`.
  2. Configurar dependências fixas: `react@19`, `react-dom@19`, `@tanstack/react-query@5`, `tailwindcss@3.4` (ou Tailwind v4), `clsx`, `tailwind-merge`, `lucide-react`.
  3. Configurar TypeScript com `strict: true`, `noImplicitAny: true` e path aliases (`@/*` -> `./src/*`).
  4. Adicionar script de automação de tipos OpenAPI: `npm run gen:api` executando `openapi-typescript ../api/openapi/openapi.yaml -o ./src/api/generated/schema.d.ts`.
- **Verificação**:
  ```bash
  cd web && npm ci && npm run build
  test -f dist/index.html
  ```

### Tarefa B1.2: Cliente HTTP e Camada de Comunicação com Backend
- **Ações**:
  1. Implementar `web/src/api/client.ts` com `openapi-fetch` tipado com o schema gerado.
  2. Implementar `web/src/api/auth-interceptor.ts` suportando:
     - Injeção de Bearer Token em memória quando disponível.
     - Injeção automática de header `X-Tenant-ID` em modo Showcase/Desenvolvimento local (`VITE_SHOWCASE_MODE=true`).
     - Mutex de Silent Refresh para prevenir condição de corrida em 401 concorrentes.
  3. Implementar `web/src/api/problem.ts` para deserialização padronizada de RFC 7807 (`application/problem+json`).
- **Verificação**:
  ```bash
  npm test src/api/problem.test.ts
  ```

### Tarefa B1.3: Sistema de Design e Layout Base
- **Ações**:
  1. Configurar design tokens no Tailwind (`primary: #0D47A1`, superfícies, cores de sucesso/erro).
  2. Desenvolver componentes base acessíveis: `Button`, `Input`, `Dialog` (Modal), `Badge`, `Skeleton`.
  3. Criar layout padrão com Navbar lateral responsiva, cabeçalho e seletor de tenant em modo showcase.
- **Verificação**:
  - Testes de renderização com Vitest e React Testing Library.

### Tarefa B1.4: Tela de Contas (`/accounts`)
- **Ações**:
  1. Hook `useAccounts()` consumindo `GET /api/v1/accounts`.
  2. Renderização de grade de cartões com saldos formatados em BRL e badges de visibilidade.
  3. Formulário em modal consumindo `POST /api/v1/accounts`, validando campos obrigatórios e rejeitando floats.
- **Verificação**:
  - Criação de conta reflete imediatamente na listagem via invalidação de cache do TanStack Query.

### Tarefa B1.5: Tela de Transações (`/transactions`)
- **Ações**:
  1. Hook `useTransactions(filters)` consumindo `GET /api/v1/transactions` com envelope `{ items, pagination }`.
  2. Tabela paginada com colunas Data, Descrição, Categoria, Valor e Indicador de IA.
  3. Dropdown de categoria inline com mutação otimista via `PATCH /api/v1/transactions/{id}` e reversão em caso de erro RFC 7807.
  4. Sincronização de filtros com a URL (query parameters).
- **Verificação**:
  - Navegação entre páginas sem recarregar a tela; persistência de filtros na URL.

### Tarefa B1.6: Embutimento dos Ativos Estáticos no Binário Go (`embed.FS`)
- **Ações**:
  1. Criar pacote `internal/adapters/inbound/http/assets/assets.go` com `//go:embed dist/*`.
  2. Registrar rota estática e fallback no Chi router (`internal/adapters/inbound/http/router.go`):
     - `/assets/*` servindo arquivos estáticos com cache longo.
     - `/*` servindo `index.html` para deep links da SPA.
  3. Atualizar `Makefile` para encadear `build-web` antes de `build-api`.
  4. Criar teste Go unitário `assets_test.go` verificando que o filesystem embutido não está vazio.
- **Verificação**:
  ```bash
  make build
  ./bin/api &
  curl -fsSI localhost:8080/transactions | grep -i "content-type: text/html"
  killall api
  ```

---

## 4. Fase B2 — Expansões e Telas Avançadas (Após Conclusão dos Endpoints de Backend)

### Tarefa B2.1: Autenticação JWT Completa
- Integrar formulário de `/login` com `POST /api/v1/auth/login`.
- Ativar ciclo de Silent Refresh com cookies httpOnly (`POST /api/v1/auth/refresh`).
- Desativar seletor manual de tenant quando autenticado via JWT.

### Tarefa B2.2: Dashboard Analítico Completo (`/dashboard`)
- Integrar com `GET /api/v1/analytics/cash-flow` e `GET /api/v1/analytics/by-category`.
- Renderizar gráficos de distribuição e indicadores de taxa de poupança calculados pelo servidor.

### Tarefa B2.3: Ingestão de Extratos com Polling Assíncrono (`/ingestion`)
- Implementar dropzone com validação prévia de tamanho (<25MB) e extensões (`.ofx`, `.csv`, `.xlsx`, `.pdf`).
- Disparar upload multipart para `POST /api/v1/ingestion/upload`.
- Polling com backoff para `GET /api/v1/ingestion/jobs/{id}` e exibição do relatório de deduplicação.

### Tarefa B2.4: Suíte de Testes E2E (Playwright)
- Cenários ponta a ponta navegando pelas telas contra o backend local `make dev-up`.
- Auditoria de acessibilidade automatizada (`axe-core`) com zero violações WCAG AA.
- Validação de isolamento de tenant na interface do usuário.

---

## 5. Critérios de Conclusão e Gates de Qualidade

Para a conclusão definitiva do frontend, os seguintes comandos devem executar com exit code 0:

```bash
# 1. Build determinístico do frontend
npm --prefix web ci && npm --prefix web run build
test -f web/dist/index.html || exit 1

# 2. Verificação anti-drift de contrato
npm --prefix web run gen:api && git diff --exit-code web/src/api/generated

# 3. Blindagem de segurança (zero tokens em storage, zero floats em moeda)
! grep -rE "localStorage|sessionStorage" web/src
! grep -rE "parseFloat|Number\(.*amount" web/src

# 4. Linting e testes do frontend
npm --prefix web run lint
npm --prefix web run test

# 5. Build do binário unificado Go com assets embutidos
go build -v ./...
go test -v -race ./...
golangci-lint run
```
