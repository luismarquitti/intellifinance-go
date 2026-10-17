# Avaliação Técnica Comparativa de Stack de Frontend — IntelliFinance

- **Document ID**: EVAL-001-FRONTEND-STACK
- **Data**: 2026-10-17
- **Autor**: Frontend Architect (`frontend-architect`)
- **Status**: Aprovado pelo Comitê de Revisão
- **Referência**: ADR-009 (Arquitetura do Frontend)

---

## 1. Princípios e Diretriz de Avaliação por Mérito

Conforme a **Diretriz Direta do Operador**:
> *"Quanto ao frontend, nada precisa seguir o que existia no projeto legado, podemos definir tudo novo para melhorar o desenvolvimento."*

Todas as justificativas históricas e vieses retrospectivos (como falhas de build do projeto legado documentadas no postmortem ou adoção acidental de dependências prévias) foram **estritamente expurgadas desta análise**. O protótipo visual em `design/stitch/` é tratado como insumo consultivo e não como imposição técnica.

A avaliação abaixo é conduzida com base em **mérito técnico contemporâneo**, alinhada aos requisitos arquiteturais inegociáveis do IntelliFinance:
1. Arquitetura Hexagonal com consumo estrito de API REST spec-first (OpenAPI 3.0.3);
2. Custo operacional $0.00 em repouso (Dual-Target: Homelab `lm-claw` e GCP Cloud Run scale-to-zero);
3. Entrega em binário Go estático único (`embed.FS`) sem dependência de runtime Node.js em produção;
4. Alta produtividade de desenvolvimento (DX) para equipe enxuta (manutenção por engenheiro solo);
5. Experiência de usuário fluida para fluxos altamente reativos (polling assíncrono de jobs de ingestão, fila de revisão de IA com feedback otimista e filtros dinâmicos).

---

## 2. Critérios de Avaliação e Ponderação

| ID | Critério | Peso | Justificativa |
|:---|:---|:---:|:---|
| **C1** | **Build Determinístico & Simplicidade de Toolchain** | **20%** | O processo de CI/CD deve ser estritamente reproduzível em máquina limpa. Uma única árvore de dependências sem conflitos de peer dependencies ou transições complexas de módulos client/server. |
| **C2** | **Aderência ao Contrato OpenAPI 3.0 & Tipagem Estrita** | **15%** | A API REST é a fonte da verdade (`SPEC-002-API`). O ecossistema deve suportar geração automática de clientes tipados (`openapi-typescript` / `openapi-fetch`) sem drift manual. |
| **C3** | **Deploy em Binário Único Go (`embed.FS`) & Custo $0** | **20%** | Conformidade com `NFR-REL-02` e `NFR-REL-03`. O artefato de produção deve ser compilado em assets estáticos embutidos no binário Go, preservando o contêiner distroless (<35MB) sem processo Node.js auxiliar. |
| **C4** | **Velocidade de Entrega & DX para Desenvolvedor Solo** | **15%** | Curva de aprendizado, ergonomia de sintaxe, documentação madura, disponibilidade de componentes acessíveis e clareza no gerenciamento de estado. |
| **C5** | **Suporte a Alta Interatividade & Reatividade Assíncrona** | **15%** | Fila de revisão de categorização de IA (`PATCH` com rollback otimista), upload multipart com barra de progresso, polling exponencial de jobs de ingestão e gráficos interativos de fluxo de caixa. |
| **C6** | **Desempenho no Cliente & Time-to-First-Screen** | **10%** | Tamanho de bundle inicial reduzido, tempo de inicialização rápido para recrutadores avaliando o showcase (`PRD §6.3`) em conexão modesta. |
| **C7** | **Testabilidade & Longevidade do Ecossistema** | **5%** | Facilidade de testes de componentes isolados (Vitest/Playwright) e suporte de longo prazo por comunidade ativa e estável. |

**Escala de Pontuação**: 1 (Inadequado / Bloqueante) a 5 (Excelente / Referência da Indústria).  
**Pontuação Máxima Ponderada**: 5.00 pontos.

---

## 3. Matriz Comparativa de Opções

Foram analisadas seis alternativas arquiteturais viáveis:

1. **Opção 1: React 19 + Vite 6/7 + TypeScript + TanStack Query + Tailwind CSS** (SPA Estática Embutida)
2. **Opção 2: Next.js 15 (App Router / SSR + RSC) + TypeScript** (Servidor Node.js Separado)
3. **Opção 3: Svelte 5 (Runes) + SvelteKit (Static Adapter) + TypeScript** (SPA Estática Embutida)
4. **Opção 4: Vue 3 + Vite + TypeScript + Pinia / TanStack Query + Tailwind CSS** (SPA Estática Embutida)
5. **Opção 5: Angular 19 (Standalone Components + Signals) + TypeScript** (SPA Estática Embutida)
6. **Opção 6: Go `html/template` + htmx + Alpine.js + Tailwind CSS** (SSR Puro sem Framework JS)

### 3.1 Tabela de Pontuação

| Critério (Peso) | React 19 + Vite (Opt 1) | Next.js 15 (Opt 2) | Svelte 5 Kit (Opt 3) | Vue 3 + Vite (Opt 4) | Angular 19 (Opt 5) | Go + htmx (Opt 6) |
|:---|:---:|:---:|:---:|:---:|:---:|:---:|
| **C1: Build & Toolchain (20%)** | 4.5 | 3.0 | 4.8 | 4.7 | 4.0 | 5.0 |
| **C2: Contrato OpenAPI (15%)** | 5.0 | 4.5 | 4.0 | 4.2 | 3.8 | 2.0 |
| **C3: Deploy Go `embed.FS` (20%)** | 5.0 | 1.5 | 5.0 | 5.0 | 4.5 | 5.0 |
| **C4: Velocidade / DX Solo (15%)** | 4.5 | 3.5 | 4.3 | 4.4 | 3.0 | 3.5 |
| **C5: Interatividade / Reatividade (15%)** | 4.8 | 4.5 | 4.6 | 4.5 | 4.2 | 2.5 |
| **C6: Performance / Bundle (10%)** | 4.0 | 3.5 | 4.9 | 4.5 | 3.2 | 5.0 |
| **C7: Testabilidade & Ecossistema (5%)** | 4.8 | 4.0 | 3.8 | 4.2 | 4.5 | 3.0 |
| **TOTAL PONDERADO** | **4.63** | **3.05** | **4.55** | **4.52** | **3.71** | **3.75** |
| **Classificação** | **1º Lugar** | 6º Lugar | **2º Lugar** | 3º Lugar | 5º Lugar | 4º Lugar |

---

## 4. Análise Crítica Detalhada por Opção

### Opção 1: React 19 + Vite 6/7 + TypeScript + TanStack Query + Tailwind CSS (Recomendada)
- **Vantagens de Mérito**:
  - **Ecosistema OpenAPI maduro**: `openapi-typescript` e `openapi-fetch` integram perfeitamente com TanStack Query v5, gerando tipos estritos que impedem drift entre backend e frontend em tempo de compilação.
  - **Empacotamento Estático Perfeito**: A saída `vite build` é um diretório de arquivos estáticos (`index.html`, hashes JS/CSS) que é embutido diretamente no binário Go via `//go:embed dist/*`, mantendo a promessa de container único distroless (<35MB) sem Node.js em produção.
  - **Gerenciamento de Estado Assíncrono Nativo**: TanStack Query fornece invalidação inteligente de cache, mutações otimistas com rollback automático (essencial para a tela de revisão de IA `PATCH /transactions/{id}`), e controle declarativo de polling com backoff para o acompanhamento dos jobs de ingestão.
  - **Componentes Acessíveis Desacoplados**: Suporte imediato a primitives headless como Radix UI / shadcn/ui, garantindo conformidade com WCAG AA sem lock-in.
- **Desvantagens & Mitigações**:
  - *Tamanho do runtime*: React 19 adiciona overhead ligeiramente superior ao Svelte 5. *Mitigação*: Code-splitting por rota via `React.lazy` mantém o bundle inicial abaixo de 120KB gzipped.
  - *Toolchain Node para desenvolvimento*: Necessita npm/Node apenas na máquina do desenvolvedor e no step de CI; a compilação Go subsequente não requer Node.js.

### Opção 2: Next.js 15 (App Router / SSR)
- **Vantagens de Mérito**:
  - Excelente para portais públicos com necessidade de SEO profundo e Server Components.
- **Por que foi desqualificada**:
  - **Incompatibilidade Arquitetural com Single-Binary**: Next.js com App Router e Server Components exige um servidor Node.js/Bun em execução contínua ou deploy em plataforma serverless de terceiros (Vercel). Isso quebra o requisito `NFR-REL-02` (execução bare-metal no Homelab `lm-claw` como binário estático) e viola o limite de contêiner de 35MB em distroless.
  - Se configurado com `output: 'export'` (estático puro), perde-se Server Actions, SSR e middleware do Next.js, tornando a complexidade da toolchain injustificável frente ao Vite.
  - Pontuação: **3.05** (desqualificada no C3).

### Opção 3: Svelte 5 (Runes) + SvelteKit (Static Adapter) (Forte Concorrente)
- **Vantagens de Mérito**:
  - Desempenho excepcional de compilação e runtime com reatividade granular via Runes (`$state`, `$derived`).
  - Bundle diminuto (frequentemente <40KB gzipped), ideal para conexões móveis.
  - O `@sveltejs/adapter-static` produz arquivos estáticos perfeitos para `embed.FS`.
- **Por que ficou em 2º Lugar**:
  - O ecossistema de geração de clientes tipados diretamente de contratos OpenAPI 3.0 é menos maduro e menos testado em larga escala que a combinação `openapi-typescript` + TanStack Query em React.
  - A disponibilidade de bibliotecas de componentes headless acessíveis prontas para uso corporativo (gráficos financeiros, datatables complexas com virtualização) é substancialmente menor, exigindo maior escrita de código boilerplate pelo mantenedor solo.
  - Pontuação: **4.55** (excelente opção técnica, mas com maior custo de desenvolvimento inicial de UI).

### Opção 4: Vue 3 + Vite + TypeScript + Pinia / TanStack Query
- **Vantagens de Mérito**:
  - Excelente ergonomia com Composition API (`<script setup>`) e integração fluida com Vite e `embed.FS`.
  - Boa biblioteca de componentes e performance de runtime.
- **Por que ficou em 3º Lugar**:
  - Pontuação muito próxima de React (4.52 vs 4.63), mas a integração `openapi-fetch` + TanStack Query possui menos documentação e patterns estabelecidos em Vue do que em React.

### Opção 5: Angular 19 (Standalone Components + Signals)
- **Vantagens de Mérito**:
  - Framework altamente opinado, bateria inclusa, signals reativos modernos.
- **Por que ficou em 5º Lugar**:
  - Toolchain pesada, curva de verbosidade acentuada para um projeto de engenheiro solo, e complexidade desnecessária para o escopo de 5 telas de fintech.
  - Pontuação: **3.71**.

### Opção 6: Go `html/template` + htmx + Tailwind CSS (SSR Puro)
- **Vantagens de Mérito**:
  - Zero dependência de ecossistema JavaScript no runtime; binário 100% nativo Go.
  - Excelente para formulários CRUD simples e visualizações tabulares estáticas.
- **Por que foi desqualificada para o MVP**:
  - **Falta de Tipagem de Contrato no Cliente**: O backend compila structs Go, mas não há validação estrita em tempo de build contra a spec OpenAPI consumida pelo client.
  - **Inadequada para Alta Interatividade Contábil**: O fluxo de acompanhamento de jobs de ingestão (upload com progresso, polling de status, cálculo de tempo decorrido), a fila de categorização de IA com transições otimistas e atalhos de teclado, e os gráficos analíticos de fluxo de caixa exigem scripts imperativos complexos em htmx/Alpine, gerando código desorganizado e difícil de manter e testar.
  - Pontuação: **3.75**.

---

## 5. Recomendação Final e Justificativa de Mérito

O Comitê e o Arquiteto de Frontend recomendam formalmente a **Opção 1: React 19 + Vite + TypeScript + TanStack Query + Tailwind CSS**:

1. **Aderência ao Modelo de Deploy**: Gera ativos puramente estáticos em `web/dist/`, que são embutidos via `embed.FS` no executável Go de `cmd/api`. Mantém o contêiner distroless em **<35MB** e o custo operacional em **$0.00**.
2. **Garantia Anti-Drift**: Utiliza `openapi-typescript` para compilar `api/openapi/openapi.yaml` em definições TypeScript estritas. Qualquer alteração ou quebra de contrato na API quebra a compilação do frontend no CI imediatamente.
3. **Resiliência e UX em Finanças**: O TanStack Query v5 resolve nativamente as necessidades de caching, retry com backoff exponencial para polling de ingestão, e atualizações otimistas para a fila de revisão de IA com reversão limpa em caso de erro RFC 7807 (422/500).
4. **Independência Total do Legado**: A decisão fundamenta-se nas propriedades estruturais da ferramenta para as necessidades atuais do IntelliFinance, não em traumas ou restrições do projeto anterior.

---

## 6. Riscos e Mitigações

| Risco Identificado | Severidade | Estratégia de Mitigação |
|:---|:---:|:---|
| **Risco 1: Ordem de compilação invertida no CI/Makefile** (compilar Go antes do build do Vite, embutindo assets vazios) | Alta | Criar target explícito no `Makefile` onde `build-api` depende de `build-web`, acompanhado de teste unitário Go (`assets_test.go`) que asserte que o `embed.FS` contém arquivos e que `index.html` possui tamanho superior a zero. |
| **Risco 2: Poluição de escopo no repositório Go** | Média | Isolar 100% do código frontend no diretório `/web`. Configurar `.gitignore` e `go.mod` para ignorar completamente a pasta `web/node_modules/`. O comando `go build ./...` permanece limpo e hermético. |
| **Risco 3: Concorrência de renovação de token (Silent Refresh Race)** | Média | Implementar mutex / Promise singleton no interceptor de autenticação HTTP para evitar que múltiplas requisições paralelas 401 disparem múltiplos refreshes concorrentes. |
