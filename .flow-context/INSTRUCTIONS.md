# MISSION DIRECTIVE — AGENT: 03-patch-fixer
# Identity: Surgical Patch Fixer
Name: Scalpel
Role: Especialista em Correção Cirúrgica e Resolução de Falhas de Gate
Version: 2.1.0
Emoji: 🩺
Capabilities:
  - Análise aprofundada de relatórios de falha do gate mecânico (`go test -race`, `golangci-lint`, `go vet`)
  - Correção direcionada de apontamentos levantados pelo Comitê de Revisão ou pelo Shadow Judge
  - Aplicação de patches mínimos e atômicos sem efeitos colaterais em módulos adjacentes

# CORE BOUNDARIES & RED LINES (SOUL)
# Soul: Scalpel (03-patch-fixer)

Você é o cirurgião de código da FSM. Você é invocado estritamente quando a implementação falhou em algum gate mecânico ou foi reprovada por um auditor no ambiente estéril.
Sua missão é resolver o problema apontado no arquivo `fix_reason.txt` com a menor quantidade de mudanças necessárias, sem refatorações amplas e sem alterar contratos existentes.

Princípios Inegociáveis (Red Lines):
1. Leia atentamente o arquivo `fix_reason.txt`. Suas alterações DEVEM atacar especificamente os pontos ali documentados.
2. NUNCA desabilite verificações de teste, linter ignorado (`//nolint`) ou flags de race detector (`-race`) para fazer um gate passar.
3. Não refatore código não relacionado à falha. Mantenha o diff mínimo e cirúrgico.
4. Após o patch, execute localmente `go test -race ./...` e `golangci-lint run` para garantir que a correção não gerou regressões.
5. Faça o commit da correção no branch com mensagem semântica descritiva (`fix: <motivo do patch>`).

# ESPECIFICAÇÃO TÉCNICA E CRITÉRIOS DE ACEITE
# Especificação Técnica Formal — [Phase 5] Validação de Isolamento Criptográfico Multi-Tenant (RLS Testing Suite)

- **Task ID**: `gh-26` (Issue #26)
- **Fase**: `Phase 5 — Public Portfolio Polish & Showcase`
- **Arquiteto Responsável**: Archon (`01-spec-architect`)
- **Artefato de Planejamento**: [Implementation Plan: [Phase 5] Validação de Isolamento Criptográfico Multi-Tenant (RLS Testing Suite)](file:///home/luismarquitti/.gemini/antigravity-cli/brain/4a90b7e2-7620-4065-bf95-1bed8b167ae6/spec_rls_isolation_testing_plan.md)
- **Arquivos Alvo**:
  - [`test/e2e/rls_isolation_test.go`](file:///home/luismarquitti/repos/intellifinance-go/test/e2e/rls_isolation_test.go) (Suíte E2E de validação de isolamento RLS, prova estrutural e mutações maliciosas)
  - [`test/e2e/rls_concurrency_test.go`](file:///home/luismarquitti/repos/intellifinance-go/test/e2e/rls_concurrency_test.go) (Suíte de concorrência com estresse de 64 goroutines e multiplexação de pool sob `-race`)
  - [`internal/domain/tenant_isolation.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/tenant_isolation.go) (Modelo de domínio para relatórios de auditoria e vereditos de isolamento)
  - [`internal/domain/tenant_isolation_test.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/tenant_isolation_test.go) (Testes unitários dos tipos de domínio e invariantes contábeis)
  - [`internal/ports/tenant_isolation.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/ports/tenant_isolation.go) (Contrato de porta formal para verificação programática de isolamento)
  - [`internal/adapters/outbound/persistence/postgres/isolation_verifier.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/adapters/outbound/persistence/postgres/isolation_verifier.go) (Adaptador de persistência para execução de provas estruturais e geração de relatórios)
  - [`internal/adapters/outbound/persistence/postgres/isolation_verifier_test.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/adapters/outbound/persistence/postgres/isolation_verifier_test.go) (Testes de integração do verificador de isolamento)

---

## 1. Visão Geral e Invariantes Arquiteturais

Esta especificação define os contratos formais, requisitos determinísticos no padrão EARS, modelos de domínio e cenários de teste automatizados para a validação empírica e matemática do **Isolamento Criptográfico e Relacional Multi-Tenant** do **IntelliFinance**.

O objetivo primordial é fornecer uma suíte de testes de integração e ponta a ponta (E2E) que comprove, de forma incontestável em pipelines de CI/CD e ambientes de homologação, que nenhum dado financeiro ou metadado vaza entre inquilinos distintos (por exemplo: Tenant A "Família Marquitti" vs Tenant B "Convidado / Edson Silva"), mesmo na ocorrência de omissões ou falhas lógicas no código da aplicação.

### 1.1 Invariantes Inegociáveis (Red Lines)

1. **Defesa em Profundidade Nativa do Banco (Engine-Level PostgreSQL RLS)**:
   - A segregação multi-tenant é imposta diretamente no motor de armazenamento relacional do PostgreSQL 16+ através de políticas de *Row-Level Security* (`ENABLE ROW LEVEL SECURITY` e `FORCE ROW LEVEL SECURITY`), conforme estabelecido na migração [`migrations/000001_init_schema.up.sql`](file:///home/luismarquitti/repos/intellifinance-go/migrations/000001_init_schema.up.sql#L450-L543).
   - O isolamento **NÃO DEVE** depender unicamente de cláusulas `WHERE tenant_id = $1` injetadas pelas queries da camada de aplicação. A suíte de teste DEVE executar consultas cruas omitindo filtros de tenant para demonstrar que o motor PostgreSQL intercepta a chamada e retorna estritamente zero registros de terceiros.
   - A cláusula `FORCE ROW LEVEL SECURITY` garante que até mesmo o proprietário das tabelas (`table owner`) seja forçado a obedecer às políticas de isolamento.

2. **Invariante Contábil Zero Floats (`Zero Floats Policy`)**:
   - É **terminantemente proibido** o uso de `float32` ou `float64` em qualquer struct, método, assinatura de função, cálculo de saldos, subtotais ou asserções de teste.
   - Todas as quantias monetárias DEVEM operar estritamente sobre a biblioteca `github.com/shopspring/decimal` ([`decimal.Decimal`](file:///home/luismarquitti/repos/intellifinance-go/go.mod#L13)).
   - Construtores de fixtures financeiras DEVEM utilizar `decimal.RequireFromString("1500.50")` ou `decimal.NewFromIntWithExponent`, evitando conversões a partir de ponto flutuante.

3. **Segregação Transacional Hermética no Pool (`pgxpool`)**:
   - Conexões físicas TCP são reutilizadas continuamente pelo pool [`*pgxpool.Pool`](file:///home/luismarquitti/repos/intellifinance-go/internal/adapters/outbound/persistence/postgres/tx_manager.go#L15).
   - A injeção de identidade de tenancy DEVE ocorrer exclusivamente via variáveis de sessão locais à transação (`SET LOCAL app.current_tenant_id = $1` e `SET LOCAL app.current_user_id = $2`), encapsuladas pela função [`postgres.WithTenantTx`](file:///home/luismarquitti/repos/intellifinance-go/internal/adapters/outbound/persistence/postgres/tx_manager.go#L44-L85).
   - Quando a transação finaliza (`COMMIT` ou `ROLLBACK`), as configurações de sessão expiram automaticamente, garantindo que a conexão devolvida ao pool retorne ao estado limpo sem contaminação (*connection pollution*).

4. **Garantia Absoluta de Não-Vazamento Cross-Tenant (Leitura, Mutação e Injeção)**:
   - **Leitura**: Sob a sessão autenticada do Tenant A, consultas a [`accounts`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/account.go), [`transactions`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/transaction.go), [`categories`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/category.go), [`transaction_splits`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/split.go) e [`ingestion_jobs`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/ingestion_job.go) DEVEM retornar exatamente 0 registros pertencentes ao Tenant B.
   - **Mutação Cruzada (`UPDATE` / `DELETE`)**: Tentativas de alterar ou excluir entidades do Tenant B a partir de uma transação vinculada ao Tenant A DEVEM afetar exatamente 0 linhas ou ser categoricamente rejeitadas pelo PostgreSQL.
   - **Injeção Cruzada (`INSERT`)**: Tentativas de inserir registros especificando o `tenant_id` do Tenant B a partir de uma sessão com `app.current_tenant_id = Tenant A` DEVEM falhar imediatamente com erro de violação de política de RLS do PostgreSQL (`ERROR: new row violates row-level security policy`).

5. **Blindagem de Privacidade Intra-Tenant (Household Discretionary Privacy)**:
   - No modelo de condomínio familiar (`HOUSEHOLD`), despesas e contas marcadas como `visibility = 'PRIVATE'` pertencentes ao Usuário 1 (ex.: gastos pessoais discricionários) DEVEM ser completamente invisíveis para o Usuário 2 da mesma família.
   - Apenas registros marcados como `visibility = 'SHARED'` são agregados no orçamento conjunto da família.

6. **Integridade Criptográfica e Unicidade Determinística de Fingerprints**:
   - Todas as transações ingeridas possuem uma impressão digital criptográfica (`Fingerprint`) calculada deterministicamente via `crypto/sha256` a partir da tupla `(date, amount, clean_description, source, sequence_index)`.
   - Transações com mesmo valor, data e descrição em tenants distintos DEVEM coexistir sem interferência mútua, com deduplicação escopada estritamente pelo `tenant_id`.

7. **Mínimo Privilégio de Banco e Execução Segura**:
   - Os testes de isolamento DEVEM conectar-se ao banco utilizando a role de aplicação não-privilegiada `intellifinance_app` (`NOSUPERUSER NOBYPASSRLS`), garantindo fidelidade de produção.
   - Se o banco de dados não estiver disponível (ex.: pipeline de CI leve ou ambiente sem Docker), a suíte DEVE emitir diagnóstico informativo claro e utilizar `t.Skipf`, mantendo o build verde.

8. **Concorrência Segura sob o Race Detector (`go test -race`)**:
   - A suíte de teste DEVE executar 64 goroutines simultâneas sob alta taxa de reciclagem de conexões em pool restrito (`MaxConns = 10`), comprovando que não ocorrem condições de corrida, data races ou vazamento de variáveis de sessão entre threads concorrentes.

---

## 2. Requisitos Funcionais e Critérios de Aceite no Formato EARS

Os requisitos formais abaixo regem o comportamento da suíte de teste de isolamento multi-tenant:

### 2.1 Provisionamento e Carga de Entidades
- **REQ-RLS-01**: **Quando** a suíte de teste inicia, **se** a conexão com o PostgreSQL estiver disponível, o sistema **DEVE** provisionar deterministicamente dois inquilinos isolados: Tenant A (*Família Marquitti*, tipo `HOUSEHOLD`) e Tenant B (*Convidado / Edson Silva*, tipo `PERSONAL`).
- **REQ-RLS-02**: **Quando** os inquilinos são criados, **se** os parâmetros forem válidos, o sistema **DEVE** criar no mínimo 2 usuários vinculados ao Tenant A (Usuário Administrador `userA1` e Usuário Cônjuge `userA2`) e 1 usuário vinculado ao Tenant B (`userB1`).
- **REQ-RLS-03**: **Quando** os registros financeiros são inseridos para o Tenant A e para o Tenant B, **se** as transações contiverem valores monetários, o sistema **DEVE** instanciar todas as quantias monetárias estritamente através de `decimal.Decimal`, com precisão de 2 casas decimais, proibindo qualquer tipo float.
- **REQ-RLS-04**: **Quando** as transações forem persistidas, **se** possuírem metadados canônicos, o sistema **DEVE** gerar hashes SHA-256 criptográficos determinísticos para seus fingerprints, validando a integridade de deduplicação independente por inquilino.

### 2.2 Isolamento Estrito de Leitura (Cross-Tenant Read Isolation)
- **REQ-RLS-05**: **Quando** uma consulta de listagem de contas (`ListAccounts`) é executada sob o contexto transacional do Tenant A, **se** existirem contas cadastradas para o Tenant A e para o Tenant B, o sistema **DEVE** retornar exclusivamente as contas do Tenant A e exatamente 0 contas do Tenant B.
- **REQ-RLS-06**: **Quando** uma consulta direta por chave primária UUID (`GetAccountByID`) de uma conta pertencente ao Tenant B for realizada sob o contexto transacional do Tenant A, **se** o ID existir no banco de dados, o sistema **DEVE** falhar com erro canônico `pgx.ErrNoRows`, comprovando a invisibilidade absoluta da entidade.
- **REQ-RLS-07**: **Quando** uma listagem de transações (`ListTransactions`) ou categorias (`ListCategories`) é executada sob o contexto transacional do Tenant B, **se** houver registros do Tenant A, o sistema **DEVE** retornar 0 registros do Tenant A.
- **REQ-RLS-08**: **Quando** uma consulta SQL crua sem cláusula `WHERE tenant_id` for executada via `tx.QueryRow` sob o contexto transacional do Tenant B, **se** o PostgreSQL RLS estiver ativo com `FORCE ROW LEVEL SECURITY`, o sistema **DEVE** retornar contagem igual a 0 para IDs de entidades do Tenant A, provando a blindagem no motor do banco.

### 2.3 Bloqueio de Mutações Maliciosas (Cross-Tenant Mutation Defense)
- **REQ-RLS-09**: **Quando** um comando de atualização (`UPDATE accounts SET name = 'Invasão'`) direcionado ao ID de uma conta do Tenant B for disparado sob a sessão do Tenant A, **se** a transação estiver ativa, o sistema **DEVE** retornar exatamente 0 linhas afetadas (`RowsAffected() == 0`).
- **REQ-RLS-10**: **Quando** um comando de exclusão (`DELETE FROM transactions WHERE id = $1`) direcionado a uma transação do Tenant B for disparado sob a sessão do Tenant A, **se** a transação estiver ativa, o sistema **DEVE** retornar exatamente 0 linhas afetadas (`RowsAffected() == 0`).
- **REQ-RLS-11**: **Quando** um comando de inserção (`INSERT INTO accounts`) for submetido sob a sessão do Tenant A, **se** o payload informar explicitamente o `tenant_id` pertencente ao Tenant B, o sistema **DEVE** abortar a operação com erro de violação de política RLS (`violates row-level security policy`).

### 2.4 Blindagem de Sessão Desautenticada e Vazamento Fora de Transação
- **REQ-RLS-12**: **Quando** uma consulta for realizada diretamente sobre o pool de conexões sem invocação de `WithTenantTx` (variáveis `app.current_tenant_id` e `app.current_user_id` não configuradas), **se** as tabelas estiverem sob `FORCE ROW LEVEL SECURITY`, o sistema **DEVE** retornar exatamente 0 registros para qualquer tabela escopada por tenant.
- **REQ-RLS-13**: **Quando** um comando de inserção for executado diretamente sobre o pool fora de `WithTenantTx`, **se** a tabela exigir validação de `app.current_tenant_id`, o sistema **DEVE** rejeitar o comando por violação de política RLS.

### 2.5 Privacidade Intra-Tenant (Household Co-Budgeting Isolation)
- **REQ-RLS-14**: **Quando** o Usuário Cônjuge (`userA2`) consulta as contas do Tenant A, **se** o Usuário Administrador (`userA1`) possuir uma conta cadastrada com `visibility = 'PRIVATE'`, o sistema **DEVE** retornar `pgx.ErrNoRows` para a conta privada na consulta direta por ID e omiti-la das listagens gerais.
- **REQ-RLS-15**: **Quando** qualquer membro do Tenant A consultar as contas do domicílio, **se** uma conta estiver cadastrada com `visibility = 'SHARED'`, o sistema **DEVE** retornar a conta normalmente com seu saldo consolidado em `decimal.Decimal`.

### 2.6 Concorrência e Reciclagem de Pool sob `-race`
- **REQ-RLS-16**: **Quando** 64 goroutines simultâneas executarem rajadas de transações alternadas entre Tenant A e Tenant B em um pool com teto de 10 conexões físicas (`MaxConns = 10`), **se** o binário for compilado com `-race`, o sistema **DEVE** concluir 100% das transações sem vazamento de contexto de tenancy entre iterações e sem condições de corrida.

### 2.7 Emissão de Relatório de Auditoria
- **REQ-RLS-17**: **Quando** a suíte de testes de isolamento concluir sua execução, **se** todos os testes forem bem-sucedidos, o sistema **DEVE** emitir um relatório estruturado nos logs de teste (`t.Logf`) contendo as contagens verificadas, o veredito `ISOLATION_PASSED` e a prova da integridade criptográfica.

---

## 3. Contratos de Domínio e Interfaces Go

Os contratos e modelos a seguir definem os tipos estritos para a auditoria de isolamento e comprovação determinística.

### 3.1 Modelo de Domínio (`internal/domain/tenant_isolation.go`)

```go
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// IsolationVerdict define o veredito determinístico de uma verificação de isolamento.
type IsolationVerdict string

const (
	VerdictPassed    IsolationVerdict = "ISOLATION_PASSED"
	VerdictViolation IsolationVerdict = "ISOLATION_VIOLATION"
	VerdictSkipped   IsolationVerdict = "ISOLATION_SKIPPED"
)

// IsolationCheckType categoriza o vetor de teste de isolamento auditado.
type IsolationCheckType string

const (
	CheckCrossTenantRead     IsolationCheckType = "CROSS_TENANT_READ"
	CheckCrossTenantMutation IsolationCheckType = "CROSS_TENANT_MUTATION"
	CheckCrossTenantInsert   IsolationCheckType = "CROSS_TENANT_INSERT"
	CheckUnauthenticatedRead IsolationCheckType = "UNAUTHENTICATED_READ"
	CheckIntraTenantPrivacy  IsolationCheckType = "INTRA_TENANT_PRIVACY"
	CheckConcurrencyPoolLeak IsolationCheckType = "CONCURRENCY_POOL_LEAK"
)

// IsolationAuditRecord registra o resultado pontual de um vetor de auditoria.
type IsolationAuditRecord struct {
	CheckType        IsolationCheckType `json:"check_type"`
	TargetTable      string             `json:"target_table"`
	Operation        string             `json:"operation"`
	AuthenticatedAs  uuid.UUID          `json:"authenticated_as"`
	VictimTenantID   uuid.UUID          `json:"victim_tenant_id"`
	RecordsObserved  int64              `json:"records_observed"`
	ExpectedRecords  int64              `json:"expected_records"`
	RowsAffected     int64              `json:"rows_affected"`
	ExpectedAffected int64              `json:"expected_affected"`
	Verdict          IsolationVerdict   `json:"verdict"`
	Details          string             `json:"details"`
}

// TenantIsolationReport consolida o relatório formal de auditoria multi-tenant.
type TenantIsolationReport struct {
	ExecutionID      uuid.UUID              `json:"execution_id"`
	Timestamp        time.Time              `json:"timestamp"`
	TenantAID        uuid.UUID              `json:"tenant_a_id"`
	TenantAName      string                 `json:"tenant_a_name"`
	TenantBID        uuid.UUID              `json:"tenant_b_id"`
	TenantBName      string                 `json:"tenant_b_name"`
	TotalChecks      int                    `json:"total_checks"`
	PassedChecks     int                    `json:"passed_checks"`
	Violations       int                    `json:"violations"`
	FinalVerdict     IsolationVerdict       `json:"final_verdict"`
	AuditRecords     []IsolationAuditRecord `json:"audit_records"`
	TotalAssetVolume decimal.Decimal        `json:"total_asset_volume"`
}

// NewTenantIsolationReport instancia um novo relatório de auditoria formal.
func NewTenantIsolationReport(tenantAID uuid.UUID, tenantAName string, tenantBID uuid.UUID, tenantBName string) *TenantIsolationReport {
	return &TenantIsolationReport{
		ExecutionID:      uuid.New(),
		Timestamp:        time.Now().UTC(),
		TenantAID:        tenantAID,
		TenantAName:      strings.TrimSpace(tenantAName),
		TenantBID:        tenantBID,
		TenantBName:      strings.TrimSpace(tenantBName),
		TotalChecks:      0,
		PassedChecks:     0,
		Violations:       0,
		FinalVerdict:     VerdictPassed,
		AuditRecords:     make([]IsolationAuditRecord, 0),
		TotalAssetVolume: decimal.Zero,
	}
}

// AddRecord registra um vetor auditado no relatório e atualiza os contadores.
func (r *TenantIsolationReport) AddRecord(rec IsolationAuditRecord) {
	r.AuditRecords = append(r.AuditRecords, rec)
	r.TotalChecks++
	if rec.Verdict == VerdictPassed {
		r.PassedChecks++
	} else if rec.Verdict == VerdictViolation {
		r.Violations++
		r.FinalVerdict = VerdictViolation
	}
}

// FormatTable renderiza o relatório em formato tabular legível para logs de teste.
func (r *TenantIsolationReport) FormatTable() string {
	var sb strings.Builder
	sb.WriteString("\n========================================================================================\n")
	sb.WriteString(fmt.Sprintf("🛡️ INTELLIFINANCE RLS ISOLATION AUDIT REPORT — Execution: %s\n", r.ExecutionID))
	sb.WriteString(fmt.Sprintf("Timestamp: %s | Overall Verdict: %s\n", r.Timestamp.Format(time.RFC3339), r.FinalVerdict))
	sb.WriteString(fmt.Sprintf("Tenant A: %s (%s) | Tenant B: %s (%s)\n", r.TenantAName, r.TenantAID, r.TenantBName, r.TenantBID))
	sb.WriteString(fmt.Sprintf("Total Checks: %d | Passed: %d | Violations: %d\n", r.TotalChecks, r.PassedChecks, r.Violations))
	sb.WriteString("----------------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("%-24s | %-16s | %-8s | %-6s | %-6s | %s\n", "CHECK TYPE", "TABLE", "OP", "OBS", "EXP", "VERDICT"))
	sb.WriteString("----------------------------------------------------------------------------------------\n")
	for _, rec := range r.AuditRecords {
		sb.WriteString(fmt.Sprintf("%-24s | %-16s | %-8s | %-6d | %-6d | %s\n",
			rec.CheckType, rec.TargetTable, rec.Operation, rec.RecordsObserved, rec.ExpectedRecords, rec.Verdict))
	}
	sb.WriteString("========================================================================================\n")
	return sb.String()
}
```

---

### 3.2 Contrato de Interface da Porta (`internal/ports/tenant_isolation.go`)

```go
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// TenantIsolationVerifier define a porta formal para execução de provas de isolamento RLS.
type TenantIsolationVerifier interface {
	// VerifyCrossTenantRead executa asserções de invisibilidade de leitura entre dois inquilinos.
	VerifyCrossTenantRead(ctx context.Context, tenantAID, tenantBID uuid.UUID) (*domain.TenantIsolationReport, error)

	// VerifyCrossTenantMutation executa tentativas de mutações maliciosas cruzadas (UPDATE/DELETE).
	VerifyCrossTenantMutation(ctx context.Context, attackerTenantID, victimTenantID uuid.UUID) (*domain.TenantIsolationReport, error)

	// VerifyIntraTenantPrivacy verifica a ocultação de registros PRIVATE dentro do mesmo condomínio familiar.
	VerifyIntraTenantPrivacy(ctx context.Context, tenantID, ownerUserID, partnerUserID uuid.UUID) (*domain.TenantIsolationReport, error)

	// VerifyUnauthenticatedAccess valida que acessos fora de WithTenantTx retornam zero registros.
	VerifyUnauthenticatedAccess(ctx context.Context, sampleTenantID uuid.UUID) (*domain.TenantIsolationReport, error)
}
```

---

### 3.3 Implementação do Adaptador Postgres (`internal/adapters/outbound/persistence/postgres/isolation_verifier.go`)

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.TenantIsolationVerifier = (*IsolationVerifier)(nil)

// IsolationVerifier implementa a porta ports.TenantIsolationVerifier sobre o PostgreSQL.
type IsolationVerifier struct {
	pool      *pgxpool.Pool
	txManager *TxManager
}

// NewIsolationVerifier constrói um novo verificador de isolamento.
func NewIsolationVerifier(pool *pgxpool.Pool) *IsolationVerifier {
	return &IsolationVerifier{
		pool:      pool,
		txManager: NewTxManager(pool),
	}
}

// VerifyCrossTenantRead valida a invisibilidade estrita de leitura entre inquilinos.
func (v *IsolationVerifier) VerifyCrossTenantRead(ctx context.Context, tenantAID, tenantBID uuid.UUID) (*domain.TenantIsolationReport, error) {
	report := domain.NewTenantIsolationReport(tenantAID, "Tenant A", tenantBID, "Tenant B")

	// 1. Executa verificação autenticado como Tenant A contra entidades de Tenant B
	err := v.txManager.WithTenantTx(ctx, tenantAID, uuid.Nil, func(tx pgx.Tx) error {
		tables := []string{"accounts", "transactions", "categories", "transaction_splits", "ingestion_jobs"}
		for _, tbl := range tables {
			var count int64
			query := fmt.Sprintf("SELECT count(*) FROM %s WHERE tenant_id = $1", tbl) //nolint:gosec // query estática com tabela controlada
			if scanErr := tx.QueryRow(ctx, query, tenantBID).Scan(&count); scanErr != nil {
				return fmt.Errorf("failed to query table %s: %w", tbl, scanErr)
			}

			verdict := domain.VerdictPassed
			if count != 0 {
				verdict = domain.VerdictViolation
			}

			report.AddRecord(domain.IsolationAuditRecord{
				CheckType:       domain.CheckCrossTenantRead,
				TargetTable:     tbl,
				Operation:       "SELECT",
				AuthenticatedAs: tenantAID,
				VictimTenantID:  tenantBID,
				RecordsObserved: count,
				ExpectedRecords: 0,
				Verdict:         verdict,
				Details:         fmt.Sprintf("Queried %s belonging to Tenant B from Tenant A session", tbl),
			})
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during VerifyCrossTenantRead: %w", err)
	}

	return report, nil
}

// VerifyCrossTenantMutation valida o bloqueio estrito de UPDATE e DELETE entre inquilinos.
func (v *IsolationVerifier) VerifyCrossTenantMutation(ctx context.Context, attackerTenantID, victimTenantID uuid.UUID) (*domain.TenantIsolationReport, error) {
	report := domain.NewTenantIsolationReport(attackerTenantID, "Attacker Tenant", victimTenantID, "Victim Tenant")

	err := v.txManager.WithTenantTx(ctx, attackerTenantID, uuid.Nil, func(tx pgx.Tx) error {
		// Tentativa maliciosa de UPDATE em contas do inquilino vítima
		cmdTag, updateErr := tx.Exec(ctx, "UPDATE accounts SET name = 'MALICIOUS_OVERWRITE' WHERE tenant_id = $1", victimTenantID)
		if updateErr != nil && !errors.Is(updateErr, pgx.ErrNoRows) {
			// Erro do PostgreSQL é aceitável como bloqueio
			report.AddRecord(domain.IsolationAuditRecord{
				CheckType:       domain.CheckCrossTenantMutation,
				TargetTable:     "accounts",
				Operation:       "UPDATE",
				AuthenticatedAs: attackerTenantID,
				VictimTenantID:  victimTenantID,
				RowsAffected:    0,
				ExpectedAffected: 0,
				Verdict:         domain.VerdictPassed,
				Details:         fmt.Sprintf("UPDATE rejected by engine error: %v", updateErr),
			})
		} else {
			verdict := domain.VerdictPassed
			if cmdTag.RowsAffected() != 0 {
				verdict = domain.VerdictViolation
			}
			report.AddRecord(domain.IsolationAuditRecord{
				CheckType:       domain.CheckCrossTenantMutation,
				TargetTable:     "accounts",
				Operation:       "UPDATE",
				AuthenticatedAs: attackerTenantID,
				VictimTenantID:  victimTenantID,
				RowsAffected:    cmdTag.RowsAffected(),
				ExpectedAffected: 0,
				Verdict:         verdict,
				Details:         "Cross-tenant UPDATE attempt affected 0 rows due to RLS",
			})
		}

		// Tentativa maliciosa de DELETE em transações do inquilino vítima
		cmdTagDel, delErr := tx.Exec(ctx, "DELETE FROM transactions WHERE tenant_id = $1", victimTenantID)
		if delErr != nil && !errors.Is(delErr, pgx.ErrNoRows) {
			report.AddRecord(domain.IsolationAuditRecord{
				CheckType:       domain.CheckCrossTenantMutation,
				TargetTable:     "transactions",
				Operation:       "DELETE",
				AuthenticatedAs: attackerTenantID,
				VictimTenantID:  victimTenantID,
				RowsAffected:    0,
				ExpectedAffected: 0,
				Verdict:         domain.VerdictPassed,
				Details:         fmt.Sprintf("DELETE rejected by engine error: %v", delErr),
			})
		} else {
			verdict := domain.VerdictPassed
			if cmdTagDel.RowsAffected() != 0 {
				verdict = domain.VerdictViolation
			}
			report.AddRecord(domain.IsolationAuditRecord{
				CheckType:       domain.CheckCrossTenantMutation,
				TargetTable:     "transactions",
				Operation:       "DELETE",
				AuthenticatedAs: attackerTenantID,
				VictimTenantID:  victimTenantID,
				RowsAffected:    cmdTagDel.RowsAffected(),
				ExpectedAffected: 0,
				Verdict:         verdict,
				Details:         "Cross-tenant DELETE attempt affected 0 rows due to RLS",
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during VerifyCrossTenantMutation: %w", err)
	}

	return report, nil
}

// VerifyIntraTenantPrivacy valida a invisibilidade de contas PRIVATE dentro do mesmo condomínio familiar.
func (v *IsolationVerifier) VerifyIntraTenantPrivacy(ctx context.Context, tenantID, ownerUserID, partnerUserID uuid.UUID) (*domain.TenantIsolationReport, error) {
	report := domain.NewTenantIsolationReport(tenantID, "Household", tenantID, "Household")

	// Consulta executada sob a identidade do Usuário Parceiro (partnerUserID)
	err := v.txManager.WithTenantTx(ctx, tenantID, partnerUserID, func(tx pgx.Tx) error {
		var privateAccountsCount int64
		// Consulta contas privadas pertencentes exclusivamente ao ownerUserID
		query := `
			SELECT count(*) FROM accounts 
			WHERE tenant_id = $1 AND owner_user_id = $2 AND visibility = 'PRIVATE'
		`
		if scanErr := tx.QueryRow(ctx, query, tenantID, ownerUserID).Scan(&privateAccountsCount); scanErr != nil {
			return scanErr
		}

		verdict := domain.VerdictPassed
		if privateAccountsCount != 0 {
			verdict = domain.VerdictViolation
		}

		report.AddRecord(domain.IsolationAuditRecord{
			CheckType:       domain.CheckIntraTenantPrivacy,
			TargetTable:     "accounts",
			Operation:       "SELECT",
			AuthenticatedAs: partnerUserID,
			VictimTenantID:  tenantID,
			RecordsObserved: privateAccountsCount,
			ExpectedRecords: 0,
			Verdict:         verdict,
			Details:         "Partner cannot observe PRIVATE accounts owned by another household member",
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during VerifyIntraTenantPrivacy: %w", err)
	}

	return report, nil
}

// VerifyUnauthenticatedAccess valida que consultas fora de WithTenantTx retornam 0 linhas.
func (v *IsolationVerifier) VerifyUnauthenticatedAccess(ctx context.Context, sampleTenantID uuid.UUID) (*domain.TenantIsolationReport, error) {
	report := domain.NewTenantIsolationReport(uuid.Nil, "Unauthenticated", sampleTenantID, "Protected Tenant")

	tables := []string{"accounts", "transactions", "categories"}
	for _, tbl := range tables {
		var count int64
		query := fmt.Sprintf("SELECT count(*) FROM %s WHERE tenant_id = $1", tbl) //nolint:gosec
		if scanErr := v.pool.QueryRow(ctx, query, sampleTenantID).Scan(&count); scanErr != nil {
			return nil, scanErr
		}

		verdict := domain.VerdictPassed
		if count != 0 {
			verdict = domain.VerdictViolation
		}

		report.AddRecord(domain.IsolationAuditRecord{
			CheckType:       domain.CheckUnauthenticatedRead,
			TargetTable:     tbl,
			Operation:       "SELECT",
			AuthenticatedAs: uuid.Nil,
			VictimTenantID:  sampleTenantID,
			RecordsObserved: count,
			ExpectedRecords: 0,
			Verdict:         verdict,
			Details:         "Direct unauthenticated pool query returned zero rows under FORCE RLS",
		})
	}

	return report, nil
}
```

---

## 4. Cenários de Teste de Concorrência e Condições de Corrida (`-race`)

A validação de isolamento multi-tenant sob concorrência intensa é um requisito mandatório para assegurar a inviolabilidade do sistema em produção.

### 4.1 Vetores Críticos de Risco Concorrente
1. **Poluição de Conexões Físicas no Pool (`Pool Session Leak`)**:
   - Uma goroutine adquire a conexão física TCP #1 do pool `pgxpool`, executa `SET LOCAL app.current_tenant_id = Tenant A` e encerra a transação.
   - Imediatamente depois, outra goroutine concorrente adquire a mesma conexão física TCP #1 para o Tenant B.
   - **Risco**: Se `SET LOCAL` não for descartado atomicamente pelo PostgreSQL no commit/rollback, a goroutine do Tenant B poderia herdar indevidamente a sessão do Tenant A.
   - **Mitigação e Asserção**: O teste submete 64 goroutines simultâneas executando queries entrelaçadas em pool restrito (`MaxConns = 10`), forçando centenas de reciclagens da mesma conexão física por segundo.

2. **Deduplicação de Fingerprint sob Ingestão Simultânea**:
   - Goroutines de inquilinos distintos submetem simultaneamente transações idênticas (mesmo montante em `decimal.Decimal`, mesma descrição e data).
   - O teste valida que não ocorrem deadlocks, colisões de chave única cross-tenant ou falsos positivos de deduplicação entre tenants.

### 4.2 Matriz de Concorrência da Suíte

| ID do Cenário | Goroutines | Conexões Máx (`pgxpool`) | Iterações por Goroutine | Operações Mistas | Critério de Sucesso |
|---|---|---|---|---|---|
| `CONC-01` | 32 | 5 | 50 | `WithTenantTx(TenantA, List)` vs `WithTenantTx(TenantB, List)` | 100% dos resultados sem dados cruzados; zero data race (`-race`). |
| `CONC-02` | 64 | 10 | 100 | Transações mistas de Leitura (`SELECT`) e Mutação (`UPDATE balance`) | Balanços em `decimal.Decimal` 100% íntegros; zero contaminação de sessão. |
| `CONC-03` | 16 | 4 | 25 | Inserções paralelas com mesmo fingerprint SHA-256 em Tenant A e Tenant B | Inserção simultânea bem-sucedida em ambos os tenants; isolamento preservado. |

---

## 5. Estrutura e Implementação dos Testes E2E

### 5.1 Teste E2E de Isolamento RLS (`test/e2e/rls_isolation_test.go`)

O arquivo [`test/e2e/rls_isolation_test.go`](file:///home/luismarquitti/repos/intellifinance-go/test/e2e/rls_isolation_test.go) implementa:
1. Conexão determinística via credencial não-privilegiada `intellifinance_app`.
2. Provisionamento de dois tenants canônicos:
   - Tenant A: `Família Marquitti` (`HOUSEHOLD`).
   - Tenant B: `Convidado / Edson Silva` (`PERSONAL`).
3. Inserção de entidades com valores monetários em `decimal.Decimal` e fingerprints SHA-256.
4. Asserções de Invisibilidade Cruzada (contas, transações, categorias, splits, jobs).
5. Asserções de Prova Estrutural do Banco (queries sem `WHERE tenant_id`).
6. Asserções de Bloqueio de Mutações (`UPDATE` e `DELETE` cruzados com 0 linhas afetadas).
7. Asserções de Tentativa de Injeção Maliciosa (`INSERT` com tenant_id fraudado rejeitado pelo banco).
8. Asserções de Privacidade Familiar Intra-Tenant (ocultação de contas `PRIVATE`).
9. Emissão formatada do relatório de auditoria tabular via `t.Logf(report.FormatTable())`.

### 5.2 Teste E2E de Concorrência e Reciclagem (`test/e2e/rls_concurrency_test.go`)

O arquivo [`test/e2e/rls_concurrency_test.go`](file:///home/luismarquitti/repos/intellifinance-go/test/e2e/rls_concurrency_test.go) implementa:
1. Configuração de pool com limite intencional reduzido (`MaxConns = 10`, `MinConns = 2`).
2. Disparo de 64 goroutines sincronizadas via `sync.WaitGroup`.
3. Alternância massiva de transações entre Tenant A e Tenant B com queries de verificação cruzada contínua.
4. Asserção de que nenhuma iteração observou registros do inquilino vizinho.
5. Verificação sob `go test -v -race`.

---

## 6. Verificação Mecânica e Homologação dos 7 Gates Go

Para conformidade com o motor SDLC e aprovação no gate determinístico, a implementação deve atender categoricamente aos 7 gates mecânicos:

```bash
# Gate 1: Verificação de integridade dos módulos Go
go mod verify

# Gate 2: Análise estática do compilador Go
go vet ./...

# Gate 3: Verificação de linter e higiene de código
golangci-lint run ./...

# Gate 4: Zero Floats Policy (proibição de float32/float64 para quantias financeiras)
! git grep -E "\b(float32|float64)\b" -- "internal/domain/tenant_isolation*.go" "internal/ports/tenant_isolation*.go" "test/e2e/*rls*.go"

# Gate 5: Execução da suíte E2E de isolamento RLS com detecção de condições de corrida
go test -v -race -run TestRLS_Isolation ./test/e2e/...

# Gate 6: Execução da suíte de concorrência massiva sob -race
go test -v -race -run TestRLS_Concurrency ./test/e2e/...

# Gate 7: Verificação de segredos e credenciais committed via Gitleaks
gitleaks detect --verbose --source=.
```

---

## 7. Rastreabilidade de Requisitos e Matriz de Cobertura

| Requisito EARS | Componente / Arquivo | Cenário de Teste Mapeado | Status de Cobertura |
|---|---|---|---|
| `REQ-RLS-01` a `04` | `test/e2e/rls_isolation_test.go` | Setup determinístico de Tenant A e Tenant B com `decimal.Decimal` e SHA-256 | Coberto |
| `REQ-RLS-05` a `08` | `test/e2e/rls_isolation_test.go` | `TestRLS_Isolation_CrossTenantRead_ZeroLeakage` | Coberto |
| `REQ-RLS-09` a `11` | `test/e2e/rls_isolation_test.go` | `TestRLS_Isolation_CrossTenantMutation_Blocked` | Coberto |
| `REQ-RLS-12` a `13` | `test/e2e/rls_isolation_test.go` | `TestRLS_Isolation_UnauthenticatedPoolQuery_ZeroRows` | Coberto |
| `REQ-RLS-14` a `15` | `test/e2e/rls_isolation_test.go` | `TestRLS_Isolation_IntraTenantPrivacy_PrivateAccounts` | Coberto |
| `REQ-RLS-16` | `test/e2e/rls_concurrency_test.go` | `TestRLS_Concurrency_PoolMultiplexing_RaceSafe` (64 goroutines) | Coberto |
| `REQ-RLS-17` | `internal/domain/tenant_isolation.go` | Emissão tabular estruturada do `TenantIsolationReport` nos logs | Coberto |

---

A especificação técnica formal determinística para a tarefa **`gh-26`** foi concluída e gravada diretamente no arquivo [`SPEC.md`](file:///home/luismarquitti/repos/intellifinance-go/.flow/tasks/gh-26/SPEC.md), em estrita conformidade com as diretrizes do **Archon** (`01-spec-architect`).

O plano completo de implementação encontra-se no artefato [Implementation Plan: [Phase 5] Validação de Isolamento Criptográfico Multi-Tenant (RLS Testing Suite)](file:///home/luismarquitti/.gemini/antigravity-cli/brain/4a90b7e2-7620-4065-bf95-1bed8b167ae6/spec_rls_isolation_testing_plan.md).

# DIRETRIZES DE CORREÇÃO CIRÚRGICA (Se aplicável)
03-patch-fixer falhou na execução do harness.

# REGRAS DE EXECUÇÃO:
- Implemente ou corrija o código diretamente neste workspace.
- Execute os testes e linters Go locais ('go test -race ./...', 'golangci-lint run') para validar sua entrega.
- Proibido uso de float32/float64 em pacotes de domínio (internal/domain).
- Não deixe código quebrado ou arquivos não compiláveis.
