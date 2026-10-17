# IntelliFinance Frontend Specification: Screens, Routes & Data Contracts

- **Document ID**: SPEC-007-FRONTEND
- **Status**: Proposed (companion to ADR-009)
- **Specification Standard**: consome OpenAPI 3.0.3 conforme `SPEC-002-API`
- **Stack**: React 19 + Vite 7 + TypeScript + TanStack Query + Tailwind (ADR-009 §5)
- **Locale**: pt-BR (única)

---

## 1. Escopo

Especifica a SPA de IntelliFinance: rotas, telas, estados, contratos de dados consumidos da API REST e os requisitos de integração que o backend precisa satisfazer.

Fora de escopo: app mobile nativo, multi-idioma, telas de administração de tenant.

---

## 2. Mapa de Rotas

| Rota | Tela | Auth | Descrição |
|---|---|---|---|
| `/login` | Login | Não | Autenticação e emissão de sessão |
| `/` | Redirect | Sim | Redireciona para `/dashboard` |
| `/dashboard` | Painel | Sim | Saldos, burn rate, distribuição por categoria |
| `/transactions` | Transações | Sim | Lista paginada com filtros e revisão de IA |
| `/accounts` | Contas | Sim | CRUD de contas com visibilidade |
| `/ingestion` | Ingestão | Sim | Upload multipart e acompanhamento de job |
| `*` | Not Found | — | Estado vazio com link para o painel |

O roteamento é client-side; o backend serve `index.html` para qualquer rota não-API (ADR-009 §6.1).

---

## 3. Pré-requisitos de Backend

Itens que a API precisa expor antes que a Fase B esteja completa. Nenhum deles altera o contrato existente — são adições.

| # | Requisito | Endpoint | Estado hoje | Nota |
|---|---|---|---|---|
| B1 | Login com emissão de JWT + refresh httpOnly | `POST /api/v1/auth/login` | Documentado no ADR-002 §9, **sem handler** | Bloqueante |
| B2 | Renovação silenciosa de sessão | `POST /api/v1/auth/refresh` | **Inexistente** | Bloqueante para o boot da SPA |
| B3 | Encerramento de sessão / revogação do refresh | `POST /api/v1/auth/logout` | **Inexistente** | Necessário para o fluxo de logout |
| B4 | Agregações para o painel | `GET /api/v1/analytics/cash-flow` | **Inexistente** | Saldo, receita, despesa, savings rate |
| B5 | Distribuição por categoria | `GET /api/v1/analytics/by-category` | **Inexistente** | Aceita janela temporal |
| B6 | Upload multipart de extrato | `POST /api/v1/ingestion/upload` | Documentado no ADR-002 §9 | Verificar implementação |
| B7 | Status do job de ingestão | `GET /api/v1/ingestion/jobs/{id}` | Documentado no ADR-002 §9 | Usado no polling |
| B8 | Sobrescrita de categoria | `PATCH /api/v1/transactions/{id}/category` | Documentado no ADR-002 §9 | Usado na revisão |
| B9 | CORS para origem de dev | middleware existente | Existe (`middleware/cors.go`) | Configurar origem do Vite em dev |

**Decisão de arquitetura:** agregações são calculadas no servidor (PRD §4.7, FR-7.4). O cliente nunca soma valores monetários.

---

## 4. Envelopes Consumidos

Conforme `SPEC-002-API` §2:

**Coleção paginada**
```json
{
  "items": [ /* recurso */ ],
  "pagination": { "page": 1, "limit": 50, "total": 142, "total_pages": 3, "has_next": true, "has_prev": false }
}
```

**Erro (RFC 7807)**
```json
{
  "type": "https://api.intellifinance.app/errors/validation-failed",
  "title": "Validation Failed",
  "status": 422,
  "detail": "The transaction amount cannot be zero.",
  "instance": "/api/v1/transactions",
  "code": "ERR_VALIDATION_FAILED",
  "timestamp": "2026-10-17T16:00:00Z",
  "invalid_params": [ { "name": "amount", "reason": "Value must be non-zero" } ]
}
```

O cliente trata todo erro através do modelo de `§6.3` do ADR-009.

---

## 5. Especificação das Telas

Cada tela define: **dados consumidos**, **estados obrigatórios**, **ações** e **critérios de aceitação**.

---

### 5.1 Login — `/login`

**Dados**: `POST /api/v1/auth/login` → `{ access_token, expires_in }` + cookie de refresh.

**Layout**: centrado, campos de e-mail e senha, marca IntelliFinance, paleta primária `#0D47A1`.

**Estados**
| Estado | Comportamento |
|---|---|
| Idle | Formulário habilitado |
| Submitting | Botão desabilitado com indicador; campos não editáveis |
| Error 401 | Mensagem a partir do `detail` do RFC 7807; foco no campo de senha |
| Error 422 | `invalid_params` mapeado inline por campo |
| Error de rede | Mensagem genérica com ação de retry |

**Ações**: submeter credenciais; redirecionar para `/dashboard` no sucesso; link para o README de setup local quando não há backend.

**Critérios**: nenhuma credencial persistida fora de memória; senha nunca logada; autofocus no primeiro campo; submit por Enter.

---

### 5.2 Painel — `/dashboard`

**Dados**
- `GET /api/v1/analytics/cash-flow?from=&to=` → receita total, despesa total, savings, savings rate
- `GET /api/v1/analytics/by-category?from=&to=` → valor por categoria
- `GET /api/v1/accounts` → saldo por conta

**Layout**: cartões de resumo no topo (receita, despesa, savings, taxa), gráfico de distribuição por categoria, lista de contas com saldo. Referência visual: `design/stitch/transactions-page/screen.png`.

**Estados**
| Estado | Comportamento |
|---|---|
| Loading | Skeletons nos cartões e no gráfico |
| Empty | "Nenhuma transação no período" com CTA para `/ingestion` |
| Error | Cartão de erro com `detail`, botão "Tentar novamente" |
| Success | Cartões + gráfico + lista de contas |

**Regras**
- Valores monetários formatados em `pt-BR`/BRL; nenhuma aritmética no cliente.
- Contas com `visibility = PERSONAL_PRIVATE` de outros membros **não aparecem** — a decisão vem da API.
- Taxa de savings exibida com uma casa decimal.

**Ações**: alternar janela temporal (mês corrente, 3 meses, ano); clicar em conta navega para `/transactions` filtrado.

---

### 5.3 Transações — `/transactions`

**Dados**
- `GET /api/v1/transactions?page=&limit=&account_id=&from=&to=&category_id=` → envelope paginado
- `PATCH /api/v1/transactions/{id}/category` → sobrescrita

**Layout**: tabela com data, descrição, categoria, valor, indicador de IA; barra de filtros no topo; paginação no rodapé.

**Colunas**
| Coluna | Fonte | Regra |
|---|---|---|
| Data | `date` | `DD/MM/AAAA` |
| Descrição | `description` | Truncar com tooltip |
| Categoria | `category.name` | Chip; clicável para reclassificar |
| Valor | `amount` | BRL; negativo em vermelho, positivo em verde |
| IA | `is_ai_categorized`, `ai_confidence` | Badge "Revisar" quando `needs_review` |

**Estados**
| Estado | Comportamento |
|---|---|
| Loading | Skeleton de linhas mantendo o cabeçalho |
| Empty (sem dados) | "Nenhuma transação" + CTA para ingestão |
| Empty (filtro) | "Nenhum resultado para os filtros" + limpar filtros |
| Error | Faixa de erro com `detail` e retry |
| Success | Tabela paginada |

**Ações**: filtrar por conta, período e categoria; paginar; reclassificar categoria (otimista com rollback em erro); abrir a fila de revisão.

**Critérios**: filtros refletidos na query string (deep link compartilhável); paginação respeita `has_next`/`has_prev`; erro de `PATCH` reverte a linha e exibe o `detail`.

---

### 5.4 Contas — `/accounts`

**Dados**
- `GET /api/v1/accounts`
- `POST /api/v1/accounts`
- `GET /api/v1/categories` (para contexto)

**Layout**: grade de cartões de conta; modal de criação.

**Campos do formulário**
| Campo | Obrigatório | Validação |
|---|---|---|
| Nome | Sim | 1-120 caracteres, trim |
| Tipo | Sim | `CHECKING`, `CREDIT_CARD`, `INVESTMENT`, `CASH`, `BUSINESS` |
| Moeda | Sim | ISO 4217, default `BRL` |
| Saldo inicial | Não | String decimal, 2 casas, aceita formato `pt-BR` |
| Visibilidade | Sim | `HOUSEHOLD_SHARED`, `PERSONAL_PRIVATE`, `RESTRICTED_BUSINESS` |

**Estados**: loading (skeletons), empty ("Crie sua primeira conta"), error (faixa com retry), success (grade).

**Regras**: `PERSONAL_PRIVATE` carrega aviso de que a conta não será visível para outros membros do tenant; o aviso descreve o comportamento, não implementa filtragem no cliente.

**Critérios**: validação de formulário espelha `invalid_params` do backend; erro de campo exibido inline; saldo inicial enviado como string decimal, nunca `parseFloat`.

---

### 5.5 Ingestão — `/ingestion`

**Dados**
- `POST /api/v1/ingestion/upload` (multipart)
- `GET /api/v1/ingestion/jobs/{id}` → status e contadores

**Layout**: dropzone, seletor de conta de destino, histórico de jobs.

**Fluxo**
1. Usuário seleciona arquivo (`.ofx`, `.csv`, `.xlsx`, `.pdf`) e a conta de destino.
2. Client valida extensão e tamanho (limite de 25MB, NFR-PERF-04) antes do upload.
3. Upload com barra de progresso real.
4. Resposta `202 Accepted` com `job_id` → inicia polling.
5. Polling com backoff (1s, 2s, 4s, 8s, teto de 10s), encerrando em estado terminal ou após timeout de 5 minutos.
6. Exibe resumo: `inserted_count`, `duplicate_count`, `error_count`.

**Estados do job**
| Estado | UI |
|---|---|
| `PENDING` | Aguardando processamento |
| `PROCESSING` | Processando com contador de itens |
| `COMPLETED` | Resumo com os três contadores |
| `FAILED` | Erro com `detail` e opção de reenviar |
| Timeout de polling | Aviso de que o processamento continua; link para o histórico |

**Regras**: nenhum arquivo real de cliente no repositório — fixtures de `test/fixtures/` são gerados por `cmd/fixturegen` (ADR-008 §7).

**Critérios**: rejeitar extensão não suportada; rejeitar arquivo acima do limite; polling para em estado terminal; o resumo distingue claramente inseridos de duplicados.

---

## 6. Estados Transversais

| Estado | Regra |
|---|---|
| **Boot** | Tenta refresh silencioso; sucesso → rota pretendida; falha → `/login` |
| **401** | Uma tentativa de refresh; segunda falha → `/login` limpando estado em memória |
| **403** | Tela de acesso negado com o `detail` da API |
| **404 (API)** | Tratado como recurso inexistente; nunca inferir existência |
| **5xx** | Mensagem genérica + retry; sem expor stack |
| **Offline** | Aviso de conexão; queries pausadas até reconectar |

Todo estado de erro preserva o `status` HTTP e nunca renderiza JSON bruto.

---

## 7. Design Tokens

Derivados de `design/stitch/transactions-page/code.html`, a formalizar em `design/tokens.md`.

| Token | Light | Dark |
|---|---|---|
| `primary` | `#0D47A1` | `#0D47A1` |
| `error` | `#D32F2F` | `#D32F2F` |
| `success` | `#2E7D32` | `#2E7D32` |
| `background` | `#F4F6F8` | `#111721` |
| `card` | `#FFFFFF` | `#1F2937` |
| `border` | `#E5E7EB` | `#374151` |
| `text` | `#1F2937` | `#F9FAFB` |

Tipografia: Roboto (300/400/500/700). Ícones: Material Symbols Outlined. Tema claro e escuro suportados.

---

## 8. Requisitos Não-Funcionais do Frontend

| ID | Requisito | Verificação |
|---|---|---|
| FE-NFR-01 | Build reprodutível de clone limpo, sem erro de resolução ou peer dependency | `npm ci && npm run build` em clone limpo |
| FE-NFR-02 | Tipos gerados de `openapi.yaml`; regeneração sem diff | `npm run gen:api && git diff --exit-code` |
| FE-NFR-03 | Nenhum token em `localStorage`/`sessionStorage` | Grep no bundle + revisão no CI |
| FE-NFR-04 | Nenhuma aritmética monetária com `number` no cliente | Grep por `parseFloat`/`Number(` em módulos de domínio |
| FE-NFR-05 | Navegação completa por teclado; foco visível | Revisão manual + `eslint-plugin-jsx-a11y` |
| FE-NFR-06 | SPA servida pelo mesmo binário Go, sem runtime adicional | Inspeção do artefato + ADR-009 §6.1 |
| FE-NFR-07 | Nenhum dado pessoal real em fixtures ou capturas | Alinhado a ADR-008 §7 |

---

## 9. Verificação de Aceitação

```bash
# 1. Build determinístico
rm -rf web/node_modules web/dist
npm --prefix web ci && npm --prefix web run build

# 2. Sem drift de contrato
npm --prefix web run gen:api && git diff --exit-code web/src/api/generated

# 3. Sem token em storage
! grep -rE "localStorage|sessionStorage" web/src

# 4. Sem aritmética monetária no cliente
! grep -rE "parseFloat|Number\(.*amount" web/src

# 5. Lint e testes
npm --prefix web run lint && npm --prefix web test

# 6. Stack completa
make dev-up
curl -fsS localhost:8080/healthz
curl -fsSI localhost:8080/            | grep -i "content-type: text/html"
curl -fsSI localhost:8080/transactions | grep -i "content-type: text/html"
```

Todos os comandos acima devem terminar com exit code 0.

---

## 10. Referências

- `docs/adrs/ADR-009-frontend-architecture.md` (decisão de stack)
- `docs/specs/api.md` (SPEC-002-API — contratos REST)
- `docs/adrs/ADR-002-api-architecture.md` (RFC 7807, envelopes, rotas)
- `docs/PRD.md` §4.5, §4.7, §6.3, §7.2
- `design/stitch/transactions-page/code.html` (referência visual)
