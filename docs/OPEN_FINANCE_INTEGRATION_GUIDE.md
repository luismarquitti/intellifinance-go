# Guia de Integração Open Finance — Pluggy.ai & Meu Pluggy

- **Documento**: `docs/OPEN_FINANCE_INTEGRATION_GUIDE.md`
- **Status**: Autorizativo / Especificação de Arquitetura e Operação
- **Data**: 11 de Novembro de 2026
- **Ambiente**: IntelliFinance Go (Homelab & GCP Serverless)
- **Normas Relacionadas**: `ADR-007` (Estratégia Open Finance), `ADR-010` (Invariantes de Transferência), `ADR-012` (Governança HITL e Servidor MCP), `docs/CONSOLIDATED_MASTER_PLAN.md`

---

## 1. Visão Geral da Arquitetura

O IntelliFinance Go adota a especificação formal do **Open Finance Brasil** normalizada pela plataforma **Pluggy.ai**. A integração é dividida em dois modos operacionais complementares:

1. **Modo Sandbox (Desenvolvimento & CI/CD)**:
   - Conector simulado da Pluggy com credenciais sintéticas (`user_ok`, `user_error`).
   - Permite testar o fluxo ponta a ponta sem qualquer dependência de contas bancárias reais ou riscos de privacidade (PII).
   - Utilizado pela suíte de testes de integração e smoke tests do repositório público.
2. **Modo Pessoal (Meu Pluggy — Gratuito & Perpétuo)**:
   - Destinado ao uso pessoal do mantenedor (até 5 conexões bancárias simultâneas gratuitas, sem necessidade de CNPJ ou diligência de compliance empresarial).
   - As contas são conectadas através do portal do usuário final [Meu Pluggy](https://meu.pluggy.ai) e espelhadas na aplicação via conector **MeuPluggy**.
   - Atualização automática em segundo plano a cada 24 horas conduzida pela infraestrutura da Pluggy, emitindo webhooks de sincronização.

```
+----------------------------------------------------------------------------------------------------+
|                                    FLUXO OPEN FINANCE NO INTELLIFINANCE                            |
+----------------------------------------------------------------------------------------------------+
|                                                                                                    |
|    [ BANCOS BRASILEIROS ]                                                                          |
|    (Itaú, Nubank, Bradesco, Inter, etc.)                                                           |
|             │                                                                                      |
|             ▼ Consentimento do Usuário via App Bancário (1 ano)                                    |
|    [ MEU PLUGGY / PLUGGY CONNECT ]                                                                 |
|             │                                                                                      |
|             ▼ Sincronização Diária (24h)                                                           |
|    [ PLUGGY API ENGINE ]                                                                           |
|             │                                                                                      |
|             ├───────────────────────────┬──────────────────────────────────────────┐               |
|             ▼ Webhook HMAC SHA-256      ▼ Polling / Sincronização                  ▼ Schemas       |
|    /open-finance/webhook         GET /accounts, /transactions, /bills       Contratos padronizados |
|             │                                   │                                                  |
|             ▼                                   ▼                                                  |
|    [ ADAPTADOR PLUGGY GO ]                                                                         |
|    (internal/adapters/outbound/openfinance/pluggy)                                                 |
|             │                                                                                      |
|             ▼ Validação e Mapeamento de Entidades                                                  |
|    [ CAMADA DE GOVERNANÇA HITL (ADR-012) ]                                                         |
|    ├── provider_balance_snapshots (Saldo informado pelo banco, sem tocar no ledger)                |
|    ├── credit_card_invoices (Faturas e parcelas separadas do débito)                              |
|    └── pending_actions (Ações que requerem consentimento humano antes de escrita contábil)        |
|             │                                                                                      |
|             ▼ Aprovação Humana Explícita                                                           |
|    [ LEDGER CONTÁBIL IMUTÁVEL ] (accounts.balance via trg_sync_balance)                            |
+----------------------------------------------------------------------------------------------------+
```

---

## 2. Passo a Passo: Cadastro e Configuração do "Meu Pluggy"

> **Instruções para o Usuário**: Siga este roteiro assim que desejar ativar a sincronização com seus bancos reais.

### Passo 1: Cadastro no Meu Pluggy
1. Acesse o portal oficial: **[https://meu.pluggy.ai](https://meu.pluggy.ai)**.
2. Crie sua conta informando seu e-mail e definindo uma senha segura.
3. Conecte suas contas bancárias reais (Itaú, Nubank, etc.):
   - O Meu Pluggy solicitará o CPF e abrirá o fluxo oficial do Open Finance para autorização no app do seu banco.
   - Cada banco é autorizado individualmente e a permissão é válida por 12 meses.

### Passo 2: Cadastro no Pluggy Dashboard de Desenvolvedor
1. Acesse o portal do desenvolvedor: **[https://dashboard.pluggy.ai](https://dashboard.pluggy.ai)**.
2. **IMPORTANTE**: Faça login com o **MESMO e-mail e senha** utilizados no Meu Pluggy no Passo 1.
3. Ativação do Conector MeuPluggy:
   - Acesse **Customização** -> **Conectores**.
   - Na barra de busca, digite `MeuPluggy`.
   - Na aba **Direct Connectors** do grupo *Personal*, certifique-se de que o conector **MeuPluggy** esteja marcado como **Ativo**.

### Passo 3: Criação da Aplicação e Conexão do Conector
1. No menu lateral, acesse **Applications** e clique na sua aplicação padrão (ou crie uma nova com nome `IntelliFinance`).
2. Obtenha as credenciais da API:
   - Copie o **Client ID**.
   - Gere e copie o **Client Secret** (ele só é exibido uma única vez).
3. Conecte sua conta do Meu Pluggy:
   - Acesse a aba **Demo** da sua aplicação.
   - Clique em **Connect Account**.
   - No widget da Pluggy que abrir, **escolha o conector "MeuPluggy"** (NÃO selecione o seu banco diretamente, nem a Sandbox!).
   - Autentique-se com as credenciais do seu Meu Pluggy.
   - Todas as contas conectadas no Passo 1 serão importadas automaticamente como **Items** da sua aplicação.
4. Identificação do `itemId`:
   - Na mesma tela **Demo**, localize o item recém-conectado na lista de **Connected Items** e copie o seu `itemId` (UUID). Guarde esse identificador.

### Passo 4: Configuração das Variáveis de Ambiente no Host `lm-claw`
No host `lm-claw`, configure o arquivo de ambiente pessoal em `~/.config/intellifinance/personal.env`:

```bash
# Credenciais Pluggy Open Finance
PLUGGY_CLIENT_ID="<seu_client_id_aqui>"
PLUGGY_CLIENT_SECRET="<seu_client_secret_aqui>"
PLUGGY_WEBHOOK_SECRET="<seu_webhook_secret_aqui>"
PLUGGY_ENVIRONMENT="production" # ou sandbox
```

---

## 3. Mapeamento Canônico de Schemas e Entidades

A consulta aos esquemas oficiais da Pluggy via MCP estabelece os seguintes padrões para o domínio Go:

### 3.1 Contas (`/accounts`)
| Campo Pluggy | Tipo | Mapeamento no Domínio Go | Semântica Contábil |
|---|---|---|---|
| `id` | UUID string | `ExternalID` | Identificador único do provedor |
| `type` | String (`BANK`, `CREDIT`, `INVESTMENT`) | `domain.AccountType` | `CHECKING`, `SAVINGS`, `CREDIT_CARD`, `INVESTMENT` |
| `balance` | Number (Double) | `provider_balance_snapshots.balance` | **Snapshot de observação**, nunca gravado diretamente em `accounts.balance` |
| `currencyCode` | String (`BRL`, `USD`) | `Currency` | Validação de moeda ISO 4217 |
| `name` | String | `Name` | Nome amigável retornado pela instituição |
| `number` | String | `Metadata["account_number"]` | Número de agência/conta mascarado |

### 3.2 Faturas de Cartão de Crédito (`/bills`)
| Campo Pluggy | Tipo | Mapeamento no Banco de Dados | Semântica Contábil |
|---|---|---|---|
| `id` | UUID string | `credit_card_invoices.external_id` | Identificador único da fatura na Pluggy |
| `dueDate` | ISO DateTime | `credit_card_invoices.due_date` | Data de vencimento da fatura para pagamento |
| `billClosingDate` | ISO DateTime (Nullable) | `credit_card_invoices.closing_date` | Data de corte/fechamento da fatura |
| `totalAmount` | Number (Double) | `credit_card_invoices.total_amount` | Valor total consolidado a pagar em `NUMERIC(15,2)` |
| `minimumPaymentAmount` | Number (Double) | `credit_card_invoices.minimum_payment` | Pagamento mínimo permitido sem inadimplência |
| `status` | Derivado de datas | `credit_card_invoices.status` | `OPEN` (aberta), `CLOSED` (fechada), `PAID` (liquidada) |

### 3.3 Transações e Parcelas (`/transactions`)
| Campo Pluggy | Tipo | Mapeamento no Domínio Go | Semântica Contábil |
|---|---|---|---|
| `id` | UUID string | `transactions.external_id` | Identificador externo estável para deduplicação |
| `amount` | Number (Double) | `transactions.amount` | Positivo para entradas (`CREDIT`), negativo para saídas (`DEBIT`) |
| `type` | String (`CREDIT`, `DEBIT`) | `domain.TransactionType` | Invertido pela Pluggy para cartão: compras são `DEBIT`, pagamentos são `CREDIT` |
| `status` | String (`POSTED`, `PENDING`) | `domain.TransactionStatus` | `COMPLETED` se `POSTED`; transações `PENDING` não afetam saldo realizado |
| `creditCardMetadata.billId` | UUID string | `transactions.bill_id` | Vínculo explícito da transação à sua respectiva fatura |
| `creditCardMetadata.installmentNumber` | Integer | `transaction_splits.installment_number` | Número da parcela atual (ex.: parcela 2) |
| `creditCardMetadata.totalInstallments` | Integer | `transaction_splits.total_installments` | Total de parcelas contratadas (ex.: de 10x) |
| `paymentData.paymentMethod` | String (`PIX`, `TED`, `BOLETO`) | `transactions.metadata["payment_method"]` | Meio de pagamento técnico (não é categoria contábil) |

---

## 4. Regras Invariantes de Governança e Segurança (HITL)

1. **Separação Rígida de Saldos (ADR-012)**:
   - O saldo real do ledger em `accounts.balance` reflete apenas a soma algébrica das transações registradas no banco de dados, auditadas pelo trigger `trg_sync_balance`.
   - O saldo informado pela Pluggy no momento da sincronização é armazenado na tabela `provider_balance_snapshots`.
   - Divergências entre os dois valores são expostas como *Itens de Conciliação Pendente* para decisão humana, sem jamais alterar o passado arbitrariamente.
2. **Prevenção de Dupla Contagem no Cartão de Crédito**:
   - Compras realizadas no cartão geram débitos na conta de cartão (`CREDIT_CARD`) e vinculam-se à fatura (`bill_id`).
   - O pagamento da fatura debitado da conta corrente é registrado como uma **Transferência Interna** (`POST /transfers/confirm` ou auto-conciliação) entre a Conta Corrente e a Conta do Cartão de Crédito.
   - Dessa forma, as despesas são contabilizadas no momento da compra (pelo regime de competência), e o pagamento da fatura é um evento neutro de liquidação de passivo (não gerando uma segunda despesa no fluxo de caixa).
3. **Servidor MCP Go (`cmd/mcp`) e HITL Mandatório**:
   - Modelos de IA só possuem acesso a ferramentas de leitura (`list_accounts`, `get_balances`, `list_transactions`, `summarize_transactions`).
   - Sugestões de categorização ou vinculação geradas por agentes são gravadas na tabela `pending_actions` (`status =  PENDING`).
   - Apenas o usuário humano através da interface web do IntelliFinance pode clicar em **Aprovar** para efetivar a transação.
