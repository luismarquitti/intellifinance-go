# Consolidação e Veredito do Comitê de Revisão — Frontend IntelliFinance

- **Document ID**: REV-001-FRONTEND-COMMITTEE
- **Data**: 2026-10-17
- **Membros do Comitê**:
  - Archon (`01-spec-architect`)
  - UI/Frontend Architect (`frontend-architect`)
  - Shadow Judge (`04-devils-advocate`)
  - Security Reviewer (`security-reviewer`)
- **Status**: Veredito Final Consolidado (Aprovado com Modificações)

---

## 1. Matriz de Decisões do Comitê sobre Apontamentos Críticos

Para cada vulnerabilidade, premissa falsa ou apontamento levantado nas passagens anteriores, o Comitê deliberou conforme a tabela abaixo:

| # | Apontamento / Crítica | Origem | Decisão | Justificativa de Uma Linha | Ação Incorporada nos Documentos |
|:---:|:---|:---:|:---:|:---|:---|
| **1** | Expurgar justificativas retrospectivas baseadas no trauma do build legado | Shadow Judge | **ACEITAR** | A diretriz direta do operador revoga formalmente o legado como premissa de decisão arquitetural. | `ADR-009` e `STACK_EVALUATION.md` foram reescritos fundamentando a escolha de stack 100% no mérito técnico contemporâneo. |
| **2** | Ausência real de handlers de autenticação JWT (`/auth/login`, `/auth/refresh`) no backend Go | Shadow Judge | **ACEITAR** | O código atual do backend requer `X-Tenant-ID` e não possui rotas de auth registradas no router. | Especificado modo de compatibilidade/showcase na Fase B1 (injeção de tenant) e adiado login JWT para Fase B2. |
| **3** | Risco de deadlock ou violação de invariante contábil pela ausência de rotas `/analytics/*` | Shadow Judge | **ACEITAR** | O frontend não pode calcular agregações com float, e o backend ainda não expõe endpoints de analytics. | Desacoplado o escopo da Fase B: Telas de Transações e Contas entram na Fase B1; Dashboard analítico completo entra na Fase B2. |
| **4** | Correção da rota de sobrescrita de categoria de `/transactions/{id}/category` para `/transactions/{id}` | Shadow Judge | **ACEITAR** | O handler real em `transaction_handler.go:265` aceita mutação de `category_id` via `PATCH /transactions/{id}`. | `SPEC.md` e `docs/specs/frontend.md` alinhados com o payload real `{ "category_id": "uuid" }`. |
| **5** | Resolver divergência de formato de envelopes paginados (`items` vs `data`) | Shadow Judge | **ACEITAR** | A implementação Go real em `dto/pagination.go` e `docs/specs/api.md` usam padronizadamente `items` e `pagination`. | Uniformizado em todos os contratos TypeScript de `SPEC.md` e `frontend.md`. |
| **6** | Implementar Mutex / Queue para prevenir corrida no Silent Refresh (concorrência de 401) | Shadow Judge & Sec Reviewer | **ACEITAR** | Evita tempestade de requisições de refresh que causam invalidação prematura de refresh token rotativo. | Adicionado requisito `REQ-FE-05` e especificado algoritmo singleton com fila de Promise em `auth-interceptor.ts`. |
| **7** | Manter Access Token estritamente em memória volátil, proibindo `localStorage` e `sessionStorage` | Sec Reviewer | **ACEITAR** | Mitiga roubo de credenciais via XSS conforme o padrão de segurança `NFR-SEC-05`. | Invariante red line consolidada em `SPEC.md`, `ADR-009` e na matriz de testes de aceitação. |
| **8** | Proibição de filtragem client-side como mecanismo de privacidade intra-tenant | Sec Reviewer | **ACEITAR** | Controles de privacidade e multi-tenancy são atribuições exclusivas do PostgreSQL RLS e do middleware. | Invariante formalizada no `SPEC.md` (Zero Client Visibility Inference). |
| **9** | Sanitização obrigatória de descrições e proibir `dangerouslySetInnerHTML` | Sec Reviewer | **ACEITAR** | Descrições de extratos bancários brutos externos contêm caracteres não confiáveis que podem induzir XSS. | Adicionada diretriz explícita no guia de componentes e no linter do frontend. |
| **10** | Trocar stack de React para Svelte 5 devido a bundle ligeiramente menor | Discussão Aberta | **REJEITAR** | O ecossistema de geração tipada automática a partir de OpenAPI e componentes de tabela/gráficos prontos em React acelera a entrega do mantenedor solo. | Mantida a recomendação de React 19 + Vite com pontuação 4.63 vs 4.55 de SvelteKit. |

---

## 2. Incorporação Estrutural nos Entregáveis

As decisões acima foram diretamente transcritas para:
1. `docs/adrs/ADR-009-frontend-architecture.md`: Reescrito sem nenhuma referência ao postmortem ou legado, justificando a escolha por mérito (deploy em binário único, integração OpenAPI, DX para single-maintainer e custo $0).
2. `docs/specs/frontend.md`: Reescrita com sincronização estrita aos endpoints e DTOs reais em Go, clarificando o envelope `items`/`pagination` e o tratamento de erros RFC 7807.
3. `.flow/tasks/gh-36/SPEC.md`: Especificação técnica de alta fidelidade com 38 requisitos no padrão EARS, arquitetura de camadas e tipos TypeScript.
4. `.flow/tasks/gh-36/PLAN.md`: Plano de implementação dividido em Fase A (especificação), Fase B1 (MVP com endpoints existentes) e Fase B2 (integração de auth e analytics).
5. `.flow/tasks/gh-36/ISSUE_UPDATE.md`: Descrição da Issue #36 amadurecida para guiar a implementação sem ambiguidades.

---

## 3. Veredito Final

O Comitê de Revisão declara a **Issue #36 tecnicamente amadurecida**, a **PR #37 devidamente corrigida e auditada**, e o projeto plenamente apto para seguir para o ciclo de implementação sem riscos arquiteturais ocultos.
