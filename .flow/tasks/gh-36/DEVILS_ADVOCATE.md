# Auditoria Crítica Adversarial (Devil's Advocate) — Frontend IntelliFinance

- **Document ID**: AUDIT-001-DEVILS-ADVOCATE
- **Auditor**: Shadow Judge (`04-devils-advocate`)
- **Data**: 2026-10-17
- **Alvos da Auditoria**:
  - `docs/adrs/ADR-009-frontend-architecture.md` (versão original da PR #37)
  - `docs/specs/frontend.md` (versão original da PR #37)
  - `.flow/tasks/gh-36/SPEC.md` & `.flow/tasks/gh-36/PLAN.md`

---

## 1. Sumário Executivo do Ataque

A proposta original contida na PR #37 (`ADR-009` e `docs/specs/frontend.md`) sofre de quatro deficiências graves que a tornavam perigosa ou irrealizável se aprovada sem refinamento:
1. **Falácia do Trauma Retrospectivo**: Todo o arcabouço decisório de `ADR-009` apoiava-se no medo das falhas de build do projeto legado, violando diretamente a nova diretriz do operador e enfraquecendo a justificativa técnica das escolhas.
2. **Alucinação de Contratos de Backend**: A especificação assumia como "prontos" endpoints de autenticação (`/api/v1/auth/login`), agregações de painel (`/api/v1/analytics/*`) e ingestão assíncrona (`/api/v1/ingestion/*`) que **simplesmente não existem no código-fonte Go atual**. Se a equipe iniciasse a codificação da UI imediatamente, a aplicação não passaria do primeiro `fetch`.
3. **Inviabilidade do Escopo de MVP**: O escopo pretendido tentava entregar cinco telas completas, mas o backend atual suporta unicamente os recursos de Contas, Categorias e Transações. Faltava um estagiamento claro (Fase B1 vs. Fase B2).
4. **Vulnerabilidade de Concorrência de Sessão**: A estratégia de renovação silenciosa de token descrita no ADR-009 original omitia a proteção contra rajadas concorrentes de requisições 401, o que geraria invalidação em cascata de refresh tokens.

Abaixo, detalho cada apontamento adversarial com citação precisa de arquivos e linhas.

---

## 2. Apontamentos Críticos Detalhados

### Ponto 1: Justificativa Falaciosa Baseada no Legado (ADR-009 Original)
- **Localização**: `docs/adrs/ADR-009-frontend-architecture.md`: Linhas 24, 37, 63, 81.
- **Ataque**:
  - O ADR justificava a rejeição do Next.js afirmando na Linha 81: *"Next.js reproduz exatamente a classe de problema que matou o frontend legado: módulos que resolvem em um contexto de renderização e não no outro"*.
  - A adoção do Tailwind era justificada na Linha 84 por: *"O protótipo em design/stitch/... já é Tailwind... Adotar Tailwind significa portar tokens, não redesenhar"*.
  - **Crítica Adversarial**: Essa argumentação é intelectualmente frágil e defensiva. O operador revogou explicitamente o legado como restrição. Se Next.js não serve, não é porque o legado quebrou com Vite/Apollo há um ano; é porque Next.js exige um runtime Node.js em produção, o que viola frontalmente a compilação em binário estático Go (`embed.FS`) e a meta de contêiner distroless <35MB a custo $0.00 (`NFR-REL-02`, `NFR-REL-03`). A decisão precisava ser resgatada e reescrita por mérito contemporâneo.

### Ponto 2: O Blefe da Autenticação Inexistente
- **Localização**: `docs/specs/frontend.md`: Linhas 41-43, 93-109; `docs/adrs/ADR-009-frontend-architecture.md`: Linhas 126-137.
- **Ataque**:
  - A especificação descreve com riqueza de detalhes a tela de login consumindo `POST /api/v1/auth/login` e o interceptor de refresh em `POST /api/v1/auth/refresh`.
  - **A Realidade do Código**: Executando `grep -rniE "auth|login|refresh|logout" internal/ --include="*.go"`, constatamos que **não há um único handler de login ou emissão de JWT implementado no backend**.
  - Mais crítico ainda: o middleware que protege a API hoje em `internal/adapters/inbound/http/middleware/tenant.go:56` rejeita qualquer requisição sem o header `X-Tenant-ID` com HTTP 401:
    ```go
    tenantHeader := r.Header.Get(HeaderTenantID)
    if tenantHeader == "" {
        prob := problem.NewUnauthorized(r.URL.Path, "Missing required X-Tenant-ID header.")
        problem.Render(w, r, prob)
        return
    }
    ```
  - **Consequência**: Um desenvolvedor frontend que seguisse a spec original bateria de frente contra uma parede de 401 imediata. A spec precisava explicitar essa lacuna e definir formalmente um modo de compatibilidade/showcase para desenvolvimento local que injete `X-Tenant-ID` até que o backend de JWT seja concluído.

### Ponto 3: A Falha Fantasma das Agregações no Dashboard
- **Localização**: `docs/specs/frontend.md`: Linhas 44-45, 114-118.
- **Ataque**:
  - A spec afirma: *"Decisão de arquitetura: agregações são calculadas no servidor (PRD §4.7, FR-7.4). O cliente nunca soma valores monetários"*. Ao mesmo tempo, define que a tela de Dashboard consome `GET /api/v1/analytics/cash-flow` e `GET /api/v1/analytics/by-category`.
  - **A Realidade do Código**: Em `internal/adapters/inbound/http/router.go`, não há nenhuma rota `/analytics/*`.
  - **A Armadilha**: Se o cliente está proibido de calcular somatórios (o que é uma excelente regra contábil para evitar `0.1 + 0.2 = 0.30000000000000004`), e o servidor não possui os endpoints de métricas de cash flow prontos, a tela de Dashboard no MVP seria inoperável ou forçaria o desenvolvedor a quebrar a invariante e somar floats no JavaScript no desespero de entregar a tela.
  - **Correção Exigida**: O plano de implementação deve desacoplar o MVP: a Fase B1 deve focar nas telas cujos dados já existem (Transações e Contas), e a tela de Dashboard deve ter tratamento explícito de estados ou aguardar os endpoints de analytics na Fase B2.

### Ponto 4: Assunção Incorreta da Rota de Sobrescrita de Categoria
- **Localização**: `docs/specs/frontend.md`: Linhas 48, 142.
- **Ataque**:
  - O documento original referenciava a rota `PATCH /api/v1/transactions/{id}/category` como endpoint de sobrescrita.
  - **A Realidade do Código**: Em `internal/adapters/inbound/http/router.go:82`, a rota registrada é `PATCH /transactions/{id}` vinculada a `handlers.TransactionHandler.UpdateTransaction`. Não existe sub-recurso `/category`.
  - Em `internal/adapters/inbound/http/dto/transaction.go:124`, `UpdateTransactionRequest` aceita `category_id *string`.
  - **Correção Exigida**: A spec deve orientar o cliente a consumir a rota real `PATCH /api/v1/transactions/{id}` enviando `{ "category_id": "uuid" }`, evitando criar um falso pré-requisito de backend desnecessário.

### Ponto 5: Divergência Crítica de Envelopes nos Documentos de Referência
- **Localização**: Conflito entre `docs/adrs/ADR-002-api-architecture.md` §6 e `docs/specs/api.md` §2.1.
- **Ataque**:
  - `ADR-002` (Linhas 131-137) descreve o envelope paginado como `{"data": [...], "meta": {"page": 1, ...}}`.
  - `docs/specs/api.md` (Linhas 33-47) e o código real em `internal/adapters/inbound/http/dto/pagination.go` descrevem o envelope como `{"items": [...], "pagination": {"page": 1, ...}}`.
  - Um frontend gerado a partir do ADR-002 quebraria ao ler `.data`. A spec deve fixar taxativamente que `SPEC-002-API` e os DTOs reais em Go (`items` e `pagination`) são a única fonte da verdade autoritativa.

### Ponto 6: Risco de Tempestade de Refreshes Concorrentes (Silent Refresh Race Condition)
- **Localização**: `docs/adrs/ADR-009-frontend-architecture.md`: Linhas 134-135; `docs/specs/frontend.md`: Linhas 231-232.
- **Ataque**:
  - Ambos os documentos dizem: *"Um 401 de qualquer endpoint dispara um refresh silencioso e repete a chamada"*.
  - **Cenário Real de Falha**: O usuário carrega a página `/dashboard`, disparando 3 requisições simultâneas: `GET /analytics/cash-flow`, `GET /analytics/by-category` e `GET /accounts`. Se o token expirar, todas as 3 retornam 401 quase no mesmo milissegundo.
  - Sem um mecanismo explícito de Mutex/Queue na camada HTTP do client, a aplicação disparará **três requisições concorrentes de `/auth/refresh`**. Em sistemas que utilizam rotação estrita de refresh token (onde o refresh token antigo é invalidado no momento do uso), a segunda e terceira requisições falharão com token revogado, deslogando o usuário involuntariamente (*false logout*).
  - **Correção Exigida**: A especificação do cliente deve exigir um lock/singleton de refresh que pause as requisições concorrentes e distribua o novo token para toda a fila após um único refresh bem-sucedido.

---

## 3. Conclusão do Shadow Judge

A especificação proposta em `gh-36` só se torna apta para implementação após:
1. Reescrita completa da fundamentação de stack sem referência ao legado;
2. Formalização do inventário real de endpoints e criação do modo Showcase para desacoplamento de dependências bloqueantes;
3. Separação explícita da implementação em Fase B1 (telas funcionais com a API existente) e Fase B2 (telas dependentes de novas rotas de backend);
4. Definição do algoritmo de fila de refresh no interceptor de API.
