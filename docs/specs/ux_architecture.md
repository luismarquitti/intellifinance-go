# Especificação de Arquitetura de Informação e Usabilidade (UI/UX)

- **Document ID**: SPEC-008-UX
- **Status**: Approved / Active Implementation
- **Companion Documents**: `design/DESIGN_SYSTEM.md` (DESIGN-001), `docs/specs/frontend.md` (SPEC-007-FRONTEND), `docs/PRD.md`
- **Research Baseline**: Relatório Exploratório Lupa UX (`relatoriouiux.html` — Mobills, Organizze, Minhas Economias, Meu Dinheiro, Wallet by BudgetBakers, Spendee) + Referências Globais (Copilot Money, Monarch Money, Board UI)

---

## 1. Síntese Executiva do Benchmark de Usabilidade

A arquitetura de experiência do usuário do **IntelliFinance** une as principais forças funcionais comprovadas no mercado de gestão financeira pessoal:

| Sistema Analisado | Principal Força Absorvida | Aplicação no IntelliFinance |
| :--- | :--- | :--- |
| **Organizze** | Separação clara entre Contas Líquidas e Cartões de Crédito; identificação de conta `Manual` vs. `Conectada`; ocultação de saldos (Modo Privacidade); rótulos persistentes em formulários. | `AccountsRail.tsx`, toggle de Modo Privacidade no Header e separação contábil estrita nos KPIs. |
| **Mobills** | Leitura conjunta entre visão macro (gráfico de categorias/fluxo) e visão micro (transações); linha do tempo dedicada para faturas de cartão de crédito; botão de ação rápida `+`. | `CategoryBreakdownPanel.tsx` (gráfico + lista equivalente clicável), `InvoicesTimeline.tsx` (13 faturas Open Finance) e `QuickTransactionModal.tsx`. |
| **Spendee** | Tradução do orçamento mensal abstrato em disponibilidade diária acionável (**"Disponível por Dia"** / *Available per day*). | Cálculo determinístico `available_per_day = remaining_budget / days_remaining` exibido nos KPIs e na aba de Orçamentos. |
| **Wallet (BudgetBakers)** | Contas como unidades visuais selecionáveis que filtram todo o painel com 1 clique. | Clique em qualquer card de conta ou fatura filtra instantaneamente os indicadores e a tabela. |
| **Meu Dinheiro** | Conciliação auditável, divisão de lançamentos (*Splits*) e prevenção de rolagem horizontal mobile (`390x844`). | Central HITL com conciliação de faturas (`BILL_PAYMENT`), agrupamento por estabelecimento e layout responsivo sem overflow. |
| **Minhas Economias** | Classificação transversal por Tags e Membros da família sem poluir a árvore de categorias. | Suporte a tags, notas e visibilidade (`SHARED`, `PERSONAL`, `RESTRICTED`) no lançamento rápido progressivo. |
| **Board UI** | Sistema de tokens Dual-Theme (Tema Claro padrão + Tema Escuro), gráficos *Capsule-Track*, *Status Dots* luminosos e navegação lateral limpa. | Identidade visual oficial documentada em `design/DESIGN_SYSTEM.md`. |

---

## 2. Arquitetura de Navegação Multi-Seção

A aplicação evolui de uma listagem única para um **App Shell Multi-Seção** com **Seletor Global de Mês** (`< Outubro | Novembro 2026 | Dezembro >` + `Todo o Período`):

1. **Principal (`overview`)**:
   - Rail de Contas Líquidas & Cartões de Crédito com filtro rápido em 1 clique;
   - 5 KPI Cards desacoplados (*Saldo em Contas Líquidas*, *Receitas no Período*, *Despesas Realizadas sem Dupla Contagem*, *Fatura Atual & Liquidez Projetada*, *Disponível por Dia*);
   - Painel Analítico Macro $\leftrightarrow$ Micro (Gráfico Capsule-Track + Lista Numérica Equivalente por Categoria com drill-down);
   - Resumo de Ações Pendentes da IA (HITL) e Transações Recentes do período.
2. **Contas & Faturas (`accounts`)**:
   - Gestão de contas bancárias conectadas via Pluggy Open Finance v2 e contas manuais (`+ Nova Conta`);
   - **Linha do Tempo das Faturas do Cartão (`InvoicesTimeline`)**: exibição interativa das 13 faturas sincronizadas (`credit_card_invoices`), mostrando status (`OPEN`, `CLOSED`, `PAID`), data de vencimento, valor total e listagem filtrada por `bill_id`.
3. **Transações (`transactions`)**:
   - Livro razão completo com barra de busca, filtros por conta, categoria, tipo, origem e status;
   - Barra de **Chips de Filtros Ativos** com remoção individual em 1 clique;
   - Badges explícitas para `BILL_PAYMENT` e `INTERNAL_TRANSFER` e edição inline de categoria em 1 clique.
4. **Orçamentos & Fixas (`budgets`)**:
   - Configuração de teto mensal global e por categoria (`budgets`);
   - Indicador **"Disponível por Dia (`R$/dia`)"** e barras de progresso em cápsula (Verde $<85\%$, Âmbar $85\text{–}99\%$, Vermelho $\ge 100\%$).
5. **Revisão IA / HITL (`hitl`)**:
   - Central dedicada para revisar propostas de conciliação de faturas (`RECONCILE_TRANSFER`) e auto-categorização (`CATEGORIZE_TRANSACTION`);
   - **Agrupamento por Estabelecimento/Categoria**: permite aprovar dezenas de transações recorrentes do mesmo comerciante (ex.: *iFood*, *Mini Extra*, *Rendimentos Aplic Aut Mais*) com um único clique no grupo ou aprovar todas em lote.

---

## 3. Catálogo de User Stories Normativas (EARS + BDD)

### US-UX-01 (P0): App Shell Dual-Theme (Tema Claro Padrão + Tema Escuro) e Modo Privacidade
- **Requisito EARS:** **Quando** o usuário acessa o IntelliFinance, o sistema **deve** iniciar no Tema Claro da Board UI por padrão e oferecer controles imediatos na Sidebar/Header para alternar para o Tema Escuro, navegar entre as 5 seções principais, alternar o mês de referência e ativar o Modo Privacidade (`R$ •••••`).

### US-UX-02 (P0): Rail de Contas Separado e Linha do Tempo de Faturas
- **Requisito EARS:** **Quando** as contas e faturas são carregadas, o sistema **deve** exibir Contas Líquidas separadas de Cartões de Crédito com badge de origem (`Pluggy` vs. `Manual`), permitindo filtrar por conta ou selecionar uma fatura específica (`bill_id`) na linha do tempo de faturas.

### US-UX-03 (P0): KPIs Desacoplados Sem Dupla Contagem e Precisão String-First
- **Requisito EARS:** **Enquanto** o resumo financeiro é exibido, o sistema **deve** excluir transferências internas e pagamentos de fatura dos totalizadores de receitas/despesas operacionais e formatar todos os valores monetários sem uso de `parseFloat`.

### US-UX-04 (P0): Lançamento Rápido com Revelação Progressiva
- **Requisito EARS:** **Quando** o usuário aciona `+ Novo Lançamento`, o sistema **deve** exibir os 5 campos essenciais com `<label>` persistente e manter metadados avançados (`Status`, `Visibilidade`, `Notas`) recolhidos em um acordeão progressivo.

### US-UX-05 (P0): Central HITL com Agrupamento Inteligente por Estabelecimento
- **Requisito EARS:** **Quando** o usuário acessa a Revisão IA (HITL), o sistema **deve** oferecer visualização agrupada por estabelecimento/categoria sugerida além da lista individual, permitindo aprovar grupos inteiros ou toda a fila em 1 clique.

### US-UX-06 (P0): Responsividade Estrita (`390x844`) sem Rolagem Horizontal
- **Requisito EARS:** **Em qualquer** viewport $\ge 320\text{px}$, o sistema **não deve** produzir rolagem horizontal no documento principal nem sobrepor controles de paginação com barras flutuantes.

### US-UX-07 (P1): Motor de Orçamento Mensal e Indicador "Disponível por Dia"
- **Requisito EARS:** **Quando** um orçamento mensal está configurado, o sistema **deve** calcular no backend (`shopspring/decimal`) o saldo restante e o valor **Disponível por Dia (`R$/dia`)** nos dias restantes do mês, sinalizando alertas visuais em $85\%$ e $100\%$.

### US-UX-08 (P1): Painel Analítico (Gráfico Capsule-Track + Lista Numérica Equivalente com Drill-Down)
- **Requisito EARS:** **Quando** a distribuição por categorias é exibida, o sistema **deve** apresentar o gráfico acompanhado de lista numérica detalhada (`Valor`, `%`, `Qtd.`) e filtrar a tabela de transações ao clicar em qualquer categoria.
