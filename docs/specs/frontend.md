# Especificação Técnica do Frontend: Telas, Rotas e Contratos de Dados

- **Document ID**: SPEC-007-FRONTEND
- **Status**: Proposed (Companion to ADR-009)
- **Specification Standard**: OpenAPI 3.0.3 conforme `SPEC-002-API` & RFC 7807 Problem Details
- **Stack Aprovada**: React 19 + Vite 6/7 + TypeScript + TanStack Query v5 + Tailwind CSS (ADR-009)
- **Modo de Entrega**: Ativos estáticos embutidos no binário Go via `embed.FS` (`cmd/api`)
- **Locale**: `pt-BR` (exclusivo)

---

## 1. Escopo & Objetivos de Produto

Esta especificação define os requisitos formais de interface, fluxos de navegação, contratos de integração e comportamentos determinísticos da SPA (Single-Page Application) do **IntelliFinance**.

O frontend materializa os seguintes requisitos de produto do PRD:
1. **Interactive Portfolio Showcase (`PRD §6.3`)**: Demonstração executável localmente via `make dev-up`, exibindo ingestão, reconciliação e categorização inteligente em tempo real.
2. **Revisão Humana de IA (`PRD §4.5`)**: Fila de inspeção de lançamentos com baixa confiança de categorização (`ai_confidence < 0.85`), permitindo sobrescrita de categoria com feedback otimista.
3. **Visão Financeira Unificada (`PRD §2`)**: Painel de saldos, segregação de contas da Clínica Eluma (`RESTRICTED_BUSINESS`) e contas discricionárias (`PERSONAL_PRIVATE`), e monitoramento de despesas familiares compartilhadas (`HOUSEHOLD_SHARED`).

**Fora de Escopo**: Aplicativo mobile nativo, internacionalização (i18n além de pt-BR) e gestão administrativa de inquilinos.

---

## 2. Mapa de Rotas da SPA

| Rota | Tela | Auth | Finalidade & Descrição |
|:---|:---|:---:|:---|
| `/login` | Login | Não | Autenticação via credenciais e emissão de sessão (JWT em memória) |
| `/` | Redirect | Sim | Redirecionamento automático para `/dashboard` |
| `/dashboard` | Painel Geral | Sim | Métricas de fluxo de caixa, distribuição por categoria e lista de contas |
| `/transactions` | Transações | Sim | Listagem paginada, filtros combinados e fila de revisão de IA |
| `/accounts` | Contas | Sim | Grade de contas bancárias, saldos e cadastro com badges de visibilidade |
| `/ingestion` | Ingestão | Sim | Dropzone para upload multipart de extratos e acompanhamento assíncrono |
| `*` | Não Encontrado | — | Estado 404 amigável com atalho de retorno ao painel |

O roteamento é gerenciado no lado do cliente. O backend Go atua como fallback servindo `index.html` para todas as rotas que não pertençam ao prefixo `/api/` ou `/healthz` (ADR-009 §6.1).

---

## 3. Inventário da API & Estagiamento de Backend

Para garantir previsibilidade técnica, a interface diferencia endpoints já disponíveis no código-fonte Go daqueles que requerem extensão de backend:

### 3.1 Endpoints Disponíveis Imediatamente (Fase B1 — MVP Core)
- `GET /healthz`, `GET /readyz`: Probes de saúde.
- `GET /api/v1/accounts`: Lista de contas do tenant autenticado (`dto.AccountListResponse`).
- `POST /api/v1/accounts`: Criação de nova conta financeira (`dto.CreateAccountRequest`).
- `GET /api/v1/accounts/{id}`: Detalhes de conta específica.
- `PATCH /api/v1/accounts/{id}`: Atualização de metadados da conta.
- `GET /api/v1/categories`: Árvore hierárquica de categorias (`dto.CategoryListResponse`).
- `GET /api/v1/transactions`: Consulta paginada e filtrada de lançamentos (`dto.TransactionListResponse`).
- `POST /api/v1/transactions`: Criação de transação avulsa.
- `GET /api/v1/transactions/{id}`: Detalhes de transação específica.
- `PATCH /api/v1/transactions/{id}`: Atualização de transação (incluindo mutação de categoria via `{ "category_id": "uuid" }`).
- `DELETE /api/v1/transactions/{id}`: Exclusão de transação (204 No Content).

*Nota sobre Autenticação na Fase B1*: O backend atual opera com o middleware `TenantContextMiddleware`, que valida o header HTTP `X-Tenant-ID`. Na Fase B1, o frontend suporta o **Modo Showcase / Dev**, injetando o header correspondente ao perfil selecionado (Família Marquitti ou Convidado) até a liberação dos endpoints JWT.

### 3.2 Endpoints Dependentes de Extensão do Backend (Fase B2)
- `POST /api/v1/auth/login`: Autenticação e emissão de JWT + cookie de refresh.
- `POST /api/v1/auth/refresh`: Renovação silenciosa de sessão via cookie HTTP-Only.
- `POST /api/v1/auth/logout`: Revogação de sessão.
- `GET /api/v1/analytics/cash-flow`: Agregações de receitas, despesas e poupança.
- `GET /api/v1/analytics/by-category`: Totais agrupados por categoria para o período.
- `POST /api/v1/ingestion/upload`: Upload multipart de arquivos de extrato (.ofx, .csv, .xlsx, .pdf).
- `GET /api/v1/ingestion/jobs/{id}`: Polling de processamento assíncrono e contadores de deduplicação.

---

## 4. Envelopes e Tratamento de Erros da API

### 4.1 Envelope de Coleção Paginada (`PaginatedResponse<T>`)
Conforme implementado em `internal/adapters/inbound/http/dto/pagination.go`:

```json
{
  "items": [ /* lista de recursos */ ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 142,
    "total_pages": 3,
    "has_next": true,
    "has_prev": false
  }
}
```

### 4.2 RFC 7807 Problem Details (`application/problem+json`)
Conforme implementado em `internal/adapters/inbound/http/problem/problem.go`:

```json
{
  "type": "https://api.intellifinance.app/errors/validation-failed",
  "title": "Validation Failed",
  "status": 422,
  "detail": "The transaction amount cannot be zero.",
  "instance": "/api/v1/transactions",
  "code": "ERR_VALIDATION_FAILED",
  "timestamp": "2026-10-17T16:00:00Z",
  "invalid_params": [
    {
      "name": "amount",
      "reason": "must be a non-zero signed decimal value"
    }
  ]
}
```

**Regras de Tratamento na UI**:
1. O campo `title` é apresentado como título do alerta/toast.
2. O campo `detail` é exibido como texto explicativo para o usuário.
3. Se presente, `invalid_params` é percorrido e vinculado diretamente aos inputs correspondentes do formulário.
4. Em falhas de rede ou 500, exibe-se mensagem genérica de erro ("Serviço temporariamente indisponível") preservando o status HTTP, proibindo a renderização de JSON bruto ou stack traces.

---

## 5. Especificação Detalhada das Telas

### 5.1 Login — `/login`
- **Contrato**: `POST /api/v1/auth/login` (Fase B2) ou Seletor de Perfil Showcase (Fase B1).
- **Layout**: Card centralizado, logotipo IntelliFinance, campos E-mail e Senha, botão "Entrar".
- **Estados**:
  - `Idle`: Formulário pronto para entrada.
  - `Submitting`: Inputs desabilitados, botão com spinner de carregamento.
  - `Error 401`: Banner de credenciais inválidas; foco redirecionado para o campo de senha.
  - `Error 422`: Erros de validação inline (`invalid_params`).
- **Invariantes**: Nenhuma credencial em storage persistente; senha nunca impressa em logs.

### 5.2 Painel Geral — `/dashboard`
- **Contratos**: `GET /api/v1/analytics/cash-flow`, `GET /api/v1/analytics/by-category`, `GET /api/v1/accounts`.
- **Layout**:
  - Topo: 4 cartões de KPIs (Receita Total, Despesa Total, Saldo Líquido, Taxa de Poupança).
  - Centro: Gráfico de distribuição por categoria e lista de contas ativas com saldo.
- **Estados**:
  - `Loading`: Skeletons animados nas mesmas dimensões dos cartões e gráficos.
  - `Empty`: Mensagem de ausência de dados no período com CTA para `/ingestion`.
  - `Success`: Métricas financeiras calculadas no servidor e formatadas em `pt-BR`.
- **Invariantes**: Zero cálculos de agregação no cliente; valores monetários formatados estritamente via `Intl.NumberFormat`.

### 5.3 Transações & Revisão de IA — `/transactions`
- **Contratos**: `GET /api/v1/transactions` (paginada), `PATCH /api/v1/transactions/{id}` (sobrescrita).
- **Colunas**:
  - Data (`date`): Formatada como `DD/MM/AAAA`.
  - Descrição (`description`): Truncada com tooltip para texto integral.
  - Conta: Nome da conta financeira e badge de visibilidade.
  - Categoria (`category_id`): Dropdown interativo com busca; chip colorido.
  - Valor (`amount`): String decimal com sinal; positivo em verde (`text-emerald-600`), negativo em vermelho (`text-rose-600`).
  - IA / Status: Badge destacada ("Revisar") se `is_ai_categorized = true` e `ai_confidence < 0.85`.
- **Ações**:
  - Paginação com botões Anterior/Próxima e indicador `Página X de Y`.
  - Filtros por Conta, Categoria, Período e flag `needs_review`.
  - Sobrescrita de categoria inline: emite `PATCH /api/v1/transactions/{id}` enviando `{ "category_id": "..." }` com atualização otimista na tabela e rollback em caso de falha.
- **Invariantes**: Filtros sincronizados com a query string da URL.

### 5.4 Contas Financeiras — `/accounts`
- **Contratos**: `GET /api/v1/accounts`, `POST /api/v1/accounts`.
- **Layout**: Grade responsiva de cartões agrupados por tipo (`Conta Corrente`, `Cartão de Crédito`, `Investimentos`, `Dinheiro`, `Empresarial`), com botão superior "Nova Conta".
- **Formulário de Cadastro (Modal)**:
  - Campos: Nome da Conta, Tipo de Conta, Moeda (`BRL`), Saldo Inicial (opcional, string decimal), Visibilidade (`Compartilhada`, `Pessoal Privada`, `Restrita Clínica`).
  - Ao selecionar `Pessoal Privada`, a interface exibe aviso informativo destacando que a conta não será visível para outros membros do tenant.
- **Invariantes**: Saldo inicial validado e enviado como string decimal, proibindo conversão por `parseFloat`.

### 5.5 Ingestão de Extratos — `/ingestion`
- **Contratos**: `POST /api/v1/ingestion/upload` (multipart), `GET /api/v1/ingestion/jobs/{id}` (polling).
- **Fluxo Operacional**:
  1. O usuário seleciona ou arrasta um arquivo (`.ofx`, `.csv`, `.xlsx`, `.pdf`) e vincula a uma conta de destino.
  2. Validação client-side imediata de extensão e limite de tamanho (máximo 25MB, `NFR-PERF-04`).
  3. Upload com indicador de progresso real.
  4. Resposta `202 Accepted` com `job_id` inicia polling automático (intervalos de 2s com backoff exponencial até teto de 10s).
  5. Conclusão do job apresenta relatório visual: `inserted_count` (novas transações), `duplicate_count` (ignoradas pela deduplicação canônica de SHA-256) e `error_count`.
- **Invariantes**: Polling finaliza obrigatoriamente em estado terminal (`COMPLETED` ou `FAILED`) ou timeout de 5 minutos.

---

## 6. Design Tokens & Identidade Visual

Tokens unificados para suporte completo a temas claro e escuro (`design/tokens.md`):

| Token | Light Mode | Dark Mode | Aplicação |
|:---|:---:|:---:|:---|
| `primary` | `#0D47A1` | `#1976D2` | Ações principais, links, branding |
| `success` | `#15803D` | `#22C55E` | Entradas financeiras, superávit, aprovação |
| `error` | `#B91C1C` | `#EF4444` | Saídas financeiras, alertas de validação, débitos |
| `warning` | `#B45309` | `#F59E0B` | Badges de revisão de IA ("Needs Review") |
| `background` | `#F8FAFC` | `#0B0F17` | Fundo principal da página |
| `surface` | `#FFFFFF` | `#151C28` | Fundo de cartões, modais e tabelas |
| `border` | `#E2E8F0` | `#222D3D` | Bordas e divisores estruturais |
| `text-primary`| `#0F172A` | `#F8FAFC` | Texto de leitura principal |
| `text-muted` | `#64748B` | `#94A3B8` | Metadados, rótulos e legendas |

Tipografia padrão: `Inter` / `system-ui`. Ícones: `lucide-react`.

---

## 7. Requisitos Não-Funcionais do Frontend

- **FE-NFR-01 (Build Determinístico)**: O build em clone limpo via `npm ci && npm run build` deve executar com zero falhas de dependências ou peer dependencies.
- **FE-NFR-02 (Anti-Drift de Contrato)**: A geração de tipos `npm run gen:api` contra `api/openapi/openapi.yaml` deve produzir diff limpo no CI (`git diff --exit-code`).
- **FE-NFR-03 (Token In-Memory)**: Nenhum access token deve ser gravado em `localStorage` ou `sessionStorage`. Fiscalizado por teste estático no CI.
- **FE-NFR-04 (Zero Floats em Moeda)**: Nenhuma operação aritmética com `number` ou `parseFloat` deve ocorrer sobre valores monetários no cliente.
- **FE-NFR-05 (Acessibilidade WCAG AA)**: Navegação por teclado funcional com anéis de foco visíveis (`focus-visible:ring-2`) e contraste mínimo de 4.5:1.
- **FE-NFR-06 (Single-Binary)**: Os ativos da SPA devem ser servidos diretamente pelo executável Go através de `embed.FS`, dispensando servidor Node.js em produção.
- **FE-NFR-07 (Zero PII)**: Nenhum dado real em fixtures ou telas de demonstração (`ADR-008`).

---

## 8. Verificação de Aceitação e Comandos de Validação

```bash
# 1. Validação de build determinístico
npm --prefix web ci && npm --prefix web run build
test -f web/dist/index.html || exit 1

# 2. Validação anti-drift de contrato OpenAPI
npm --prefix web run gen:api && git diff --exit-code web/src/api/generated

# 3. Auditoria estática de segurança e precisão
! grep -rE "localStorage|sessionStorage" web/src
! grep -rE "parseFloat|Number\(.*amount" web/src

# 4. Qualidade e testes automatizados
npm --prefix web run lint
npm --prefix web run test

# 5. Validação de entrega unificada Go
go build ./...
go test -v -race ./...
golangci-lint run
```
