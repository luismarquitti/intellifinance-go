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
# Especificação Técnica Formal — [Phase 3] Automação de Deploy Serverless no GCP (Cloud Run, Cloud Tasks)

- **Task ID**: `gh-20` (Issue #20)
- **Fase**: `Phase 3 — GCP Serverless & Cloud Portability`
- **Arquiteto Responsável**: Archon (`01-spec-architect`)
- **Artefato de Planejamento**: [Implementation Plan: [Phase 3] Automação de Deploy Serverless no GCP (Cloud Run, Cloud Tasks)](file:///home/luismarquitti/.gemini/antigravity-cli/brain/6fb974ff-27b3-4a44-a219-7fbe319798dc/spec_gcp_serverless_deploy_plan.md)
- **Arquivos Alvo**:
  - [`scripts/deploy-gcp.sh`](file:///home/luismarquitti/repos/intellifinance-go/scripts/deploy-gcp.sh) (Script de automação e deploy contínuo)
  - [`cloudbuild.yaml`](file:///home/luismarquitti/repos/intellifinance-go/cloudbuild.yaml) (Template declarativo do Google Cloud Build)
  - [`scripts/deploy_test.go`](file:///home/luismarquitti/repos/intellifinance-go/scripts/deploy_test.go) (Testes automatizados de validação sintática e flags)
  - [`internal/domain/gcp_config.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/gcp_config.go) (Modelo de domínio e validação de invariantes serverless)
  - [`internal/domain/gcp_config_test.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/domain/gcp_config_test.go) (Testes de invariantes e concorrência)
  - [`internal/ports/health.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/ports/health.go) (Contratos de porta de saúde e diagnósticos)
  - [`internal/adapters/inbound/http/handlers/health_handler.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/adapters/inbound/http/handlers/health_handler.go) (Handler HTTP de healthcheck)
  - [`internal/adapters/inbound/http/handlers/health_handler_test.go`](file:///home/luismarquitti/repos/intellifinance-go/internal/adapters/inbound/http/handlers/health_handler_test.go) (Testes de concorrência HTTP sob `-race`)

---

## 1. Visão Geral e Invariantes Arquiteturais

Esta especificação estabelece os contratos determinísticos, requisitos não-funcionais, automações em shell e validações de domínio em Go para o provisionamento e deploy contínuo dos serviços serverless do **IntelliFinance** no **Google Cloud Platform (GCP)**.

### 1.1 Invariantes Inegociáveis (Red Lines)

1. **Garantia Estrita de Scale-to-Zero ($0 Custo Ocioso)**:
   - O serviço Cloud Run DEVE ser configurado estritamente com `--min-instances=0`.
   - É **expressamente proibido** permitir `--min-instances > 0` na configuração de produção. Quando ociosa, a aplicação não deve alocar nenhuma vCPU ou memória em execução contínua, garantindo faturamento nulo ($0) de infraestrutura.
   - O teto de escalabilidade é limitado a `--max-instances=2`, `--memory=512Mi`, `--cpu=1` e `--concurrency=80` para salvaguardar custos e evitar exaustão não autorizada de orçamento.

2. **Proibição Absoluta de Ponto Flutuante (`Zero Floats`)**:
   - É **terminantemente proibido** o uso de `float32` ou `float64` em qualquer contrato Go, cálculo de custo estimado de infraestrutura, taxa de despacho de filas ou telemetria de latência/uptime.
   - Qualquer quantia financeira ou razão deve usar [`decimal.Decimal`](file:///home/luismarquitti/repos/intellifinance-go/go.mod#L13) (`github.com/shopspring/decimal`).
   - Métricas de tempo utilizam estritamente `time.Duration` ou inteiros em milissegundos/segundos (`int64`).

3. **Mínimo Privilégio de Segurança (Least Privilege IAM)**:
   - Segregação mandatória entre a Service Account de Runtime (`intellifinance-api`) e a Service Account de Despacho de Tarefas (`intellifinance-tasks-invoker`).
   - A Service Account da aplicação (`intellifinance-api`) DEVE possuir estritamente:
     - `roles/secretmanager.secretAccessor` (leitura de segredos no Secret Manager)
     - `roles/cloudtasks.enqueuer` (despacho de tarefas nas filas Cloud Tasks)
     - `roles/logging.logWriter` (escrita de logs estruturados)
     - `roles/cloudtrace.agent` (envio de telemetria e traces)
     - `roles/iam.serviceAccountUser` restrito à SA invoker (para atrelar tokens OIDC às requisições)
   - A Service Account invoker (`intellifinance-tasks-invoker`) DEVE possuir estritamente:
     - `roles/run.invoker` sobre o serviço Cloud Run `intellifinance-api`.
   - É **proibida** a concessão de privilégios amplos como `roles/editor` ou `roles/owner`.

4. **Idempotência Estrita no Provisionamento**:
   - Todo comando de infraestrutura (criação de repositório no Artifact Registry, criação de Service Accounts, concessão de IAM bindings, criação/atualização de filas Cloud Tasks) DEVE ser idempotente, verificando a existência prévia do recurso antes de tentar criá-lo, evitando falhas em execuções repetidas.

5. **Testabilidade Offline Determinística e Concorrência Segura (`-race`)**:
   - O pipeline mecânico e os testes unitários Go DEVEM executar 100% offline, sem necessidade de credenciais GCP ativas em ambiente local ou CI/CD.
   - O script `scripts/deploy-gcp.sh` DEVE oferecer a flag `--dry-run` para simulação segura e validação sintática estática via Go test (`scripts/deploy_test.go`).
   - Handlers de saúde e structs de configuração DEVEM ser *stateless* ou sincronizados via primitivas de concorrência (`sync.RWMutex`), homologados sob 64 goroutines simultâneas com `go test -race`.

6. **Encerramento Gracioso de Instâncias (Graceful Shutdown)**:
   - O runtime da aplicação DEVE interceptar `SIGTERM` (sinal enviado pelo Cloud Run antes de desligar ou escalar uma instância para zero) e `SIGINT`, permitindo uma janela de até 15 segundos para dreno e finalização de requisições em voo antes do encerramento forçado.

---

## 2. Requisitos Funcionais e Não-Funcionais no Formato EARS

### 2.1 Verificação de Ambiente e Pré-Requisitos
- **EARS-01 (Verificação de CLI e Autenticação)**: **Quando** o script `scripts/deploy-gcp.sh` for iniciado, **se** o utilitário `gcloud` não estiver instalado no `PATH` **ou** se não houver um usuário/Service Account autenticado ativo, o sistema **DEVE** abortar a execução imediatamente com código de erro não-zero (`exit 1`) e emitir uma mensagem de orientação descritiva.
- **EARS-02 (Verificação e Habilitação de APIs do GCP)**: **Quando** o provisionamento de infraestrutura for executado, **se** qualquer uma das APIs obrigatórias (`run.googleapis.com`, `cloudbuild.googleapis.com`, `artifactregistry.googleapis.com`, `cloudtasks.googleapis.com`, `secretmanager.googleapis.com`) não estiver ativada no projeto GCP, o sistema **DEVE** habilitá-la de forma idempotente via `gcloud services enable`.

### 2.2 Gerenciamento do Artifact Registry e Imagem de Contêiner
- **EARS-03 (Idempotência do Repositório Artifact Registry)**: **Quando** o script verificar o repositório de contêineres `${AR_REPO}` (padrão `"intellifinance"`), **se** o repositório Docker não existir na região configurada `${GCP_REGION}`, o sistema **DEVE** criá-lo automaticamente via `gcloud artifacts repositories create --repository-format=docker`; **se** já existir, o sistema **DEVE** prosseguir sem erro.
- **EARS-04 (Build Imutável via Google Cloud Build)**: **Quando** o build for disparado, **se** a flag `--skip-build` não estiver presente, o sistema **DEVE** submeter o diretório raiz ao Google Cloud Build via `gcloud builds submit`, marcando a imagem com a tag canônica do commit git (`${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}/${SERVICE_NAME}:${COMMIT_SHA}`) e com a tag flutuante `:latest`.

### 2.3 Gestão de Identidade e Permissões de Mínimo Privilégio (IAM)
- **EARS-05 (Criação Idempotente de Service Accounts)**: **Quando** as contas de serviço forem configuradas, **se** a SA da aplicação `intellifinance-api@${PROJECT_ID}.iam.gserviceaccount.com` **ou** a SA invoker `intellifinance-tasks-invoker@${PROJECT_ID}.iam.gserviceaccount.com` não existirem, o sistema **DEVE** criá-las via `gcloud iam service-accounts create`.
- **EARS-06 (Aplicação de Papéis Mínimos à SA da Aplicação)**: **Quando** as permissões da SA `intellifinance-api` forem atribuídas, **se** os bindings de papéis forem processados, o sistema **DEVE** conceder estritamente `roles/secretmanager.secretAccessor`, `roles/cloudtasks.enqueuer`, `roles/logging.logWriter` e `roles/cloudtrace.agent` no projeto, além de `roles/iam.serviceAccountUser` sobre a SA `intellifinance-tasks-invoker`.
- **EARS-07 (Aplicação de Papel de Invocação Cloud Run)**: **Quando** as permissões da SA `intellifinance-tasks-invoker` forem atribuídas, o sistema **DEVE** conceder estritamente `roles/run.invoker` sobre o serviço Cloud Run `intellifinance-api`, impedindo acessos externos não autorizados aos endpoints de background tasks.

### 2.4 Provisionamento das Filas do Google Cloud Tasks
- **EARS-08 (Criação Idempotente de Filas Cloud Tasks)**: **Quando** o provisionamento de mensageria for disparado, **se** as filas `q-ingest`, `q-categorize`, `q-sync` e `default` não existirem na região de destino, o sistema **DEVE** criá-las via `gcloud tasks queues create`; **se** já existirem, o sistema **DEVE** atualizar suas propriedades operacionais via `gcloud tasks queues update`.
- **EARS-09 (Configuração de Taxas e Backoff de Filas)**: **Quando** as filas Cloud Tasks forem provisionadas, o sistema **DEVE** fixar deterministicamente:
  - `--max-concurrent-dispatches=10` (proteção de saturação do Cloud Run)
  - `--max-attempts=5` (limite defensivo de retentativas)
  - `--min-backoff=5s` (retardamento inicial para erros transitórios)
  - `--max-backoff=300s` (teto de backoff exponencial de 5 minutos)
  - `--max-doublings=4` (duplicações de intervalo de retry)

### 2.5 Deploy Contínuo no Cloud Run com Scale-to-Zero
- **EARS-10 (Parâmetros Rígidos de Scale-to-Zero)**: **Quando** o comando `gcloud run deploy` for executado, o sistema **DEVE** injetar obrigatoriamente:
  - `--min-instances=0` (garantia de scale-to-zero e $0 custo ocioso)
  - `--max-instances=2` (limite máximo de réplicas simultâneas)
  - `--memory=512Mi`
  - `--cpu=1`
  - `--concurrency=80` (concorrência por instância)
  - `--timeout=60s` (deadline padrão de requisições HTTP)
  - `--platform=managed`
  - `--allow-unauthenticated` (acesso público à API REST)
  - `--service-account=intellifinance-api@${PROJECT_ID}.iam.gserviceaccount.com`
- **EARS-11 (Injeção de Variáveis e Mapeamento de Segredos)**: **Quando** o Cloud Run for implantado, o sistema **DEVE** configurar:
  - Variáveis de ambiente: `APP_ENV=production`, `QUEUE_DRIVER=cloudtasks`, `GCP_PROJECT_ID=${PROJECT_ID}`, `GCP_LOCATION=${REGION}`, `PORT=8080`
  - Mapeamento de segredos via Secret Manager (`--set-secrets`):
    - `DATABASE_URL=DATABASE_URL:latest`
    - `JWT_SECRET=JWT_SECRET:latest`
    - `PLUGGY_CLIENT_ID=PLUGGY_CLIENT_ID:latest`
    - `PLUGGY_CLIENT_SECRET=PLUGGY_CLIENT_SECRET:latest`
    - `PLUGGY_WEBHOOK_SECRET=PLUGGY_WEBHOOK_SECRET:latest`
    - `WEBHOOK_SECRET=WEBHOOK_SECRET:latest`
- **EARS-12 (Verificação Pós-Deploy da Anotação Knative `minScale`)**: **Quando** o deploy no Cloud Run finalizar, o sistema **DEVE** consultar a revisão ativa via `gcloud run services describe` e validar que a anotação `autoscaling.knative.dev/minScale` é igual a `"0"`. **Se** o valor for superior a zero, o sistema **DEVE** emitir um alarme crítico e falhar a verificação.

### 2.6 Validação de Saúde (Healthcheck) e Resiliência
- **EARS-13 (Polling Automatizado de Healthcheck HTTP 200)**: **Quando** o deploy for concluído com sucesso, o sistema **DEVE** executar requisições HTTP periódicas contra a rota de probe do serviço implantado (`${SERVICE_URL}/healthz` ou `${SERVICE_URL}/health`) por até 120 segundos com intervalo de 3 a 5 segundos. **Se** a resposta retornar status `HTTP 200 OK` com payload JSON contendo status `"UP"`, o deploy **DEVE** ser aprovado; **se** o tempo expirar sem resposta 200, o sistema **DEVE** retornar código de erro e registrar os logs das últimas requisições.
- **EARS-14 (Simulação Não-Destrutiva via `--dry-run`)**: **Quando** o script for invocado com a flag `--dry-run`, o sistema **DEVE** imprimir todos os comandos `gcloud` que seriam executados, validar as variáveis de ambiente e checar a sintaxe, sem executar mutações reais na infraestrutura do GCP.

### 2.7 Contratos Go, Precisão Numérica e Concorrência Segura
- **EARS-15 (Validação de Domínio de Parâmetros de Deploy)**: **Quando** a struct de domínio `CloudRunScaleToZeroConfig.Validate()` for chamada em Go, **se** `MinInstances > 0`, o sistema **DEVE** retornar o erro determinístico `ErrScaleToZeroViolation`; **se** `MaxInstances < 1` ou `MaxInstances > 10`, **DEVE** retornar erro de limite de concorrência.
- **EARS-16 (Invariante Contábil de Custo Ocioso Zero)**: **Quando** o método `CloudRunScaleToZeroConfig.EstimateIdleCost()` for chamado, o sistema **DEVE** retornar estritamente `decimal.Zero` (`decimal.Decimal`), garantindo conformidade com a política *Zero Floats*.
- **EARS-17 (Contrato de Healthcheck em Go)**: **Quando** o endpoint `GET /healthz` ou `GET /health` for acionado, o handler Go **DEVE** responder com `HTTP 200 OK`, `Content-Type: application/json` e payload com status `"UP"`, informando uptime e latência em inteiros exatos (`int64`).
- **EARS-18 (Dreno Gracioso sob `SIGTERM` sem Corridas)**: **Quando** o processo da aplicação receber o sinal `SIGTERM` sob alta carga de requisições concorrentes, o servidor HTTP **DEVE** interromper a recepção de novas conexões e drenar as requisições em andamento com timeout de 15 segundos, completando o encerramento com código de saída 0 e sem acionar o detector de corridas (`go test -race`).

---

## 3. Arquitetura de Arquivos e Contratos de Interfaces Go

### 3.1 Mapeamento dos Arquivos do Pacote

```
intellifinance-go/
├── scripts/
│   ├── deploy-gcp.sh               # Script principal executável de automação e deploy contínuo
│   └── deploy_test.go              # Testes unitários do script (validação sintática e flags)
├── cloudbuild.yaml                 # Template declarativo para Google Cloud Build
├── internal/
│   ├── domain/
│   │   ├── gcp_config.go           # Modelos de domínio para parâmetros serverless e scale-to-zero
│   │   └── gcp_config_test.go      # Testes de invariantes e concorrência do modelo de domínio
│   ├── ports/
│   │   └── health.go               # Contratos de porta para sondas de integridade do sistema
│   └── adapters/
│       └── inbound/
│           └── http/
│               └── handlers/
│                   ├── health_handler.go      # Handler HTTP para /health, /healthz, /readyz
│                   └── health_handler_test.go # Testes concorrentes do endpoint de saúde
```

---

### 3.2 Contratos de Porta de Diagnóstico e Saúde (`internal/ports/health.go`)

```go
package ports

import (
	"context"
	"time"
)

// HealthStatus represents the operational status of the service or subsystem.
type HealthStatus string

const (
	// HealthStatusUp indicates the component is fully operational.
	HealthStatusUp HealthStatus = "UP"
	// HealthStatusDegraded indicates non-critical subsystem degradation.
	HealthStatusDegraded HealthStatus = "DEGRADED"
	// HealthStatusDown indicates the component is non-operational.
	HealthStatusDown HealthStatus = "DOWN"
)

// SubsystemHealth captures diagnostics for an individual external dependency.
type SubsystemHealth struct {
	Name      string       `json:"name"`
	Status    HealthStatus `json:"status"`
	LatencyMs int64        `json:"latency_ms"` // Zero floats: exact integer milliseconds
	Details   string       `json:"details,omitempty"`
}

// HealthReport encapsulates the complete health evaluation of the runtime.
type HealthReport struct {
	Status      HealthStatus      `json:"status"`
	Version     string            `json:"version"`
	Environment string            `json:"environment"`
	CommitSHA   string            `json:"commit_sha"`
	UptimeSec   int64             `json:"uptime_sec"` // Zero floats: exact integer seconds
	Timestamp   time.Time         `json:"timestamp"`
	Components  []SubsystemHealth `json:"components,omitempty"`
}

// HealthChecker defines the port contract for liveness, readiness, and diagnostic checks.
type HealthChecker interface {
	// Check produces a full diagnostic evaluation of all critical subsystems.
	Check(ctx context.Context) HealthReport
	// Liveness returns true if the application process is running and responsive.
	Liveness(ctx context.Context) bool
	// Readiness returns true if the application is ready to accept incoming user traffic.
	Readiness(ctx context.Context) bool
}
```

---

### 3.3 Modelo de Domínio e Invariantes Serverless (`internal/domain/gcp_config.go`)

```go
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	// ErrScaleToZeroViolation is returned when minimum instances is greater than zero.
	ErrScaleToZeroViolation = errors.New("scale-to-zero violation: MinInstances must strictly be 0 for $0 idle cost")
	// ErrInvalidMaxInstances is returned when maximum instances exceeds safe operational limits.
	ErrInvalidMaxInstances = errors.New("invalid MaxInstances: must be between 1 and 10")
	// ErrInsufficientMemory is returned when container memory is less than 256MiB.
	ErrInsufficientMemory = errors.New("insufficient memory: must be at least 256 MiB")
	// ErrInvalidCPU is returned when CPU allocation is zero or negative.
	ErrInvalidCPU = errors.New("invalid CPU: must allocate at least 1 vCPU")
	// ErrMissingProjectID is returned when GCP project ID is empty.
	ErrMissingProjectID = errors.New("missing GCP Project ID")
	// ErrMissingRegion is returned when GCP region is empty.
	ErrMissingRegion = errors.New("missing GCP Region")
)

// CloudRunScaleToZeroConfig defines deterministic parameters for GCP Cloud Run serverless deployment.
type CloudRunScaleToZeroConfig struct {
	ProjectID    string
	Region       string
	ServiceName  string
	MinInstances int64
	MaxInstances int64
	MemoryMB     int64
	CPU          int64
	Concurrency  int64
	Timeout      time.Duration
}

// NewDefaultCloudRunConfig returns production-safe serverless configuration with strict scale-to-zero.
func NewDefaultCloudRunConfig(projectID, region string) CloudRunScaleToZeroConfig {
	return CloudRunScaleToZeroConfig{
		ProjectID:    strings.TrimSpace(projectID),
		Region:       strings.TrimSpace(region),
		ServiceName:  "intellifinance-api",
		MinInstances: 0, // Strict $0 idle cost invariant
		MaxInstances: 2,
		MemoryMB:     512,
		CPU:          1,
		Concurrency:  80,
		Timeout:      60 * time.Second,
	}
}

// Validate enforces serverless cost and architectural invariants.
func (c *CloudRunScaleToZeroConfig) Validate() error {
	if c.ProjectID == "" {
		return ErrMissingProjectID
	}
	if c.Region == "" {
		return ErrMissingRegion
	}
	if c.MinInstances != 0 {
		return ErrScaleToZeroViolation
	}
	if c.MaxInstances < 1 || c.MaxInstances > 10 {
		return ErrInvalidMaxInstances
	}
	if c.MemoryMB < 256 {
		return ErrInsufficientMemory
	}
	if c.CPU < 1 {
		return ErrInvalidCPU
	}
	if c.Timeout <= 0 || c.Timeout > 300*time.Second {
		return fmt.Errorf("invalid timeout %v: must be between 1s and 300s", c.Timeout)
	}
	return nil
}

// EstimateIdleCost returns the monetary cost of running this configuration when idle.
// Enforces the Zero Floats invariant: strictly returns decimal.Zero.
func (c *CloudRunScaleToZeroConfig) EstimateIdleCost() decimal.Decimal {
	if c.MinInstances == 0 {
		return decimal.Zero
	}
	return decimal.RequireFromString("0.00")
}

// CloudTasksQueueSpec specifies background task queue rate limits and backoff rules.
type CloudTasksQueueSpec struct {
	Name                    string
	Location                string
	MaxDispatchesPerSecond  decimal.Decimal // Zero Floats: exact decimal rate
	MaxBurstSize            int64
	MaxConcurrentDispatches int64
	MaxAttempts             int64
	MinBackoff              time.Duration
	MaxBackoff              time.Duration
	MaxDoublings            int64
}

// DefaultProductionQueues returns standard queues required by IntelliFinance.
func DefaultProductionQueues(location string) []CloudTasksQueueSpec {
	return []CloudTasksQueueSpec{
		{
			Name:                    "q-ingest",
			Location:                location,
			MaxDispatchesPerSecond:  decimal.NewFromInt(10),
			MaxBurstSize:            20,
			MaxConcurrentDispatches: 10,
			MaxAttempts:             5,
			MinBackoff:              5 * time.Second,
			MaxBackoff:              300 * time.Second,
			MaxDoublings:            4,
		},
		{
			Name:                    "q-categorize",
			Location:                location,
			MaxDispatchesPerSecond:  decimal.NewFromInt(5),
			MaxBurstSize:            10,
			MaxConcurrentDispatches: 5,
			MaxAttempts:             3,
			MinBackoff:              5 * time.Second,
			MaxBackoff:              180 * time.Second,
			MaxDoublings:            3,
		},
		{
			Name:                    "q-sync",
			Location:                location,
			MaxDispatchesPerSecond:  decimal.NewFromInt(5),
			MaxBurstSize:            10,
			MaxConcurrentDispatches: 5,
			MaxAttempts:             5,
			MinBackoff:              10 * time.Second,
			MaxBackoff:              300 * time.Second,
			MaxDoublings:            4,
		},
		{
			Name:                    "default",
			Location:                location,
			MaxDispatchesPerSecond:  decimal.NewFromInt(10),
			MaxBurstSize:            20,
			MaxConcurrentDispatches: 10,
			MaxAttempts:             5,
			MinBackoff:              5 * time.Second,
			MaxBackoff:              300 * time.Second,
			MaxDoublings:            4,
		},
	}
}
```

---

### 3.4 Handler HTTP de Saúde e Probes (`internal/adapters/inbound/http/handlers/health_handler.go`)

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// HealthHandler serves health and readiness probes for Cloud Run and load balancers.
type HealthHandler struct {
	mu            sync.RWMutex
	startTime     time.Time
	version       string
	commitSHA     string
	environment   string
	checker       ports.HealthChecker
	isShuttingDown atomic.Bool
}

// NewHealthHandler constructs a thread-safe HealthHandler.
func NewHealthHandler(version, commitSHA, environment string, checker ports.HealthChecker) *HealthHandler {
	return &HealthHandler{
		startTime:   time.Now(),
		version:     version,
		commitSHA:   commitSHA,
		environment: environment,
		checker:     checker,
	}
}

// SetShuttingDown marks the server as in draining mode, causing readiness probes to fail.
func (h *HealthHandler) SetShuttingDown() {
	h.isShuttingDown.Store(true)
}

// HandleHealthz responds with HTTP 200 OK if the process is responsive (Liveness probe).
func (h *HealthHandler) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP"}`))
}

// HandleReadyz responds with HTTP 200 OK if ready for traffic, or 503 Service Unavailable if draining.
func (h *HealthHandler) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.isShuttingDown.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"DRAINING"}`))
		return
	}
	if h.checker != nil && !h.checker.Readiness(r.Context()) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"NOT_READY"}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"READY"}`))
}

// HandleHealth serves comprehensive diagnostic report under GET /health.
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uptime := int64(time.Since(h.startTime).Seconds()) // Zero Floats: integer seconds

	var report ports.HealthReport
	if h.checker != nil {
		report = h.checker.Check(r.Context())
		if report.UptimeSec == 0 {
			report.UptimeSec = uptime
		}
	} else {
		report = ports.HealthReport{
			Status:      ports.HealthStatusUp,
			Version:     h.version,
			Environment: h.environment,
			CommitSHA:   h.commitSHA,
			UptimeSec:   uptime,
			Timestamp:   time.Now().UTC(),
		}
	}

	statusCode := http.StatusOK
	if report.Status == ports.HealthStatusDown {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(report)
}
```

---

## 4. Especificação Detalhada do Script `scripts/deploy-gcp.sh` e `cloudbuild.yaml`

### 4.1 Código Canônico do `scripts/deploy-gcp.sh`

```bash
#!/usr/bin/env bash
# scripts/deploy-gcp.sh
# Automação de provisionamento e deploy contínuo serverless no Google Cloud com scale-to-zero ($0 custo ocioso).
set -euo pipefail

# ------------------------------------------------------------------------------
# 1. Configurações e Variáveis Padrão
# ------------------------------------------------------------------------------
GCP_PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null || echo "intellifinance-prod")}"
GCP_REGION="${GCP_REGION:-us-central1}"
SERVICE_NAME="${SERVICE_NAME:-intellifinance-api}"
AR_REPO="${AR_REPO:-intellifinance}"
COMMIT_SHA="${COMMIT_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo "manual-$(date +%s)")}"
IMAGE_TAG="${IMAGE_TAG:-${COMMIT_SHA}}"
IMAGE_URI="${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/${AR_REPO}/${SERVICE_NAME}:${IMAGE_TAG}"
IMAGE_LATEST="${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/${AR_REPO}/${SERVICE_NAME}:latest"

APP_SA="intellifinance-api@${GCP_PROJECT_ID}.iam.gserviceaccount.com"
INVOKER_SA="intellifinance-tasks-invoker@${GCP_PROJECT_ID}.iam.gserviceaccount.com"

# Scale-to-Zero Guard: Inegociável para garantir custo $0 ocioso
MIN_INSTANCES=0
MAX_INSTANCES=2
MEMORY="512Mi"
CPU="1"
TIMEOUT="60s"
CONCURRENCY=80

DRY_RUN=false
SKIP_BUILD=false
SKIP_INFRA=false

# ------------------------------------------------------------------------------
# 2. Parsing de Argumentos da Linha de Comando
# ------------------------------------------------------------------------------
usage() {
  cat <<EOF
Uso: $0 [OPÇÕES]
Opções:
  --project <ID>       Define o GCP Project ID (atual: ${GCP_PROJECT_ID})
  --region <REGIAO>    Define a região GCP (atual: ${GCP_REGION})
  --tag <TAG>          Tag customizada da imagem Docker (atual: ${IMAGE_TAG})
  --skip-build         Pula o estágio de compilação da imagem no Cloud Build
  --skip-infra         Pula a verificação/criação de Artifact Registry, IAM e Filas
  --dry-run            Imprime os comandos que seriam executados sem aplicar alterações
  -h, --help           Exibe esta mensagem de ajuda
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --project) GCP_PROJECT_ID="$2"; shift 2 ;;
    --region) GCP_REGION="$2"; shift 2 ;;
    --tag) IMAGE_TAG="$2"; shift 2 ;;
    --skip-build) SKIP_BUILD=true; shift ;;
    --skip-infra) SKIP_INFRA=true; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    -h|--help) usage ;;
    *) echo "Erro: Opção desconhecida: $1" >&2; exit 1 ;;
  esac
done

execute_cmd() {
  if [ "$DRY_RUN" = true ]; then
    echo "[DRY-RUN] $*"
  else
    "$@"
  fi
}

log_info() { echo -e "\033[1;34m[INFO]\033[0m $*"; }
log_success() { echo -e "\033[1;32m[SUCCESS]\033[0m $*"; }
log_warn() { echo -e "\033[1;33m[WARN]\033[0m $*"; }
log_error() { echo -e "\033[1;31m[ERROR]\033[0m $*" >&2; }

# ------------------------------------------------------------------------------
# 3. Pré-Requisitos e APIs
# ------------------------------------------------------------------------------
check_prerequisites() {
  log_info "Validando pré-requisitos locais e ferramentas..."
  if ! command -v gcloud &>/dev/null; then
    log_error "CLI 'gcloud' não encontrado. Instale o Google Cloud SDK."
    exit 1
  fi

  if [ "$DRY_RUN" = false ]; then
    local active_account
    active_account="$(gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null || true)"
    if [ -z "$active_account" ]; then
      log_error "Nenhuma conta ativa autenticada no gcloud. Execute 'gcloud auth login'."
      exit 1
    fi
    log_info "Conta ativa autenticada: ${active_account}"
  fi

  log_info "GCP Project: ${GCP_PROJECT_ID} | Região: ${GCP_REGION}"
  
  if [ "$SKIP_INFRA" = false ]; then
    log_info "Verificando APIs essenciais do GCP..."
    local required_apis=(
      run.googleapis.com
      cloudbuild.googleapis.com
      artifactregistry.googleapis.com
      cloudtasks.googleapis.com
      secretmanager.googleapis.com
    )
    for api in "${required_apis[@]}"; do
      execute_cmd gcloud services enable "${api}" --project="${GCP_PROJECT_ID}"
    done
  fi
}

# ------------------------------------------------------------------------------
# 4. Repositório no Artifact Registry
# ------------------------------------------------------------------------------
setup_artifact_registry() {
  [ "$SKIP_INFRA" = true ] && return 0
  log_info "Configurando repositório no Artifact Registry (${AR_REPO})..."

  if [ "$DRY_RUN" = true ]; then
    echo "[DRY-RUN] gcloud artifacts repositories describe ${AR_REPO} --location=${GCP_REGION} --project=${GCP_PROJECT_ID} || gcloud artifacts repositories create ${AR_REPO} ..."
    return 0
  fi

  if ! gcloud artifacts repositories describe "${AR_REPO}" --location="${GCP_REGION}" --project="${GCP_PROJECT_ID}" &>/dev/null; then
    log_info "Criando repositório '${AR_REPO}' no formato Docker..."
    gcloud artifacts repositories create "${AR_REPO}" \
      --repository-format=docker \
      --location="${GCP_REGION}" \
      --description="IntelliFinance Docker Artifacts" \
      --project="${GCP_PROJECT_ID}"
  else
    log_info "Repositório '${AR_REPO}' já existe no Artifact Registry."
  fi
}

# ------------------------------------------------------------------------------
# 5. Service Accounts e Mínimo Privilégio IAM
# ------------------------------------------------------------------------------
setup_iam_service_accounts() {
  [ "$SKIP_INFRA" = true ] && return 0
  log_info "Configurando Service Accounts e permissões de Mínimo Privilégio (IAM)..."

  # 1. SA da Aplicação (Runtime Cloud Run)
  if [ "$DRY_RUN" = false ]; then
    if ! gcloud iam service-accounts describe "${APP_SA}" --project="${GCP_PROJECT_ID}" &>/dev/null; then
      log_info "Criando Service Account da aplicação: ${APP_SA}..."
      gcloud iam service-accounts create intellifinance-api \
        --display-name="IntelliFinance Cloud Run Runtime SA" \
        --project="${GCP_PROJECT_ID}"
    fi
  fi

  # 2. SA do Invoker (Cloud Tasks Dispatcher)
  if [ "$DRY_RUN" = false ]; then
    if ! gcloud iam service-accounts describe "${INVOKER_SA}" --project="${GCP_PROJECT_ID}" &>/dev/null; then
      log_info "Criando Service Account do Cloud Tasks Invoker: ${INVOKER_SA}..."
      gcloud iam service-accounts create intellifinance-tasks-invoker \
        --display-name="IntelliFinance Cloud Tasks OIDC Invoker SA" \
        --project="${GCP_PROJECT_ID}"
    fi
  fi

  # 3. Permissões estritas da SA da Aplicação
  local app_roles=(
    roles/secretmanager.secretAccessor
    roles/cloudtasks.enqueuer
    roles/logging.logWriter
    roles/cloudtrace.agent
  )
  for role in "${app_roles[@]}"; do
    execute_cmd gcloud projects add-iam-policy-binding "${GCP_PROJECT_ID}" \
      --member="serviceAccount:${APP_SA}" \
      --role="${role}" \
      --condition=None --quiet >/dev/null
  done

  # 4. Permissão para a SA da aplicação atrelar identidade da SA invoker ao enfileirar tarefas
  execute_cmd gcloud iam service-accounts add-iam-policy-binding "${INVOKER_SA}" \
    --member="serviceAccount:${APP_SA}" \
    --role="roles/iam.serviceAccountUser" \
    --project="${GCP_PROJECT_ID}" --quiet >/dev/null
}

# ------------------------------------------------------------------------------
# 6. Filas do Google Cloud Tasks
# ------------------------------------------------------------------------------
setup_cloud_tasks_queues() {
  [ "$SKIP_INFRA" = true ] && return 0
  log_info "Configurando filas de background jobs no Google Cloud Tasks..."

  local queues=(q-ingest q-categorize q-sync default)
  for q in "${queues[@]}"; do
    if [ "$DRY_RUN" = true ]; then
      echo "[DRY-RUN] gcloud tasks queues describe ${q} --location=${GCP_REGION} || gcloud tasks queues create ${q} ..."
      continue
    fi

    if ! gcloud tasks queues describe "${q}" --location="${GCP_REGION}" --project="${GCP_PROJECT_ID}" &>/dev/null; then
      log_info "Criando fila '${q}' no Cloud Tasks..."
      gcloud tasks queues create "${q}" \
        --location="${GCP_REGION}" \
        --max-concurrent-dispatches=10 \
        --max-attempts=5 \
        --min-backoff=5s \
        --max-backoff=300s \
        --max-doublings=4 \
        --project="${GCP_PROJECT_ID}"
    else
      log_info "Fila '${q}' já existe. Atualizando limites operacionais..."
      gcloud tasks queues update "${q}" \
        --location="${GCP_REGION}" \
        --max-concurrent-dispatches=10 \
        --max-attempts=5 \
        --min-backoff=5s \
        --max-backoff=300s \
        --max-doublings=4 \
        --project="${GCP_PROJECT_ID}"
    fi
  done
}

# ------------------------------------------------------------------------------
# 7. Compilação e Envio da Imagem via Google Cloud Build
# ------------------------------------------------------------------------------
build_and_push_image() {
  if [ "$SKIP_BUILD" = true ]; then
    log_info "Pulando etapa de compilação da imagem (--skip-build)."
    return 0
  fi

  log_info "Submetendo imagem ao Google Cloud Build: ${IMAGE_URI}..."
  execute_cmd gcloud builds submit \
    --project="${GCP_PROJECT_ID}" \
    --tag="${IMAGE_URI}" \
    --substitutions="COMMIT_SHA=${COMMIT_SHA}" \
    .
}

# ------------------------------------------------------------------------------
# 8. Deploy do Cloud Run com Scale-to-Zero ($0 Custo Ocioso)
# ------------------------------------------------------------------------------
deploy_cloud_run() {
  log_info "Implantando serviço Cloud Run '${SERVICE_NAME}' com scale-to-zero..."

  execute_cmd gcloud run deploy "${SERVICE_NAME}" \
    --project="${GCP_PROJECT_ID}" \
    --region="${GCP_REGION}" \
    --image="${IMAGE_URI}" \
    --platform=managed \
    --service-account="${APP_SA}" \
    --allow-unauthenticated \
    --min-instances="${MIN_INSTANCES}" \
    --max-instances="${MAX_INSTANCES}" \
    --memory="${MEMORY}" \
    --cpu="${CPU}" \
    --concurrency="${CONCURRENCY}" \
    --timeout="${TIMEOUT}" \
    --ingress=all \
    --execution-environment=gen2 \
    --set-env-vars="APP_ENV=production,QUEUE_DRIVER=cloudtasks,GCP_PROJECT_ID=${GCP_PROJECT_ID},GCP_LOCATION=${GCP_REGION},PORT=8080" \
    --set-secrets="DATABASE_URL=DATABASE_URL:latest,JWT_SECRET=JWT_SECRET:latest,PLUGGY_CLIENT_ID=PLUGGY_CLIENT_ID:latest,PLUGGY_CLIENT_SECRET=PLUGGY_CLIENT_SECRET:latest,PLUGGY_WEBHOOK_SECRET=PLUGGY_WEBHOOK_SECRET:latest,WEBHOOK_SECRET=WEBHOOK_SECRET:latest"

  execute_cmd gcloud run services add-iam-policy-binding "${SERVICE_NAME}" \
    --project="${GCP_PROJECT_ID}" \
    --region="${GCP_REGION}" \
    --member="serviceAccount:${INVOKER_SA}" \
    --role="roles/run.invoker" \
    --quiet >/dev/null
}

# ------------------------------------------------------------------------------
# 9. Verificação Pós-Deploy: Scale-to-Zero e Healthcheck HTTP 200
# ------------------------------------------------------------------------------
verify_post_deploy() {
  [ "$DRY_RUN" = true ] && { log_info "Modo dry-run ativo. Pulando verificações remotas."; return 0; }

  log_info "Verificando anotação de scale-to-zero (minScale=0)..."
  local min_scale
  min_scale="$(gcloud run services describe "${SERVICE_NAME}" \
    --project="${GCP_PROJECT_ID}" \
    --region="${GCP_REGION}" \
    --format='value(spec.template.metadata.annotations["autoscaling.knative.dev/minScale"])' 2>/dev/null || echo "0")"

  if [ -n "$min_scale" ] && [ "$min_scale" != "0" ]; then
    log_error "Violação de Invariante: minScale é '${min_scale}', esperado '0'! Risco de custos ociosos!"
    exit 1
  fi
  log_success "Scale-to-zero validado com sucesso: minScale=0 ($0 custo ocioso garantido)."

  local service_url
  service_url="$(gcloud run services describe "${SERVICE_NAME}" \
    --project="${GCP_PROJECT_ID}" \
    --region="${GCP_REGION}" \
    --format='value(status.url)')"

  log_info "Sondando endpoint de healthcheck: ${service_url}/healthz..."
  local attempts=0
  local max_attempts=24 # 24 * 5s = 120 segundos de timeout
  local health_ok=false

  while [ $attempts -lt $max_attempts ]; do
    attempts=$((attempts + 1))
    local http_code
    http_code="$(curl -s -o /dev/null -w "%{http_code}" "${service_url}/healthz" || true)"
    if [ "$http_code" = "200" ]; then
      health_ok=true
      break
    fi
    log_info "Tentativa ${attempts}/${max_attempts}: HTTP ${http_code}. Aguardando 5s..."
    sleep 5
  done

  if [ "$health_ok" = true ]; then
    log_success "Deploy concluído com sucesso! Healthcheck HTTP 200 OK em ${service_url}/healthz"
  else
    log_error "Timeout aguardando Healthcheck HTTP 200 em ${service_url}/healthz!"
    exit 1
  fi
}

# ------------------------------------------------------------------------------
# 10. Fluxo Principal
# ------------------------------------------------------------------------------
main() {
  log_info "=== Iniciando Pipeline de Deploy Serverless GCP IntelliFinance ==="
  check_prerequisites
  setup_artifact_registry
  setup_iam_service_accounts
  setup_cloud_tasks_queues
  build_and_push_image
  deploy_cloud_run
  verify_post_deploy
  log_success "=== Deploy Finalizado com Sucesso ==="
}

main "$@"
```

---

### 4.2 Template Declarativo `cloudbuild.yaml`

```yaml
# cloudbuild.yaml
# Template determinístico para Google Cloud Build com integração ao Artifact Registry e Cloud Run.
steps:
  # 1. Compilar a imagem Docker distroless utilizando cache otimizado
  - name: 'gcr.io/cloud-builders/docker'
    args:
      - 'build'
      - '-t'
      - '${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_AR_REPO}/${_SERVICE_NAME}:${SHORT_SHA}'
      - '-t'
      - '${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_AR_REPO}/${_SERVICE_NAME}:latest'
      - '.'
    id: 'build-image'

  # 2. Publicar a imagem no Artifact Registry
  - name: 'gcr.io/cloud-builders/docker'
    args:
      - 'push'
      - '--all-tags'
      - '${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_AR_REPO}/${_SERVICE_NAME}'
    id: 'push-image'

  # 3. Deploy no Google Cloud Run com parâmetros de Scale-to-Zero
  - name: 'gcr.io/google.com/cloudsdktool/cloud-sdk'
    entrypoint: 'gcloud'
    args:
      - 'run'
      - 'deploy'
      - '${_SERVICE_NAME}'
      - '--project=${PROJECT_ID}'
      - '--region=${_REGION}'
      - '--image=${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_AR_REPO}/${_SERVICE_NAME}:${SHORT_SHA}'
      - '--platform=managed'
      - '--service-account=intellifinance-api@${PROJECT_ID}.iam.gserviceaccount.com'
      - '--allow-unauthenticated'
      - '--min-instances=0'
      - '--max-instances=2'
      - '--memory=512Mi'
      - '--cpu=1'
      - '--concurrency=80'
      - '--timeout=60s'
      - '--ingress=all'
      - '--execution-environment=gen2'
      - '--set-env-vars=APP_ENV=production,QUEUE_DRIVER=cloudtasks,GCP_PROJECT_ID=${PROJECT_ID},GCP_LOCATION=${_REGION},PORT=8080'
      - '--set-secrets=DATABASE_URL=DATABASE_URL:latest,JWT_SECRET=JWT_SECRET:latest,PLUGGY_CLIENT_ID=PLUGGY_CLIENT_ID:latest,PLUGGY_CLIENT_SECRET=PLUGGY_CLIENT_SECRET:latest,PLUGGY_WEBHOOK_SECRET=PLUGGY_WEBHOOK_SECRET:latest,WEBHOOK_SECRET=WEBHOOK_SECRET:latest'
    id: 'deploy-cloud-run'

substitutions:
  _REGION: 'us-central1'
  _AR_REPO: 'intellifinance'
  _SERVICE_NAME: 'intellifinance-api'

images:
  - '${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_AR_REPO}/${_SERVICE_NAME}:${SHORT_SHA}'
  - '${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_AR_REPO}/${_SERVICE_NAME}:latest'

options:
  logging: CLOUD_LOGGING_ONLY
```

---

## 5. Cenários de Teste de Concorrência e Condições de Corrida (`-race`)

### 5.1 Testes de Domínio e Invariantes Serverless (`internal/domain/gcp_config_test.go`)

```go
package domain_test

import (
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

func TestCloudRunConfig_ScaleToZeroInvariant(t *testing.T) {
	t.Parallel()

	t.Run("Valid default configuration passes validation", func(t *testing.T) {
		cfg := domain.NewDefaultCloudRunConfig("proj-123", "us-central1")
		require.NoError(t, cfg.Validate())
		assert.Equal(t, int64(0), cfg.MinInstances)
		assert.Equal(t, int64(2), cfg.MaxInstances)
		assert.True(t, cfg.EstimateIdleCost().Equal(decimal.Zero), "Idle cost must be exactly $0")
	})

	t.Run("Rejects MinInstances > 0 with ErrScaleToZeroViolation", func(t *testing.T) {
		cfg := domain.NewDefaultCloudRunConfig("proj-123", "us-central1")
		cfg.MinInstances = 1
		err := cfg.Validate()
		require.ErrorIs(t, err, domain.ErrScaleToZeroViolation)
	})

	t.Run("Rejects invalid MaxInstances", func(t *testing.T) {
		cfg := domain.NewDefaultCloudRunConfig("proj-123", "us-central1")
		cfg.MaxInstances = 0
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidMaxInstances)

		cfg.MaxInstances = 11
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidMaxInstances)
	})

	t.Run("Rejects missing ProjectID or Region", func(t *testing.T) {
		cfg1 := domain.NewDefaultCloudRunConfig("", "us-central1")
		assert.ErrorIs(t, cfg1.Validate(), domain.ErrMissingProjectID)

		cfg2 := domain.NewDefaultCloudRunConfig("proj-123", "")
		assert.ErrorIs(t, cfg2.Validate(), domain.ErrMissingRegion)
	})
}

func TestCloudRunConfig_ConcurrentReads_RaceFree(t *testing.T) {
	t.Parallel()

	cfg := domain.NewDefaultCloudRunConfig("prod-project", "southamerica-east1")
	require.NoError(t, cfg.Validate())

	const goroutines = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				assert.Equal(t, int64(0), cfg.MinInstances)
				assert.True(t, cfg.EstimateIdleCost().IsZero())
				_ = cfg.Validate()
			}
		}()
	}

	wg.Wait()
}

func TestCloudTasksQueues_DefaultSpecs(t *testing.T) {
	t.Parallel()

	queues := domain.DefaultProductionQueues("us-central1")
	assert.Len(t, queues, 4)

	names := make(map[string]bool)
	for _, q := range queues {
		names[q.Name] = true
		assert.False(t, q.MaxDispatchesPerSecond.IsZero(), "Queue must have positive rate limit")
		assert.Greater(t, q.MaxAttempts, int64(0))
		assert.Greater(t, q.MinBackoff, time.Duration(0))
		assert.GreaterOrEqual(t, q.MaxBackoff, q.MinBackoff)
	}

	assert.True(t, names["q-ingest"])
	assert.True(t, names["q-categorize"])
	assert.True(t, names["q-sync"])
	assert.True(t, names["default"])
}
```

---

### 5.2 Testes Automatizados do Script de Deploy (`scripts/deploy_test.go`)

```go
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findScriptPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)

	candidate := filepath.Join(wd, "deploy-gcp.sh")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	candidate = filepath.Join(wd, "..", "scripts", "deploy-gcp.sh")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	t.Fatalf("Could not locate scripts/deploy-gcp.sh from %s", wd)
	return ""
}

func TestDeployScript_BashSyntaxValidation(t *testing.T) {
	t.Parallel()

	scriptPath := findScriptPath(t)

	cmd := exec.Command("bash", "-n", scriptPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Bash syntax check failed: %s", string(output))
}

func TestDeployScript_MandatoryScaleToZeroFlags(t *testing.T) {
	t.Parallel()

	scriptPath := findScriptPath(t)
	contentBytes, err := os.ReadFile(scriptPath)
	require.NoError(t, err)
	content := string(contentBytes)

	assert.Contains(t, content, "--min-instances=\"${MIN_INSTANCES}\"")
	assert.Contains(t, content, "MIN_INSTANCES=0")
	assert.Contains(t, content, "--max-instances=\"${MAX_INSTANCES}\"")
	assert.Contains(t, content, "MAX_INSTANCES=2")
	assert.Contains(t, content, "--memory=\"${MEMORY}\"")
	assert.Contains(t, content, "--cpu=\"${CPU}\"")
	assert.Contains(t, content, "autoscaling.knative.dev/minScale")

	assert.Contains(t, content, "intellifinance-api@")
	assert.Contains(t, content, "intellifinance-tasks-invoker@")
	assert.Contains(t, content, "roles/secretmanager.secretAccessor")
	assert.Contains(t, content, "roles/cloudtasks.enqueuer")
	assert.Contains(t, content, "roles/run.invoker")

	assert.Contains(t, content, "q-ingest")
	assert.Contains(t, content, "q-categorize")
	assert.Contains(t, content, "q-sync")
}

func TestDeployScript_DryRunExecution(t *testing.T) {
	t.Parallel()

	scriptPath := findScriptPath(t)

	cmd := exec.Command("bash", scriptPath, "--dry-run", "--project", "test-project-ci", "--region", "us-central1")
	outputBytes, err := cmd.CombinedOutput()
	output := string(outputBytes)

	require.NoError(t, err, "Dry-run execution failed: %s", output)
	assert.Contains(t, output, "[DRY-RUN]")
	assert.Contains(t, output, "gcloud run deploy")
	assert.Contains(t, output, "test-project-ci")
}
```

---

### 5.3 Testes Concorrentes do HealthHandler (`internal/adapters/inbound/http/handlers/health_handler_test.go`)

```go
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

type mockHealthChecker struct {
	isReady bool
}

func (m *mockHealthChecker) Check(ctx context.Context) ports.HealthReport {
	return ports.HealthReport{
		Status:      ports.HealthStatusUp,
		Version:     "1.0.0",
		Environment: "test",
		CommitSHA:   "abcdef1",
		UptimeSec:   120,
	}
}

func (m *mockHealthChecker) Liveness(ctx context.Context) bool  { return true }
func (m *mockHealthChecker) Readiness(ctx context.Context) bool { return m.isReady }

func TestHealthHandler_ConcurrentRequests_RaceFree(t *testing.T) {
	t.Parallel()

	checker := &mockHealthChecker{isReady: true}
	handler := handlers.NewHealthHandler("1.0.0", "abcdef1", "production", checker)

	const goroutines = 64
	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			handler.HandleHealthz(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}()
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()
			handler.HandleReadyz(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}()
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			handler.HandleHealth(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}()
	}

	wg.Wait()
}

func TestHealthHandler_GracefulShutdownDraining(t *testing.T) {
	t.Parallel()

	checker := &mockHealthChecker{isReady: true}
	handler := handlers.NewHealthHandler("1.0.0", "abcdef1", "production", checker)

	recNormal := httptest.NewRecorder()
	handler.HandleReadyz(recNormal, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assert.Equal(t, http.StatusOK, recNormal.Code)

	handler.SetShuttingDown()

	recDraining := httptest.NewRecorder()
	handler.HandleReadyz(recDraining, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assert.Equal(t, http.StatusServiceUnavailable, recDraining.Code)
	assert.Contains(t, recDraining.Body.String(), "DRAINING")

	recLiveness := httptest.NewRecorder()
	handler.HandleHealthz(recLiveness, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, recLiveness.Code)
}
```

---

## 6. Matriz de Homologação Mecânica

| Identificador | Verificação / Comando | Condição de Sucesso |
| :--- | :--- | :--- |
| **GATE-01** | `bash -n scripts/deploy-gcp.sh` | Retorno 0; sem erros sintáticos de Bash |
| **GATE-02** | `bash scripts/deploy-gcp.sh --dry-run` | Retorno 0; saída estruturada sem execução de comandos destrutivos |
| **GATE-03** | `go test -race -v -count=1 ./scripts/...` | Retorno 0; asserção das flags `--min-instances=0` e `--max-instances=2` |
| **GATE-04** | `go test -race -v -count=1 ./internal/domain/...` | Retorno 0; invariante contábil $0 custo ocioso (`decimal.Zero`) validada sem condições de corrida |
| **GATE-05** | `go test -race -v -count=1 ./internal/adapters/inbound/http/handlers/...` | Retorno 0; endpoints `/healthz`, `/readyz` e `/health` aprovados sob 64 goroutines |
| **GATE-06** | `golangci-lint run ./...` | Retorno 0; conformidade GoDoc, sem avisos de segurança ou complexidade |
| **GATE-07** | `grep -rn "float32\|float64" internal/domain/gcp_config* internal/ports/health*` | Retorno vazio (zero ocorrências de float em métricas ou custos) |

---

A especificação técnica formal completa foi gravada diretamente no arquivo [`SPEC.md`](file:///home/luismarquitti/repos/intellifinance-go/.flow/tasks/gh-20/SPEC.md), em estrita conformidade com as diretrizes do **Archon** (`01-spec-architect`).

# DIRETRIZES DE CORREÇÃO CIRÚRGICA (Se aplicável)
03-patch-fixer falhou na execução do harness.

# REGRAS DE EXECUÇÃO:
- Implemente ou corrija o código diretamente neste workspace.
- Execute os testes e linters Go locais ('go test -race ./...', 'golangci-lint run') para validar sua entrega.
- Proibido uso de float32/float64 em pacotes de domínio (internal/domain).
- Não deixe código quebrado ou arquivos não compiláveis.
