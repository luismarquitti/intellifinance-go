# Dossiê Arquitetural: PRFlow & Integração com Agent-Skills

Este documento consolida a análise detalhada da arquitetura do **PRFlow** (anteriormente referenciado como *DevFlow*), seus diagramas de fluxo e máquina de estados em modelo agnóstico, bem como o mapeamento sistemático de integração com os artefatos do repositório **`tech-leads-club/agent-skills`**.

---

## Parte 1: Arquitetura da Solução PRFlow

O **PRFlow** é um framework de engenharia de software agnóstico a LLM e a harness, projetado para a orquestração autônoma do ciclo de vida de Pull Requests (*SDLC Agent*). Sua premissa fundamental não é apenas a geração de código (*raw code generation*), mas a **entrega auditável de software com tolerância zero a aprovações em única passagem (*zero-trust in single-pass LLM*)**.

### 1.1 Camadas Lógicas Desacopladas

```
┌────────────────────────────────────────────────────────────────────────┐
│                        Camada de Gatilho / Interface                   │
│         (CLI Local / Eventos Git / Webhooks / Comentários de Issue)    │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼────────────────────────────────────┐
│                    Orquestrador Central de Estados                     │
│    (Spec → Planning → Implementation → Multi-Review & Fix → Sync)      │
└───────────────────┬───────────────────────────────┬────────────────────┘
                    │                               │
┌───────────────────▼───────────────┐ ┌─────────────▼────────────────────┐
│       Comitê Especializado        │ │       Mecanismos de Defesa       │
│           de Revisores            │ │       e Validação Mecânica       │
│ - Code Reviewer                   │ │ - Mechanical Verification (Test) │
│ - Silent Failure Hunter           │ │ - Shadow Pass Review             │
│ - Comment & Intent Analyzer       │ │ - Bounded Iteration Fixer        │
│ - Type & Design Analyzer          │ │ - Deferral Manifest Matcher      │
│ - Test Coverage Analyzer          │ │                                  │
└───────────────────┬───────────────┘ └─────────────┬────────────────────┘
                    │                               │
┌───────────────────▼───────────────────────────────▼────────────────────┐
│                    Ciclo de Aprendizado & Feedback                     │
│      (Reflection Logs, Effectiveness Traces, Weekly Retrospective)     │
└────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Componentes e Mecanismos Centrais

1. **Orquestrador de Fases (Lifecycle Engine):**
   * **Spec:** Decompõe requisitos não estruturados em uma especificação técnica determinística.
   * **Implement:** Cria branch isolada, planeja arquitetura contra o repositório existente (*brownfield*), implementa código e testes, gerando um Draft PR.
   * **Review-and-Fix Loop:** Loop iterativo com teto de execuções (*default: 5 rounds*). Aplica patches automáticos até atingir aprovação ou saturação de escopo.
   * **Documentation & Delivery:** Atualização de documentação viva, changelog e conversão de Draft para *Ready for Review*.

2. **Painel Multiespecialista de Revisão (Specialized Review Panel):**
   * Em vez de uma avaliação monolítica, o diff é submetido a agentes com prompts de escopo fechado:
     * `code-reviewer`: Qualidade de código, padrões de projeto e manutenibilidade.
     * `silent-failure-hunter`: Exceções mascaradas, timeouts implícitos, fallbacks perigosos e edge cases não tratados.
     * `comment-analyzer`: Divergência entre intenção expressa em comentários/documentação e execução real.
     * `type-design-analyzer`: Integridade tipada, contratos de API e robustez estrutural.
     * `pr-test-analyzer`: Validação da completude da suíte de testes vs. caminhos de execução do diff.

3. **Mecanismo de "Shadow Pass" (Auditoria Estruturalmente Independente):**
   * Se o painel aprova o código, a aprovação **não** é aceita cegamente.
   * Um segundo revisor independente (*Shadow Pass*) roda sem o viés contextual do loop anterior. Se o shadow pass encontrar inconsistências, o estado regride para nova iteração de correção.

4. **Gerenciamento de Deferrals (Desacoplamento de Débito Técnico):**
   * Problemas fora do escopo estrito do PR não travam a entrega: o sistema isola apontamentos, gera um *Deferrals Manifest* e abre automaticamente novas issues de débitos/melhorias, desonerando o PR atual sem perder rastreabilidade.

5. **Ciclo Retrospectivo Contínuo (Self-Improving Loop):**
   * Cada execução registra decisões, suposições e passos descartados (*effectiveness trace*).
   * Uma retrospectiva semanal analisa correções pós-merge realizadas por humanos para identificar falhas do comitê de IA e sugerir refinamento das regras via issues/PRs.

---

## Parte 2: Diagramas do PRFlow

### 2.1 Diagrama de Fluxo Operacional (End-to-End)

```
[Início: Requisito / Issue]
          │
          ▼
   [Fase 1: /prflow:spec]
          │ ──► Entrevista de clarificação e exploração de contexto
          │ ──► Criação/Alinhamento de Issue com critérios de aceite
          ▼
 [Fase 2: /prflow:implement]
          │ ──► Criação de Branch & Planejamento Arquitetural
          │ ──► Escrita de Código e Testes Unitários/Integração
          │ ──► Abertura de Draft PR
          ▼
 [Fase 3: Loop /prflow:review-and-fix]
          │
    ┌─────┴──────────────────────────────────────────────────────┐
    │                                                            │
    ▼                                                            ▼
[Compilação & Testes Mecânicos]                      [Painel Multiespecialista]
(Passa no linter/testes da CLI?)                     (Análise dos 5 revisores)
    │                                                            │
    └─────────────────────┬──────────────────────────────────────┘
                          │
                  (Existem Falhas?)
                     /         \
          [Sim: Falhas]       [Não: Aprovado]
                │                     │
      (Limite de Loops?)              ▼
          /         \        [Fase 4: Shadow Pass]
   [Não atingido]  [Atingido] (Revisor isolado confirma aprovação?)
         │              │             /               \
         ▼              │    [Shadow Reprova]   [Shadow Aprova]
[Gera Correções &       │           │                  │
 Aplica Patches]        │           │                  │
         │              │           │                  │
         └──────────────┼───────────┘                  │
                        ▼                              ▼
             [Gera Deferral Manifest &        [Fase 5: Finalização]
              Issues de Débito Técnico]       ├── Sincroniza Documentação
                        │                     ├── Remove status Draft do PR
                        └────────────────────►└── Entrega PR para Revisão Humana
                                                       │
                                                       ▼
                                            [Revisão Final Humana & Merge]
```

### 2.2 Diagrama de Estados (Finite State Machine)

```
                  ┌──────────────┐
                  │     IDLE     │
                  └──────┬───────┘
                         │ trigger(/prflow:spec)
                         ▼
                  ┌──────────────┐
                  │  SPECIFYING  │
                  └──────┬───────┘
                         │ spec confirmed
                         ▼
                  ┌──────────────┐
                  │ IMPLEMENTING │◄─────────────────────┐
                  └──────┬───────┘                      │
                         │ build & tests pass           │
                         ▼                              │
       ┌──────────► REVIEWING ◄──────────┐              │
       │                 │               │              │
       │                 ▼               │              │
       │          (Avalia Diff)          │              │
       │           /         \           │              │
       │    [Reprovado]   [Aprovado]     │              │
       │        │              │         │              │
       │        ▼              ▼         │              │
       │   ┌────────┐    ┌───────────┐   │              │
       │   │ FIXING │    │ SHADOWING │   │              │
       │   └───┬────┘    └─────┬─────┘   │              │
       │       │               │         │              │
       │       │ patches       │ veto    │              │
       │       │ applied       └─────────┘              │
       │       │                                        │
       └───────┴───────────────┐                        │
                 (Max Loops    │                        │
                  Excedido)    │ shadow approved        │
                               ▼                        │
                         ┌───────────┐                  │
                         │ SYNCING   │ (Docs, Changelog)│
                         └─────┬─────┘                  │
                               │                        │
                               ▼                        │
                         ┌───────────┐                  │
                         │ COMPLETED │                  │
                         └─────┬─────┘                  │
                               │                        │
                               ▼                        │
                         ┌───────────┐                  │
                         │ MERGED /  ├──────────────────┘ (se houver regressão
                         │ CLOSED    │  na branch ou novos commits)
                         └───────────┘
```

#### Tabela de Invariantes e Transições de Estado

| Estado Atual | Gatilho / Condição | Próximo Estado | Invariante de Segurança |
| :--- | :--- | :--- | :--- |
| `IMPLEMENTING` | Testes locais/mecânicos falham | `IMPLEMENTING` | Não envia código quebrado para a fase de revisão. |
| `REVIEWING` | Falhas de código/segurança detectadas | `FIXING` | Limite de iterações monitorado; registra *findings* no histórico. |
| `FIXING` | Atingido o limite de iterações (`max_iterations`) | `SYNCING` | Falhas não críticas são empacotadas como *deferrals* e convertidas em novas issues. |
| `REVIEWING` | Painel aprova sem ressalvas bloqueantes | `SHADOWING` | Nenhuma aprovação em single-pass é definitiva. |
| `SHADOWING` | Shadow detecta brecha / regressão | `REVIEWING` | Quebra o ciclo de confirmação prematura. |
| `SHADOWING` | Shadow valida e assina o PR | `SYNCING` | Estado pronto para geração de docs e abertura para os humanos. |

---

## Parte 3: Mapeamento de Integração com `tech-leads-club/agent-skills`

O repositório **`tech-leads-club/agent-skills`** fornece um ecossistema tipado de *skills* com contratos estritos, schemas de validação e infraestrutura MCP. Abaixo está a correlação direta entre os artefatos disponíveis e os módulos do PRFlow.

### 3.1 Matriz de Mapeamento

| Artefato / Skill (`agent-skills`) | Categoria Nativa | Ponto de Injeção no PRFlow | Função Arquitetural no Pipeline |
| :--- | :--- | :--- | :--- |
| **`tlc-spec-driven`** | `development` | `/prflow:spec` & `/prflow:implement` (Planner) | Substitui ou calibra o *interview/spec engine* com requisitos em notação EARS, controle de estado (`.specs/STATE.md`) e particionamento em tarefas atômicas com verificação determinística. |
| **`security-best-practices`** | `security` | `/prflow:review-and-fix` (Painel) | Atua como revisor de segurança especializado para escanear vulnerabilidades em TypeScript/Python/Go, gerando o *findings payload* para o fix loop. |
| **`web-quality-audit`** / **`web-best-practices`** | `quality` | `/prflow:review-and-fix` (Painel) | Injeta critérios estritos de acessibilidade, web performance e linting no painel de revisores antes do loop de fix. |
| **`playwright-skill`** | `automation` | `/prflow:implement` (Mechanical Test) | Executa validações mecânicas E2E e regressão visual no código gerado antes do PR ser submetido ao painel de LLM. |
| **`docs-writer`** | `development` | `/prflow:docs` (Sync Phase) | Garante atualização de documentação técnica, alinhamento de links em docs, guias de estilo e changelog na etapa final. |
| **`@tech-leads-club/agent-skills-mcp`** | *Infraestrutura MCP* | Runtime Sub-Agent Execution | Fornece aos sub-agentes do PRFlow acesso sob demanda (*progressive disclosure*) a referências, scripts e templates durante o loop de correção. |

---

### 3.2 Análise dos Artefatos de Maior Impacto

#### 1. `tlc-spec-driven` $\to$ Core de Especificação e Planejamento (`/prflow:spec`)
* **Mecanismo:** Executa metodologia *Spec-Driven Development* em 4 fases adaptativas (*Specify $\to$ Design $\to$ Tasks $\to$ Execute*), persistindo decisões em `.specs/STATE.md` e aprendizados em `lessons.md`.
* **Sinergia:** O PRFlow converte requisitos vagos em specs técnicas. O `tlc-spec-driven` traz validações determinísticas via scripts Python (`validate_spec.py`, `validate_tasks.py`), garantindo que o PRFlow só avance para `/prflow:implement` quando critérios de aceite e tarefas atômicas estiverem formalmente validados.

#### 2. `security-best-practices` $\to$ Membro do Painel de Revisores (`/prflow:review`)
* **Mecanismo:** Análise sistemática de brechas arquiteturais de segurança e código inseguro (*threat modeling*, sanitização, autenticação, injeção e controle de acesso) com severidades classificadas (*Critical, High, Medium*).
* **Sinergia:** Integra-se diretamente ao comitê de revisores especialistas do PRFlow. Suas falhas não resolvidas dentro do teto de iterações alimentam de forma estruturada o *Deferrals Manifest*, gerando novas issues de segurança rastreáveis.

#### 3. `playwright-skill` $\to$ Corroboração Mecânica (*Mechanical Corroboration*)
* **Mecanismo:** Automação de browser com Playwright (geração e execução de testes de ponta a ponta, captura de screenshots, validação de fluxos de tela).
* **Sinergia:** Garante que a etapa de implementação gere e execute testes E2E reais via CLI local antes de despachar o código para a fase de revisão por LLM, reforçando a regra de *zero-trust*.

#### 4. `docs-writer` $\to$ Sincronização de Documentação (`/prflow:docs`)
* **Mecanismo:** Atualização orientada a regras de arquivos `README.md`, documentação em `/docs`, changelog e validação de hiperlinks.
* **Sinergia:** Executado na fase de fechamento (`SYNCING`) para manter a documentação viva e perfeitamente coerente com o diff introduzido, antes de desmarcar o PR como *draft*.

#### 5. `@tech-leads-club/agent-skills-mcp` $\to$ Camada de Integração Dinâmica
* **Mecanismo:** Servidor Model Context Protocol (MCP) com suporte a busca progressiva (`search_skills` $\to$ `read_skill` $\to$ `fetch_skill_files`).
* **Sinergia:** Elimina overhead de contexto. O orquestrador do PRFlow opera leve e, quando o loop de correção encontra um problema de nicho (ex: Next.js, Docker, concorrência em Go), a skill correspondente é carregada dinamicamente sob demanda.

---

### 3.3 Configuração e Provisionamento

Para instalar e disponibilizar essas skills no ambiente de trabalho:

```bash
# Instalação pontual via CLI interativo
npx @tech-leads-club/agent-skills install -s tlc-spec-driven security-best-practices docs-writer
```

Para integração contínua e consumo via MCP:

```json
{
  "mcpServers": {
    "agent-skills": {
      "command": "npx",
      "args": ["-y", "@tech-leads-club/agent-skills-mcp"]
    }
  }
}
```