# IntelliFinance Design System — Board UI Dual-Theme Edition

- **Document ID**: DESIGN-001
- **Status**: Authoritative Design System Specification
- **Default Theme**: **Light Mode (Tema Claro Padrão)** with instant **Dark Mode (`.dark`)** toggle
- **Design Reference**: Board UI (`boardui.com`) + Usability Synthesis (`SPEC-008-UX`)
- **Typography**: `Inter` (UI & Numeric Display) + `JetBrains Mono` (`tabular-nums` for financial ledger & technical badges)

---

## 1. Filosofia Visual e Princípios de Design

O Design System do **IntelliFinance** combina a estética tátil, limpa e de alta densidade da biblioteca **Board UI** com as melhores práticas de usabilidade financeira extraídas do benchmark de mercado (*Organizze, Mobills, Spendee, Wallet, Meu Dinheiro, Minhas Economias, Copilot Money*).

1. **Tema Claro como Padrão + Tema Escuro Nativo ("Um Sistema de Tokens, Dois Temas"):**
   - Todos os componentes consomem variáveis semânticas CSS (`--bui-*`) definidas em `:root` para o **Tema Claro** e remapeadas automaticamente em `html.dark` para o **Tema Escuro**.
   - Nenhum token de autenticação ou estado sensível é salvo em `localStorage`/`sessionStorage` (`FE-NFR-03`).
2. **Elevação por Tom e Bordas Hairline (`1px`):**
   - Em vez de sombras pesadas e borradas, a hierarquia espacial é construída por camadas de superfície (Canvas $\rightarrow$ Card $\rightarrow$ Inner Box $\rightarrow$ Pill Control) delimitadas por bordas de `1px`.
3. **Clareza Contábil Sem Ambiguidade (Organizze + Issue `#47`):**
   - Saldo disponível em contas líquidas jamais é somado ao limite de cartão de crédito.
   - Pagamentos de fatura (`BILL_PAYMENT`) e transferências entre contas próprias (`INTERNAL_TRANSFER`) possuem semântica visual neutra/informativa e nunca inflam os totalizadores de despesas mensais.
4. **Acessibilidade Sem Dependência Exclusiva de Cor (WCAG AA):**
   - Todo indicador financeiro ou status combina **cor + ponto luminoso (`status-dot`) ou ícone direcional + prefixo textual/sinal (`+`, `-`, `⇄`)**.

---

## 2. Arquitetura de Tokens Semânticos (`web/src/index.css`)

| Token CSS | Função Semântica | Tema Claro (`:root` — Padrão) | Tema Escuro (`html.dark`) |
| :--- | :--- | :---: | :---: |
| `--bui-bg-canvas` | Fundo global da aplicação (`<body>`) | `#FFFFFF` | `#121212` |
| `--bui-bg-sidebar` | Fundo da Sidebar Lateral e Topbar | `#F8F8FA` | `#151518` |
| `--bui-bg-card` | Superfície de Cards (KPIs, Gráficos, Containers) | `#F5F5F6` | `#18181B` |
| `--bui-bg-inner` | Superfícies internas aninhadas, inputs e boxes de ícone | `#FFFFFF` | `#212125` |
| `--bui-bg-hover` | Estado `:hover` de linhas e controles secundários | `#ECECEF` | `#27272C` |
| `--bui-bg-track` | Trilha das cápsulas de gráficos (`capsule-track`) | `#E5E5E7` | `#26262A` |
| `--bui-border` | Borda estrutural hairline (`1px`) de cards e tabelas | `#E5E5E5` | `rgba(255, 255, 255, 0.08)` |
| `--bui-border-strong` | Borda de botões pílula, selects e inputs | `#D4D4D8` | `rgba(255, 255, 255, 0.14)` |
| `--bui-text-primary` | Títulos, valores de KPIs, descrições principais | `#0A0A0A` | `#FAFAFA` |
| `--bui-text-secondary` | Rótulos secundários, datas, subtítulos | `#6B7280` | `#8E8E93` |
| `--bui-text-muted` | Cabeçalhos uppercase, placeholders, notas | `#9CA3AF` | `#52525B` |
| `--bui-accent` | Item ativo da Sidebar, CTAs primários, foco ativo | `#2563EB` | `#1D63FF` |
| `--bui-accent-hover` | Hover de botão primário | `#1D4ED8` | `#3B76FF` |
| `--bui-chart-lime` | Série primária de gráficos Capsule-Track | `#84CC16` | `#9EF01A` |
| `--bui-chart-teal` | Série secundária (Receitas / Liquidez) | `#0D9488` | `#00E5BE` |
| `--bui-chart-purple` | Série terciária / Atividade | `#7C3AED` | `#8B5CF6` |
| `--bui-badge-lime-bg` | Fundo de badge positivo / receita / conciliado | `#ECFCCB` | `rgba(158, 240, 26, 0.14)` |
| `--bui-badge-lime-text` | Texto de badge positivo / receita / conciliado | `#365314` | `#9EF01A` |
| `--bui-badge-rose-bg` | Fundo de badge despesa / estouro de orçamento | `#FFE4E6` | `rgba(244, 63, 94, 0.16)` |
| `--bui-badge-rose-text` | Texto de badge despesa / estouro de orçamento | `#9F1239` | `#FB7185` |
| `--bui-badge-amber-bg` | Fundo de badge alerta / fatura / revisão HITL | `#FEF3C7` | `rgba(245, 158, 11, 0.16)` |
| `--bui-badge-amber-text`| Texto de badge alerta / fatura / revisão HITL | `#92400E` | `#FBBF24` |
| `--bui-badge-blue-bg` | Fundo de badge transferência / Open Finance | `#DBEAFE` | `rgba(59, 130, 246, 0.16)` |
| `--bui-badge-blue-text` | Texto de badge transferência / Open Finance | `#1E40AF` | `#60A5FA` |

---

## 3. Tipografia e Regras de Formatação Monetária

1. **Família Tipográfica Principal (`font-sans`):** `'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`.
2. **Família Monoespaçada (`font-mono`):** `'JetBrains Mono', ui-monospace, SFMono-Regular, monospace` com `font-variant-numeric: tabular-nums`.
3. **Invariante `Zero Float` (`web/src/utils/money.ts`):**
   - Proibido utilizar `parseFloat()` ou `Number()` para cálculos ou formatação monetária no cliente.
   - Todas as strings decimais vindas da API (`"8420.50"`, `"-82.89"`) são processadas em centavos inteiros (`BigInt`) e formatadas deterministicamente no padrão `pt-BR` (`R$ 8.420,50`).
   - Quando o **Modo Privacidade** está ativo (`isPrivacyMode = true`), `formatCurrencyBRL` retorna `R$ •••••`.

---

## 4. Anatomia dos Componentes Assinatura (Board UI)

### 4.1 KPI Cards Minimalistas
- Container `.bui-card` (`rounded-2xl`, padding `20px`, borda `1px solid var(--bui-border)`).
- Topo: Ícone dentro de um box quadrado arredondado `.bui-inner` (`w-9 h-9 rounded-xl`).
- Centro: Rótulo descritivo em `--bui-text-secondary` (`text-xs font-medium`).
- Base: Valor principal (`text-2xl font-bold tracking-tight`) + pílula semântica ao lado (`bui-badge-lime`, `bui-badge-rose`, `bui-badge-amber` ou `bui-badge-blue`).

### 4.2 Gráficos "Capsule Track" & Lista Numérica Equivalente
- Cada barra vertical ou horizontal utiliza uma trilha completa em formato de cápsula (`.bui-bar-track`, `rounded-xl`) com preenchimento interno arredondado (`.bui-bar-fill-lime` ou `.bui-bar-fill-teal`).
- Ao lado de todo gráfico de categorias, renderiza-se obrigatoriamente a **lista numérica equivalente** em linhas `.bui-inner` clicáveis (com ponto de cor, nome da categoria, quantidade de itens, percentual e valor em `R$`).

### 4.3 Tabelas de Dados com "Status Dots" e Filtros em Pílula
- Filtros superiores em controles `.bui-pill-control` (`rounded-xl`) com `<label>` acessível e barra de **Chips de Filtros Ativos** para limpeza em 1 clique.
- Colunas de estado/conciliação exibem pílulas com ponto luminoso (`bui-dot-lime`, `bui-dot-amber`, `bui-dot-blue`).

### 4.4 Responsividade Estrita (`390x844` Mobile Viewport)
- Zero rolagem horizontal no `<body>` (`overflow-x-hidden`).
- Tabelas encapsulam `overflow-x-auto` internamente.
- Rótulos (`<label>`) sempre visíveis acima de todos os inputs de formulário.
