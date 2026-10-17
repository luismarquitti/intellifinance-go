# Especificação Técnica Formal — Frontend da Aplicação (SPA REST)

- **Task ID**: `gh-36` (Issue #36)
- **Document ID**: SPEC-007-FRONTEND
- **Fase**: `Phase 5 — Public Portfolio Polish & Showcase`
- **Arquiteto Responsável**: Archon (`01-spec-architect`) & UI/Frontend Architect (`frontend-architect`)
- **Status**: Ready for Implementation (Após Refinamento e Revisão Adversarial)
- **Stack Aprovada**: React 19 + Vite 6/7 + TypeScript + TanStack Query v5 + Tailwind CSS
- **Modo de Entrega**: Ativos estáticos compilados embutidos no binário Go via `embed.FS` (`cmd/api`)

---

## 1. Visão Geral, Objetivo e Invariantes Arquiteturais

Esta especificação define a arquitetura, telas, contratos de dados, tratamento de erros e requisitos determinísticos no padrão EARS para a interface de usuário (Frontend SPA) do **IntelliFinance**.

O frontend tem como missão principal viabilizar a experiência do **Interactive Portfolio Showcase** (`PRD §6.3`), permitindo que avaliadores, recrutadores e usuários operem o sistema localmente (`make dev-up`), visualizando saldos, importando extratos, acompanhando a categorização assistida por IA com revisão humana (`PRD §4.5`) e gerando relatórios de fluxo de caixa em tempo real.

### 1.1 Invariantes Inegociáveis (Red Lines)

1. **Invariante de Credenciais em Memória (`Token In-Memory Only`)**:
   - O Access Token (JWT, TTL ≤ 1h) DEVE ser mantido **estritamente em memória volátil** (closure/módulo JavaScript).
   - É **terminantemente proibido** persistir access tokens ou chaves em `localStorage`, `sessionStorage`, IndexedDB ou cookies acessíveis por JavaScript (`document.cookie`).
   - O Refresh Token DEVE ser gerenciado exclusivamente pelo backend via cookie HTTP-Only, `Secure`, `SameSite=Strict` (ou `Lax` em desenvolvimento local).

2. **Invariante Contábil Zero Aritmética no Cliente (`Zero Client-Side Math`)**:
   - É **terminantemente proibido** utilizar `float32`, `float64`, `number` ou funções como `parseFloat()` / `Number()` para cálculos, subtotais ou somatórios de valores monetários no cliente.
   - Toda agregação matemática (saldos consolidados, totais por categoria, burn rate, savings rate) DEVE ser calculada exclusivamente pelo servidor (`PRD §4.7` e `FR-7.4`).
   - Valores monetários chegam como strings decimais canônicas (ex: `"1450.50"`) e devem ser renderizados diretamente via `Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })`. Caso alguma manipulação de centavos seja estritamente indispensável na UI, DEVE utilizar `decimal.js` ou inteiros de centavos, nunca ponto flutuante IEEE 754.

3. **Invariante de Isolamento e Não-Inferência de Tenant (`Zero Client Visibility Inference`)**:
   - O frontend renderiza estritamente os dados devolvidos pela API.
   - O frontend **NUNCA DEVE** tentar inferir regras de visibilidade (`SHARED`, `PERSONAL_PRIVATE`, `RESTRICTED_BUSINESS`) ou aplicar filtros client-side como substituto de controles de privacidade de inquilino. A aplicação de segurança é de responsabilidade soberana das políticas de Row-Level Security (RLS) do PostgreSQL e do middleware do backend.

4. **Invariante Zero Vazamento de PII (`Zero Real PII Policy`)**:
   - Nenhuma informação bancária real, nome completo, CPF real ou holerite verídico DEVE ser renderizado ou incluído em fixtures, capturas de tela ou testes do frontend (`ADR-008`).
   - Todos os dados simulados e de demonstração DEVEM provir de `cmd/fixturegen` ou geradores estáticos com CPFs matematicamente válidos sob a faixa de teste `000.xxx.xxx-xx`.

5. **Invariante de Empacotamento em Binário Único (`Single-Binary Embedding`)**:
   - O frontend DEVE ser compilado como artefato puramente estático (`web/dist/`) e embutido no binário Go através do pacote padrão `embed.FS` (`internal/adapters/inbound/http/assets`).
   - Não deve existir nenhum servidor Node.js/Bun em produção ou runtime adicional, mantendo a imagem Docker distroless abaixo de 35MB (`NFR-REL-03`).

---

## 2. Inventário Real da API: Endpoints Existentes vs. Lacunas

Antes de definir os contratos do frontend, auditou-se o código-fonte real do backend (`internal/adapters/inbound/http/router.go` e handlers).

### 2.1 Endpoints Existentes e Funcionais no Backend

| Método | Endpoint Real | Handler Implementado | DTOs de Entrada / Saída |
|:---|:---|:---|:---|
| `GET` | `/healthz` | `router.go:41` (anônimo) | Saída: `{"status":"UP"}` |
| `GET` | `/readyz` | `router.go:47` (anônimo) | Saída: `{"status":"READY"}` |
| `POST` | `/api/v1/accounts` | `handlers.AccountHandler.CreateAccount` (`account_handler.go:31`) | `dto.CreateAccountRequest` / `dto.AccountResponse` |
| `GET` | `/api/v1/accounts` | `handlers.AccountHandler.ListAccounts` (`account_handler.go:95`) | `dto.AccountListResponse` (`{"items":[...]}`) |
| `GET` | `/api/v1/accounts/{id}` | `handlers.AccountHandler.GetAccount` (`account_handler.go:144`) | `dto.AccountResponse` |
| `PATCH`| `/api/v1/accounts/{id}` | `handlers.AccountHandler.UpdateAccount` (`account_handler.go:178`) | `dto.UpdateAccountRequest` / `dto.AccountResponse` |
| `GET` | `/api/v1/categories` | `handlers.CategoryHandler.ListCategories` (`category_handler.go:30`)| `dto.CategoryListResponse` (árvore hierárquica) |
| `POST` | `/api/v1/categories` | `handlers.CategoryHandler.CreateCategory` (`category_handler.go:79`)| `dto.CreateCategoryRequest` / `dto.CategoryResponse` |
| `PATCH`| `/api/v1/categories/{id}`| `handlers.CategoryHandler.UpdateCategory` (`category_handler.go:134`)| `dto.UpdateCategoryRequest` / `dto.CategoryResponse` |
| `POST` | `/api/v1/transactions` | `handlers.TransactionHandler.CreateTransaction` (`transaction_handler.go:32`)| `dto.CreateTransactionRequest` / `dto.TransactionResponse` |
| `GET` | `/api/v1/transactions` | `handlers.TransactionHandler.ListTransactions` (`transaction_handler.go:109`)| `dto.TransactionListResponse` (Envelope Paginado) |
| `GET` | `/api/v1/transactions/{id}`| `handlers.TransactionHandler.GetTransaction` (`transaction_handler.go:231`)| `dto.TransactionResponse` |
| `PATCH`| `/api/v1/transactions/{id}`| `handlers.TransactionHandler.UpdateTransaction` (`transaction_handler.go:265`)| `dto.UpdateTransactionRequest` / `dto.TransactionResponse` |
| `DELETE`|`/api/v1/transactions/{id}`| `handlers.TransactionHandler.DeleteTransaction` (`transaction_handler.go:356`)| Retorna `204 No Content` |

*Nota de Autenticação Atual*: No código atual, todas as rotas protegidas utilizam `middleware.TenantContextMiddleware` (`internal/adapters/inbound/http/middleware/tenant.go:55`), que valida a presença e formato do header HTTP `X-Tenant-ID` (UUID) e opcionalmente `X-User-ID`.

### 2.2 Lacunas de Backend (Endpoints Faltantes / Pré-requisitos para Telas)

| ID Lacuna | Rota Necessária | Finalidade na UI | Estado Atual no Backend | Ação de Mitigação no Frontend |
|:---|:---|:---|:---|:---|
| **GAP-01** | `POST /api/v1/auth/login` | Tela de Login | **Inexistente** | Fase B1 suporta modo Dev/Showcase injetando `X-Tenant-ID` via seletor de tenant local; Fase B2 integra com o handler JWT quando concluído. |
| **GAP-02** | `POST /api/v1/auth/refresh` | Renovação Silenciosa de Sessão | **Inexistente** | Bloqueante para JWT; substituído por sessão direta em modo dev/showcase. |
| **GAP-03** | `POST /api/v1/auth/logout` | Encerramento de Sessão | **Inexistente** | Limpeza de estado em memória no cliente. |
| **GAP-04** | `GET /api/v1/auth/me` | Dados do Usuário & Tenant Ativo | **Inexistente** | Em modo showcase, deduz do perfil selecionado localmente. |
| **GAP-05** | `GET /api/v1/analytics/cash-flow` | Dashboard: Total Receitas, Despesas, Saldo, Taxa de Poupança | **Inexistente** | O frontend não fará soma com float. O Dashboard exibirá estado "Métricas em processamento" ou consumirá endpoint stub de agregação segura. |
| **GAP-06** | `GET /api/v1/analytics/by-category` | Dashboard: Distribuição por Categoria | **Inexistente** | Aguarda implementação do endpoint de agregação no backend. |
| **GAP-07** | `POST /api/v1/ingestion/upload` | Tela de Ingestão: Upload multipart de extratos | **Inexistente no router** (Domain existe) | Aguarda conexão do router com o `IngestionHandler`. |
| **GAP-08** | `GET /api/v1/ingestion/jobs/{id}` | Tela de Ingestão: Polling de progresso | **Inexistente no router** | Aguarda conexão do router com o `IngestionHandler`. |
| **GAP-09** | `PATCH /api/v1/transactions/{id}/category` | Fila de Revisão de IA | **Parcial**: existe `PATCH /api/v1/transactions/{id}` que aceita `category_id` no payload. | O frontend consumirá `PATCH /api/v1/transactions/{id}` enviando `{ "category_id": "..." }`. |

---

## 3. Requisitos de Frontend no Padrão EARS

Os requisitos formais abaixo cobrem todo o comportamento determinístico da SPA:

### 3.1 Autenticação, Sessão e Tenant Context
- **REQ-FE-01**: **Quando** o usuário submete o formulário de login com e-mail e senha válidos, **se** a resposta for `200 OK`, o sistema **DEVE** armazenar o Access Token exclusivamente em memória volátil e redirecionar para `/dashboard`.
- **REQ-FE-02**: **Quando** o usuário submete credenciais inválidas, **se** a API retornar `401 Unauthorized` com envelope RFC 7807, o sistema **DEVE** exibir o campo `detail` na interface e manter o foco no campo de senha.
- **REQ-FE-03**: **Quando** a aplicação é carregada pela primeira vez no navegador (App Boot), **se** houver cookie de refresh presente, o sistema **DEVE** disparar silenciosamente uma requisição `POST /api/v1/auth/refresh` antes de renderizar rotas protegidas.
- **REQ-FE-04**: **Quando** a tentativa de refresh silencioso no boot falha com status 401 ou erro de rede, o sistema **DEVE** redirecionar o usuário para a rota `/login`.
- **REQ-FE-05**: **Quando** qualquer requisição autenticada receber um status `401 Unauthorized`, o sistema **DEVE** enfileirar as requisições concorrentes e disparar uma única tentativa de renovação via `/auth/refresh`.
- **REQ-FE-06**: **Quando** a tentativa de renovação subsequente ao 401 falhar, o sistema **DEVE** limpar imediatamente o token da memória e redirecionar o usuário para `/login`.
- **REQ-FE-07**: **Quando** o usuário clica em "Sair" (Logout), o sistema **DEVE** invocar `POST /api/v1/auth/logout`, purgar o cache do TanStack Query, limpar variáveis em memória e navegar para `/login`.
- **REQ-FE-08**: **Quando** o frontend estiver executando em modo de Desenvolvimento / Showcase local (`VITE_SHOWCASE_MODE=true`), **se** o backend não possuir módulo de autenticação JWT ativo, o sistema **DEVE** permitir a seleção de perfil pré-configurado (Tenant Luis/Eluma ou Tenant Convidado) e injetar automaticamente o header `X-Tenant-ID` correspondente em todas as requisições.

### 3.2 Navegação e Roteamento
- **REQ-FE-09**: **Quando** um usuário desautenticado tenta acessar uma rota protegida (`/dashboard`, `/transactions`, `/accounts`, `/ingestion`), o sistema **DEVE** interceptar a navegação e redirecionar para `/login`, preservando a rota pretendida como parâmetro `redirect`.
- **REQ-FE-10**: **Quando** um usuário autenticado navega para a rota raiz `/`, o sistema **DEVE** redirecionar automaticamente para `/dashboard`.
- **REQ-FE-11**: **Quando** o usuário acessa uma URL não mapeada no roteador, o sistema **DEVE** renderizar uma tela de 404 (Not Found) amigável com botão de retorno ao Dashboard.
- **REQ-FE-12**: **Quando** o usuário aplica filtros na tela de transações (conta, categoria, período), o sistema **DEVE** sincronizar o estado com os query parameters da URL para viabilizar compartilhamento e persistência de histórico no navegador.

### 3.3 Dashboard e Painel Analítico
- **REQ-FE-13**: **Quando** a rota `/dashboard` é acessada, o sistema **DEVE** disparar em paralelo as consultas de fluxo de caixa, distribuição por categoria e lista de contas ativas.
- **REQ-FE-14**: **Quando** as consultas do dashboard estão em andamento, o sistema **DEVE** exibir skeletons com layout idêntico aos cartões e gráficos finais, evitando Cumulative Layout Shift (CLS).
- **REQ-FE-15**: **Quando** os dados de fluxo de caixa forem carregados com sucesso, o sistema **DEVE** renderizar receita, despesas, saldo líquido e taxa de poupança formatados em padrão monetário `pt-BR` (BRL).
- **REQ-FE-16**: **Quando** o período selecionado não contiver movimentações financeiras, o sistema **DEVE** exibir uma mensagem informativa de estado vazio com botão direcionando para `/ingestion`.
- **REQ-FE-17**: **Quando** o usuário clica em um cartão de conta no dashboard, o sistema **DEVE** navegar para a tela `/transactions` pré-filtrando pela conta selecionada.

### 3.4 Transações e Fila de Revisão de IA
- **REQ-FE-18**: **Quando** a rota `/transactions` é renderizada, o sistema **DEVE** exibir a tabela paginada consumindo o envelope `items` e os metadados de `pagination` (`page`, `limit`, `total`, `has_next`, `has_prev`).
- **REQ-FE-19**: **Quando** a transação possuir `is_ai_categorized = true` e `ai_confidence < 0.85`, o sistema **DEVE** exibir uma badge destacada de aviso indicando necessidade de revisão humana.
- **REQ-FE-20**: **Quando** o usuário clica na categoria de uma transação ou seleciona uma nova categoria no seletor inline, o sistema **DEVE** emitir imediatamente `PATCH /api/v1/transactions/{id}` com atualização otimista na tabela.
- **REQ-FE-21**: **Quando** a atualização otimista de categoria falhar com status 422 ou 500, o sistema **DEVE** reverter a linha para a categoria anterior e apresentar toast de erro estruturado contendo o campo `detail` do RFC 7807 retornado pela API.
- **REQ-FE-22**: **Quando** a lista de transações for filtrada e não houver registros correspondentes, o sistema **DEVE** exibir mensagem de "Nenhum resultado encontrado" com ação rápida para "Limpar filtros".
- **REQ-FE-23**: **Quando** o usuário altera a página ou a quantidade de itens por página, o sistema **DEVE** rolar o scroll suavemente para o topo da tabela e atualizar os dados via TanStack Query.

### 3.5 Gestão de Contas Financeiras
- **REQ-FE-24**: **Quando** a rota `/accounts` é acessada, o sistema **DEVE** renderizar a listagem de contas agrupadas por tipo (`CHECKING`, `CREDIT_CARD`, `INVESTMENT`, `CASH`, `BUSINESS`).
- **REQ-FE-25**: **Quando** o usuário abre o modal de criação de conta e preenche os campos obrigatórios (`name`, `type`, `currency`, `visibility`), o sistema **DEVE** validar os campos no cliente antes do envio.
- **REQ-FE-26**: **Quando** o usuário seleciona visibilidade `PERSONAL_PRIVATE`, o sistema **DEVE** exibir um aviso contextual explicando que a conta e seus lançamentos não serão visíveis para outros membros do tenant familiar.
- **REQ-FE-27**: **Quando** o formulário de conta é submetido com saldo inicial, o sistema **DEVE** transmitir o valor como string decimal formatada com ponto (ex: `"1500.50"`), proibindo terminantemente conversões com `parseFloat()`.

### 3.6 Ingestão de Extratos e Acompanhamento Assíncrono
- **REQ-FE-28**: **Quando** o usuário arrasta um arquivo para a dropzone de `/ingestion`, o sistema **DEVE** validar imediatamente no navegador a extensão (`.ofx`, `.csv`, `.xlsx`, `.pdf`) e o limite de tamanho (máximo 25MB conforme `NFR-PERF-04`).
- **REQ-FE-29**: **Quando** um arquivo inválido ou acima de 25MB for selecionado, o sistema **DEVE** rejeitar o upload instantaneamente com mensagem visual sem disparar tráfego de rede.
- **REQ-FE-30**: **Quando** o upload multipart for aceito com `202 Accepted` contendo `job_id`, o sistema **DEVE** iniciar polling automático a cada 2s com backoff até teto de 10s consultando o status do job.
- **REQ-FE-31**: **Quando** o job de ingestão atinge status `COMPLETED`, o sistema **DEVE** interromper o polling e renderizar o relatório final contendo `inserted_count`, `duplicate_count` e `error_count`.
- **REQ-FE-32**: **Quando** o polling exceder 5 minutos sem conclusão, o sistema **DEVE** alertar o usuário de que o processamento continua em segundo plano no servidor e fornecer link para a tela de transações.

### 3.7 Tratamento Global de Erros e Resiliência
- **REQ-FE-33**: **Quando** o backend retornar erro com `Content-Type: application/problem+json` (RFC 7807), o sistema **DEVE** extrair os campos `title`, `detail` e `invalid_params`, mapeando inconsistências diretamente nos inputs do formulário correspondente.
- **REQ-FE-34**: **Quando** ocorrer falha de conectividade ou queda de rede, o sistema **DEVE** pausar as requisições em background do TanStack Query e exibir banner discreto de "Sem conexão com a internet", retomando automaticamente assim que o evento `online` for detectado.
- **REQ-FE-35**: **Quando** uma exceção não tratada for capturada por um React Error Boundary, o sistema **DEVE** renderizar uma tela amigável de recuperação ("Ocorreu um erro inesperado") com botão para recarregar a página, sem jamais exibir stack traces ou detalhes técnicos de depuração ao usuário final.

### 3.8 Acessibilidade, Internacionalização e Design System
- **REQ-FE-36**: **Quando** qualquer elemento interativo receber foco via navegação por teclado (Tab/Shift+Tab), o sistema **DEVE** exibir anel de foco de alto contraste visível (`focus-visible:ring-2`).
- **REQ-FE-37**: **Quando** qualquer valor monetário for exibido na tela, o sistema **DEVE** formatá-lo segundo o padrão monetário brasileiro `pt-BR` (ex: `R$ 1.250,50` com sinal negativo `-R$ 1.250,50` destacado em vermelho).
- **REQ-FE-38**: **Quando** o tema escuro estiver ativo no sistema operacional (`prefers-color-scheme: dark`) ou for selecionado manualmente, o sistema **DEVE** alternar as classes Tailwind para garantir proporção de contraste mínima de 4.5:1 (WCAG AA).

---

## 4. Arquitetura do Frontend e Fluxo de Dados

### 4.1 Diagrama de Camadas da SPA

```
┌────────────────────────────────────────────────────────────────────────┐
│                        Camada de Apresentação (UI)                     │
│  - Páginas / Rotas (/dashboard, /transactions, /accounts, /ingestion)   │
│  - Componentes Compartilhados (Button, Input, Modal, Table, Badge)     │
│  - Primitives Acessíveis Headless (Radix UI)                           │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ (Invoca Hooks de Domínio)
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                     Camada de Estado de Servidor e UI                  │
│  - TanStack Query v5 (Cache, Polling, Invalidação, Mutações Otimistas) │
│  - Zustand (Estado Global Efêmero: Sidebar, Modal Ativo, Filtros)      │
│  - Session Store em Memória (Token JWT, Tenant ID ativo, Profile)      │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ (Executa Chamadas Tipadas)
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                 Camada de Integração de API (Anti-Drift)               │
│  - Cliente HTTP Tipado (`openapi-fetch` sobre `api/openapi/openapi.yaml`)│
│  - Interceptor HTTP: Injeção de Bearer Token / X-Tenant-ID             │
│  - Interceptor de Resposta: Parser RFC 7807 (Problem Details)          │
│  - Mutex de Silent Refresh com enfileiramento de requisições 401       │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ (Requisições HTTP REST)
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                 Backend Go Chi v5 (Binário Único `cmd/api`)            │
│  - Servidor de Ativos Estáticos (`embed.FS` em `/assets/*` e `/*`)      │
│  - Endpoints REST `/api/v1/*` com PostgreSQL RLS e RFC 7807             │
└────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Estrutura de Diretórios Padronizada (`web/`)

```
web/
├── index.html                   # Entry point HTML com fontes e tags meta
├── package.json                 # Dependências estritas e scripts de build
├── tsconfig.json                # Configuração TypeScript estrita (strict: true)
├── vite.config.ts               # Bundler Vite configurado para saída dist/
├── tailwind.config.ts           # Design tokens e variáveis de tema
└── src/
    ├── main.tsx                 # Bootstrap do React 19 e montagem no DOM
    ├── App.tsx                  # Provedores globais (QueryClient, Theme, Toast)
    ├── router/                  # Roteamento SPA client-side e Route Guards
    │   ├── routes.tsx
    │   └── ProtectedRoute.tsx
    ├── api/                     # Camada de integração com o Backend
    │   ├── generated/           # Código TypeScript gerado via openapi-typescript
    │   │   └── schema.d.ts
    │   ├── client.ts            # openapi-fetch configurado com interceptors
    │   ├── problem.ts           # Tipos e utilitários para RFC 7807 Problem Details
    │   └── auth-interceptor.ts  # Gerenciador de token em memória e silent refresh
    ├── components/              # Componentes de UI reutilizáveis
    │   ├── ui/                  # Primitives (Button, Dialog, Dropdown, Table, Input)
    │   ├── layout/              # Navbar, Sidebar, PageContainer, TenantSelector
    │   └── feedback/            # Skeleton, Toast, ErrorBoundary, EmptyState
    ├── features/                # Módulos de domínio da aplicação
    │   ├── auth/                # Login form, hooks de sessão
    │   ├── dashboard/           # Cartões de cash flow, gráficos de categoria
    │   ├── transactions/        # Tabela paginada, filtros, modal de revisão IA
    │   ├── accounts/            # Grid de contas, modal de cadastro
    │   └── ingestion/           # Dropzone, upload com progresso, job poller
    ├── lib/                     # Utilitários puros
    │   ├── currency.ts          # Formatador BRL seguro (Intl.NumberFormat)
    │   ├── date.ts              # Formatadores de data pt-BR
    │   └── cn.ts                # Merge de classes Tailwind (clsx + twMerge)
    └── store/                   # Estado global client-side (Zustand)
        └── useUIStore.ts
```

---

## 5. Contratos de Dados TypeScript

### 5.1 RFC 7807 Problem Details

```typescript
export interface InvalidParam {
  name: string;
  reason: string;
}

export interface ProblemDetail {
  type: string;
  title: string;
  status: number;
  detail: string;
  instance?: string;
  code?: string;
  timestamp?: string;
  invalid_params?: InvalidParam[];
}
```

### 5.2 Envelope Paginado

```typescript
export interface PaginationMeta {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
}

export interface PaginatedResponse<T> {
  items: T[];
  pagination: PaginationMeta;
}
```

### 5.3 Contrato de Conta Financeira

```typescript
export type AccountType = 'CHECKING' | 'CREDIT_CARD' | 'SAVINGS' | 'INVESTMENT' | 'CASH' | 'BUSINESS';
export type AccountVisibility = 'HOUSEHOLD_SHARED' | 'PERSONAL_PRIVATE' | 'RESTRICTED_BUSINESS';

export interface AccountResponse {
  id: string;
  tenant_id: string;
  owner_user_id: string;
  name: string;
  type: AccountType;
  currency: string;
  balance: string; // String decimal (ex: "14250.75"), nunca number
  visibility: AccountVisibility;
  institution_id?: string;
  institution_name?: string;
  account_number_masked?: string;
  color_hex?: string;
  icon_name?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateAccountRequest {
  name: string;
  type: AccountType;
  currency: string;
  initial_balance?: string; // String decimal
  visibility: AccountVisibility;
  institution_id?: string;
  institution_name?: string;
  account_number_masked?: string;
  color_hex?: string;
  icon_name?: string;
}
```

### 5.4 Contrato de Transação Financeira

```typescript
export type TransactionType = 'INCOME' | 'EXPENSE' | 'TRANSFER';
export type TransactionStatus = 'PENDING' | 'COMPLETED' | 'RECONCILED' | 'VOID';

export interface TransactionResponse {
  id: string;
  tenant_id: string;
  account_id: string;
  category_id?: string;
  created_by_user_id?: string;
  date: string; // YYYY-MM-DD
  amount: string; // String decimal com sinal (ex: "-142.80")
  description: string;
  clean_description?: string;
  type: TransactionType;
  status: TransactionStatus;
  visibility: string;
  source: string;
  external_id?: string;
  fingerprint: string;
  is_ai_categorized: boolean;
  ai_confidence?: number;
  is_manually_verified: boolean;
  notes?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface UpdateTransactionRequest {
  account_id?: string;
  category_id?: string;
  date?: string;
  amount?: string;
  description?: string;
  type?: TransactionType;
  status?: TransactionStatus;
  visibility?: string;
  notes?: string;
  metadata?: Record<string, unknown>;
}
```

### 5.5 Contrato de Sessão de Usuário

```typescript
export interface UserSession {
  accessToken: string | null;
  user: {
    id: string;
    email: string;
    fullName: string;
    tenantId: string;
    role: string;
  } | null;
  activeTenantId: string;
}
```

---

## 6. Especificação das Telas e Estados Determinísticos

### 6.1 Tela de Login (`/login`)
- **Dados**: `POST /api/v1/auth/login` (credenciais). Em modo Showcase: seletor de tenant local pré-configurado.
- **Estados**:
  - `Idle`: Formulário habilitado com campos de e-mail e senha.
  - `Submitting`: Botão de envio com spinner; inputs desabilitados.
  - `Error (401)`: Exibe banner vermelho com o `detail` retornado pela API; limpa o campo de senha.
  - `Error (422)`: Destaca campos com erro inline baseando-se em `invalid_params`.
  - `Network Error`: Exibe alerta de falha de conexão com botão de repetição.
- **Critérios de Aceite**: Foco inicial no campo de e-mail; envio com tecla Enter; nenhuma credencial gravada em storage.

### 6.2 Tela de Dashboard (`/dashboard`)
- **Dados**: `GET /api/v1/analytics/cash-flow`, `GET /api/v1/analytics/by-category`, `GET /api/v1/accounts`.
- **Estados**:
  - `Loading`: Skeletons animados ocupando exatamente a área dos 4 cartões de métricas, do gráfico de pizza/rosca e da lista de contas.
  - `Empty`: Quando o saldo total e as movimentações forem zero, exibe ilustração agradável e botão direcionando para `/ingestion`.
  - `Success`: 4 cartões KPI (Receita, Despesa, Saldo Líquido, Taxa de Poupança), gráfico de distribuição de despesas por categoria e lista de contas.
  - `Error`: Cartão de alerta com botão "Tentar Novamente".
- **Critérios de Aceite**: Valores monetários em BRL (`R$`); cores semânticas (verde para superávit, vermelho para déficit); clique em conta redireciona para `/transactions?account_id={id}`.

### 6.3 Tela de Transações (`/transactions`)
- **Dados**: `GET /api/v1/transactions` com paginação e filtros (`account_id`, `category_id`, `from`, `to`, `type`, `needs_review`).
- **Estados**:
  - `Loading`: Skeleton mantendo o cabeçalho da tabela fixo.
  - `Empty`: Mensagem de ausência de lançamentos com atalho para importação.
  - `Success`: Tabela com colunas: Data, Descrição, Conta, Categoria, Valor, Status/IA, Ações.
  - `Optimistic Updating`: Ao trocar a categoria via dropdown inline, a célula atualiza imediatamente; se a requisição falhar, reverte suavemente e exibe toast com o erro.
- **Critérios de Aceite**: Paginação funcional com navegação anterior/próxima; badges destacando transações de IA que requerem revisão humana (`needs_review`); query string da URL reflete exatamente os filtros aplicados.

### 6.4 Tela de Contas (`/accounts`)
- **Dados**: `GET /api/v1/accounts`, `POST /api/v1/accounts`, `PATCH /api/v1/accounts/{id}`.
- **Estados**:
  - `Loading`: Grade de cartões de conta com placeholders pulsantes.
  - `Success`: Grade de cartões agrupados por tipo, com saldo, nome da instituição e badge de visibilidade (`Compartilhada` vs `Privada`).
  - `Modal Form`: Modal acessível para nova conta com validação estrita.
- **Critérios de Aceite**: Validação de saldo sem float; aviso claro ao selecionar `PERSONAL_PRIVATE`; fechamento do modal com tecla Esc.

### 6.5 Tela de Ingestão de Extratos (`/ingestion`)
- **Dados**: `POST /api/v1/ingestion/upload` (multipart) e `GET /api/v1/ingestion/jobs/{id}` (polling).
- **Estados**:
  - `Dropzone Idle`: Área pontilhada para arrastar arquivos com lista de extensões permitidas (`.ofx`, `.csv`, `.xlsx`, `.pdf`).
  - `File Selected`: Exibe nome do arquivo, tamanho legível (KB/MB) e seletor de conta de destino obrigatório.
  - `Uploading`: Barra de progresso percentual real via `onUploadProgress` do fetcher/XHR.
  - `Processing (Polling)`: Spinner com mensagem de processamento e cronômetro de tempo decorrido.
  - `Completed`: Cartão verde de sucesso com contadores: Inseridas (`inserted_count`), Duplicadas ignoradas (`duplicate_count`) e Falhas (`error_count`), acompanhado de botão para "Ver transações importadas".
  - `Failed`: Mensagem clara do motivo da falha com botão de nova tentativa.
- **Critérios de Aceite**: Bloqueio prévio de arquivos > 25MB no cliente; interrupção estrita do polling ao alcançar estado terminal (`COMPLETED` ou `FAILED`).

---

## 7. Matriz de Requisitos vs. Verificação Determinística

| Requisito EARS | Vetor de Verificação Automatizada | Critério de Sucesso |
|:---|:---|:---|
| `REQ-FE-01` a `REQ-FE-07` | Teste de unidade Vitest em `auth-interceptor.test.ts` | Validação de armazenamento volátil em memória e chamada de refresh no 401 |
| `REQ-FE-08` | Teste de componente em `TenantSelector.test.tsx` | Verificação do header `X-Tenant-ID` presente nas requisições simuladas |
| `REQ-FE-12` | Teste E2E Playwright em `transactions-filters.spec.ts` | URL reflete `?account_id=...&category_id=...` e restaura estado no reload |
| `REQ-FE-18` a `REQ-FE-23` | Teste de componente Vitest em `TransactionsTable.test.tsx` | Paginação e reversão otimista validados com servidor mock MSW |
| `REQ-FE-27` | Auditoria de código estática via ESLint | Regex asserindo ausência de `parseFloat` e `Number()` em `web/src` |
| `REQ-FE-28` a `REQ-FE-32` | Teste de integração Vitest em `IngestionDropzone.test.tsx` | Rejeição de arquivo > 25MB e asserção de polling exponencial |
| `REQ-FE-33` | Teste unitário em `problem.test.ts` | Validação de parsing de JSON RFC 7807 para formulários |
| `REQ-FE-36` a `REQ-FE-38` | Auditoria `axe-core` / Playwright a11y | Zero violações críticas de acessibilidade WCAG AA |
| `FE-BUILD-01` | Script de compilação em clone limpo | `npm ci && npm run build` gera pasta `dist/` sem alertas |
| `FE-EMBED-01` | Teste unitário Go em `internal/adapters/inbound/http/assets_test.go` | `embed.FS` contém arquivos estáticos e `index.html` > 0 bytes |
