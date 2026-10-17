# [Frontend] Definição e desenvolvimento do frontend da aplicação (ADR + Spec + MVP)

## 1. Contexto & Motivação

O **IntelliFinance** possui um backend robusto em Go 1.22+ com Arquitetura Hexagonal, API REST spec-first via Chi v5, conformidade com RFC 7807 (Problem Details), persistência em PostgreSQL 16 com isolamento multi-tenant via Row-Level Security (RLS) e suporte a execução dual-target (Homelab MQT_Home `lm-claw` e GCP Cloud Run serverless).

Entretanto, a ausência de uma interface de usuário bloqueia três objetivos fundamentais do produto:
1. **Interactive Portfolio Showcase (`PRD §6.3`)**: O repositório público promete que avaliadores e recrutadores podem executar `make dev-up` ou `docker compose`, conectar um banco sandbox e visualizar transações sendo ingeridas, deduplicadas e categorizadas em tempo real. Sem interface, essa experiência degrada para saídas brutas de `curl`.
2. **Revisão Humana de IA (`PRD §4.5`)**: O modelo de categorização assinala lançamentos com confiança < 85% para revisão humana ("Needs Review"), permitindo que o usuário sobrescreva a categoria e alimente o aprendizado contínuo. Esse fluxo requer interface interativa.
3. **Visão Financeira Familiar (`PRD §2`, §4.7)**: O acompanhamento de saldos consolidados, segregação de receitas clínicas e apuração do burn rate mensal dependem de visualização estruturada.

---

## 2. Decisão Arquitetural da Stack (Avaliação por Mérito Próprio)

Conforme a **Diretriz Direta do Operador**, todas as decisões foram reavaliadas por **puro mérito contemporâneo**, expurgando qualquer restrição ou viés decorrente de falhas do projeto legado:
- **Stack Aprovada**: **React 19 + Vite 6/7 + TypeScript + TanStack Query v5 + Tailwind CSS**.
- **Modelo de Entrega**: Aplicação de Página Única (SPA) com compilação estática (`web/dist/`) embutida diretamente no binário Go compilado através do pacote padrão `embed.FS` (`internal/adapters/inbound/http/assets`).
- **Justificativas Chave de Mérito**:
  1. *Binário Único & Custo $0 (`NFR-REL-02`, `NFR-REL-03`)*: O artefato resultante é puramente estático, dispensando servidores Node.js em produção e preservando a imagem distroless abaixo de 35MB.
  2. *Garantia Anti-Drift com OpenAPI 3.0*: Geração automatizada de definições TypeScript a partir de `api/openapi/openapi.yaml` (`openapi-typescript`), quebrando o build no CI caso ocorra divergência de contrato.
  3. *Reatividade e Resiliência*: TanStack Query resolve nativamente cache, polling com backoff exponencial para jobs de ingestão e mutações otimistas com rollback para a fila de revisão de IA.

---

## 3. Inventário Real da API & Estagiamento do Escopo

Uma auditoria exaustiva no código-fonte Go revelou que os recursos de **Contas, Categorias e Transações** já estão plenamente funcionais no backend, enquanto os endpoints de **Autenticação JWT, Analytics e Ingestão Multipart** ainda não possuem handlers registrados no roteador Chi.

Para viabilizar entregas ágeis e evitar bloqueios, o escopo está dividido em:

### Fase A — Governança e Especificação Formal (PR #37)
- [x] Reescrita do `ADR-009` fundamentada estritamente no mérito técnico da stack.
- [x] Especificação formal `SPEC-007-FRONTEND` com 38 requisitos no padrão EARS.
- [x] Auditoria crítica adversarial (`DEVILS_ADVOCATE.md`) e de segurança (`SECURITY_REVIEW.md`).
- [x] Matriz de stack com 6 opções comparadas (`STACK_EVALUATION.md`).

### Fase B1 — Scaffolding & MVP Core (Consumindo a API Existente)
- [ ] Inicialização da estrutura em `/web` com React 19, Vite, TypeScript e Tailwind.
- [ ] Geração automática de tipos da API via `openapi-typescript`.
- [ ] Cliente HTTP tipado com suporte a modo Showcase local (injeção de header `X-Tenant-ID`).
- [ ] Implementação da Tela de Contas (`/accounts`): listagem de saldos e modal de cadastro.
- [ ] Implementação da Tela de Transações (`/transactions`): tabela paginada, filtros e sobrescrita de categoria com feedback otimista via `PATCH /api/v1/transactions/{id}`.
- [ ] Configuração do Chi router para servir os ativos via `embed.FS` em `/assets/*` e fallback da SPA em `/*`.

### Fase B2 — Expansões e Telas Avançadas (Após Conclusão dos Endpoints)
- [ ] Integração com `POST /api/v1/auth/login` e ciclo de Silent Refresh com cookies HTTP-Only.
- [ ] Tela de Dashboard (`/dashboard`) consumindo `GET /api/v1/analytics/cash-flow` e distribuição por categoria.
- [ ] Tela de Ingestão (`/ingestion`) com dropzone (<25MB), upload multipart e polling de status de job.
- [ ] Suíte de testes E2E com Playwright cobrindo acessibilidade WCAG AA.

---

## 4. Invariantes Inegociáveis (Red Lines)

1. **Tokens em Memória Apenas**: Access Token JWT mantido estritamente em memória volátil; proibido uso de `localStorage` ou `sessionStorage`.
2. **Zero Aritmética no Cliente**: Proibição terminante de `parseFloat` ou `number` para cálculos monetários. Agregações são de responsabilidade do servidor.
3. **Zero Inferência de Tenant**: O frontend nunca deve aplicar filtros client-side como substituto de isolamento de inquilino. A segurança é garantida por RLS e middleware.
4. **Zero Dados Pessoais Reais**: Nenhum dado verídico em fixtures ou testes (`ADR-008`). Uso exclusivo de fixtures sintéticas de `cmd/fixturegen`.
5. **Single-Binary**: O frontend é servido pelo binário Go via `embed.FS`.

---

## 5. Critérios de Aceitação da Issue

- [ ] `npm ci && npm run build` no diretório `/web` executa de forma limpa e determinística.
- [ ] `npm run gen:api && git diff --exit-code web/src/api/generated` garante zero drift de contrato.
- [ ] O comando `make build` compila o binário Go embutindo a SPA sem falhas.
- [ ] Ao executar `make dev-up`, a interface é acessível em `localhost:8080/` e permite navegar e operar as contas e transações da base sintética local.
- [ ] Linter Go (`golangci-lint run`) e linter do frontend passam com zero erros ou avisos.
