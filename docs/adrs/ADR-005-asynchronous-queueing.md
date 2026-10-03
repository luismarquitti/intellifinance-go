# ADR-005: Asynchronous Processing & Dual-Driver Queueing Abstraction (Asynq vs Cloud Tasks)

- **Status**: Accepted
- **Deciders**: Software Architect (Worker M3), Principal Engineer, Cloud Infrastructure Lead
- **Date**: 2026-09-24
- **Technical Story**: Milestone 3 - Architecture Decision Records

---

## 1. Context & Problem Statement

Financial document ingestion (multi-thousand-row CSV exports, SGML OFX files, multimodal PDF bank statements processed via Vertex AI) and Open Finance bank synchronization are high-latency, I/O-intensive operations. Processing these workloads synchronously inside an HTTP request-response lifecycle creates major failure modes:
1. **HTTP Request Timeouts**: Reverse proxies, API gateways, and web browsers enforce connection timeouts (typically 15 to 60 seconds). Complex PDF processing or high-latency bank API calls will trigger premature HTTP 504 Gateway Timeout errors.
2. **Resource Exhaustion**: Synchronous document parsing ties up HTTP worker goroutines, degrading API responsiveness for other household members.
3. **Lack of Resilient Retries**: If an external AI gateway or banking API returns a temporary rate-limit (HTTP 429) or connection reset, an in-flight HTTP request fails completely, requiring manual user re-upload.

However, selecting an asynchronous queueing architecture for IntelliFinance faces a fundamental conflict between deployment targets:
- **Homelab Bare-Metal (`lm-claw`)**: A Redis 7 instance is already running 24/7 at physical address `192.168.3.10:6379`. Running a dedicated background worker daemon (`cmd/worker`) pulling tasks from Redis has **zero incremental dollar cost**, provides microsecond queueing latency, and allows real-time task inspection via tools like Asynqmon.
- **GCP Serverless Production**: Google Cloud Run is designed to scale down to zero instances when idle. Running a persistent worker polling a queue keeps container instances permanently active, incurring constant compute costs (~$35/month). Furthermore, hosting a managed Redis instance on GCP (Cloud Memorystore) requires a baseline idle cost of **$35 to $50 per month** even when completely unused, violating our **Zero-Cost Personal Operation** invariant.

---

## 2. Decision Drivers

- **Zero-Cost Idle Operation in Production**: Production infrastructure must cost $0.00/month when no ingestion tasks are active, utilizing serverless scale-to-zero capabilities.
- **Offline & Self-Contained in Homelab**: Local development must function 100% offline on bare-metal hardware (`lm-claw`) without requiring GCP network access, service accounts, or internet connectivity.
- **Unified Domain Abstraction**: Application use cases must interact with a single interface contract (`ports.TaskQueue`), oblivious to whether the task is transported via Redis or Cloud Tasks.
- **Guaranteed At-Least-Once Delivery & Exponential Retries**: Automated retries with jitter for transient AI and Open Finance failures, culminating in dead-letter failure states.
- **Secure Serverless Webhook Ingestion**: Cloud Tasks HTTP triggers must be cryptographically verified using Google OpenID Connect (OIDC) identity tokens.

---

## 3. Considered Options

1. **Persistent Redis + Asynq in Both Environments** (Requires GCP Memorystore at ~$40/month in production).
2. **RabbitMQ / Apache Kafka** (Massive resource footprint, entirely unsuitable for serverless and low-power hardware).
3. **Google Cloud Pub/Sub** (Requires continuous push/pull subscriptions; high configuration complexity for simple task execution).
4. **Dual-Driver Queue Abstraction: `AsynqQueueAdapter` (Homelab) & `CloudTasksQueueAdapter` (GCP Prod) (Selected)**.

---

## 4. Evaluation & Comparative Matrix

| Evaluation Criteria | Dual-Driver Abstraction (Selected) | Single Asynq + GCP Memorystore | Google Cloud Pub/Sub | RabbitMQ / NATS |
|---|---|---|---|---|
| **GCP Idle Cost** | **$0.00 / month** (Cloud Tasks free tier: 1M tasks/mo) | **$35 - $50 / month** (Memorystore idle charge) | ~$0.00 (Free tier, but complex) | High (Requires VM or Cloud Run Min Instances) |
| **Cloud Run Scale-to-Zero** | **Yes**: Cloud Tasks wakes Cloud Run on demand via HTTP | **No**: Persistent worker daemon prevents scale-to-zero | Requires push subscription with complex push config | No |
| **Homelab Offline Capability** | **100% Offline** (Redis on `lm-claw:6379`) | 100% Offline | **Impossible** (Requires active GCP connection) | 100% Offline |
| **Application Coupling** | **Zero** (Abstracted via `ports.TaskQueue`) | Bound to Asynq/Redis SDK | Bound to Google Pub/Sub SDK | Bound to AMQP/NATS SDK |
| **Observability** | Asynqmon locally; Cloud Tasks Console in GCP | Asynqmon everywhere | Cloud Monitoring metrics | RabbitMQ Management Console |
| **Retry & Backoff Logic** | Native exponential backoff in both drivers | Native in Asynq | Native in Pub/Sub dead-letter topics | Requires broker dead-letter exchange configuration |

---

## 5. Decision Outcome

**Adopt a Dual-Driver Queueing Abstraction** implementing the `ports.TaskQueue` interface:
1. **Homelab Development Driver**: `AsynqQueueAdapter` backed by Redis 7 and consumed by a standalone background daemon (`cmd/worker`).
2. **GCP Serverless Production Driver**: `CloudTasksQueueAdapter` backed by Google Cloud Tasks. Cloud Tasks dispatches tasks as authenticated HTTP POST requests directly to the Cloud Run API service (`cmd/api`), waking instances on demand and scaling to zero when idle.

### Dual-Target Architecture Flow:

```
                            [ Use Case: SubmitStatement ]
                                          │
                                          ▼
                                 [ ports.TaskQueue ]
                                          │
                    ┌─────────────────────┴─────────────────────┐
                    │                                           │
         (QUEUE_DRIVER=asynq)                        (QUEUE_DRIVER=cloudtasks)
                    │                                           │
                    ▼                                           ▼
          [ AsynqQueueAdapter ]                       [ CloudTasksQueueAdapter ]
                    │                                           │
             (LPUSH / ZADD)                                (cloudtasks.CreateTask)
                    │                                           │
                    ▼                                           ▼
            [ Redis 7 Server ]                         [ GCP Cloud Tasks Queue ]
            (on lm-claw:6379)                                   │
                    │                                    (HTTP POST Webhook)
                    ▼                                    (OIDC Authenticated)
        [ cmd/worker (Daemon) ]                                 │
     (hibiken/asynq Worker Server)                              ▼
                    │                                 [ cmd/api (Cloud Run) ]
                    │                                (/internal/tasks/ingest)
                    │                                           │
                    └─────────────────────┬─────────────────────┘
                                          │
                                          ▼
                             [ IngestionTaskHandler ]
                                          │
                               (Parses Statement File,
                              Invokes AI Categorization,
                                Persists Transactions)
```

---

## 6. Core Interface Contracts (`internal/ports/queue.go`)

```go
package ports

import (
	"context"
	"time"
)

type Task struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`       // e.g. "task:ingest_file", "task:categorize_tx"
	Payload    []byte            `json:"payload"`    // JSON serialized parameters
	Headers    map[string]string `json:"headers,omitempty"`
	Delay      time.Duration     `json:"delay,omitempty"`
	MaxRetries int               `json:"max_retries,omitempty"`
}

type TaskQueue interface {
	Enqueue(ctx context.Context, task Task) error
}

type TaskHandler interface {
	ProcessTask(ctx context.Context, task Task) error
}
```

---

## 7. Concrete Driver Implementations

### 7.1 Homelab Driver: `AsynqQueueAdapter` (`internal/adapters/outbound/queue/asynq.go`)

```go
package queue

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"intellifinance/internal/ports"
)

type AsynqQueueAdapter struct {
	client *asynq.Client
}

func NewAsynqQueueAdapter(redisURL string) (*AsynqQueueAdapter, error) {
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}
	return &AsynqQueueAdapter{client: asynq.NewClient(opt)}, nil
}

func (a *AsynqQueueAdapter) Enqueue(ctx context.Context, task ports.Task) error {
	asynqTask := asynq.NewTask(task.Type, task.Payload)
	var opts []asynq.Option

	if task.Delay > 0 {
		opts = append(opts, asynq.ProcessIn(task.Delay))
	}
	if task.MaxRetries > 0 {
		opts = append(opts, asynq.MaxRetry(task.MaxRetries))
	} else {
		opts = append(opts, asynq.MaxRetry(5))
	}

	_, err := a.client.EnqueueContext(ctx, asynqTask, opts...)
	if err != nil {
		return fmt.Errorf("failed to enqueue asynq task: %w", err)
	}
	return nil
}
```

### 7.2 GCP Serverless Driver: `CloudTasksQueueAdapter` (`internal/adapters/outbound/queue/cloudtasks.go`)

```go
package queue

import (
	"context"
	"fmt"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"intellifinance/internal/ports"
)

type CloudTasksQueueAdapter struct {
	client       *cloudtasks.Client
	projectID    string
	location     string
	queueID      string
	serviceURL   string
	saEmail      string
}

func NewCloudTasksQueueAdapter(ctx context.Context, projectID, location, queueID, serviceURL, saEmail string) (*CloudTasksQueueAdapter, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud tasks client: %w", err)
	}
	return &CloudTasksQueueAdapter{
		client:     client,
		projectID:  projectID,
		location:   location,
		queueID:    queueID,
		serviceURL: serviceURL,
		saEmail:    saEmail,
	}, nil
}

func (c *CloudTasksQueueAdapter) Enqueue(ctx context.Context, task ports.Task) error {
	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.projectID, c.location, c.queueID)
	targetURL := fmt.Sprintf("%s/internal/tasks/%s", c.serviceURL, task.Type)

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Body:       task.Payload,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: c.saEmail,
							Audience:            c.serviceURL,
						},
					},
				},
			},
		},
	}

	if task.Delay > 0 {
		req.Task.ScheduleTime = timestamppb.New(time.Now().Add(task.Delay))
	}

	_, err := c.client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create cloud task: %w", err)
	}
	return nil
}
```

---

## 8. Ingestion Task State Machine

All asynchronous tasks follow a deterministic state machine recorded in the `ingestion_jobs` database table:

```
          [ File Upload ]
                 │
                 ▼
         ┌───────────────┐
         │    PENDING    │ (Record created in DB, enqueued to TaskQueue)
         └───────┬───────┘
                 │
                 ▼ (Worker picks up task / Cloud Tasks delivers webhook)
         ┌───────────────┐
         │  PROCESSING   │ (File downloaded, parsing & AI categorization active)
         └───────┬───────┘
                 │
       ┌─────────┴─────────┐
       │ (Success)         │ (Unrecoverable Error / Max Retries Exceeded)
       ▼                   ▼
┌──────────────┐    ┌──────────────┐
│  COMPLETED   │    │    FAILED    │
└──────────────┘    └──────────────┘
(Transactions       (Error recorded in error_details JSONB;
committed to DB)     user notified in UI)
```

---

## 9. Security & Webhook Verification

In production, Google Cloud Tasks dispatches HTTP requests to `/internal/tasks/{task_name}`. This endpoint is protected against unauthorized invocation using **Google OIDC Token Validation**:
1. Cloud Tasks generates an OpenID Connect (OIDC) JWT signed by Google, containing the configured service account email and audience.
2. The Chi middleware (`internal/adapters/inbound/http/middleware/oidc.go`) validates the token's cryptographic signature using Google's public JWKS (`https://www.googleapis.com/oauth2/v3/certs`).
3. Requests missing a valid token or with a mismatched audience are rejected immediately with HTTP 401 Unauthorized.

---

## 10. Consequences

### Positive Consequences
- **True $0 Production Cost**: Google Cloud Tasks includes **1,000,000 free task dispatches per month**. Coupled with Cloud Run scaling to zero, asynchronous processing incurs zero baseline monthly charges.
- **Immediate Homelab Iteration**: Local developers test queues using bare-metal Redis without incurring cloud latency or internet dependencies.
- **Resilient Batch Processing**: Failed tasks automatically retry with exponential backoff (e.g. 5s, 15s, 45s, 120s, 300s).
- **Strict Decoupling**: Business services enqueue jobs through `ports.TaskQueue` without knowing which infrastructure backend is active.

### Negative Consequences
- **Dual Codebase Maintenance**: Requires maintaining and testing two queue adapter implementations (`AsynqQueueAdapter` and `CloudTasksQueueAdapter`).
- **Webhook Endpoint Exposure**: In production, Cloud Run must expose internal webhook routes (`/internal/tasks/*`), requiring rigorous OIDC signature verification.

### Neutral Consequences
- Local development requires running Redis (provided via Docker Compose or native Debian service on `lm-claw`).

---

## 11. Implementation & Verification Plan

1. **Verify Homelab Asynq Pipeline**:
   - Start local Redis container: `docker compose up -d redis`.
   - Start worker: `go run ./cmd/worker/main.go`.
   - Submit a test task and observe processing logs in worker console.
2. **Verify Cloud Tasks Dispatch & OIDC Authentication**:
   - Deploy to Cloud Run staging environment.
   - Dispatch an asynchronous ingestion task using `CloudTasksQueueAdapter`.
   - Verify Cloud Tasks console shows successful task execution (HTTP 200) and Cloud Run scales up to process and back down to zero.
3. **Verify State Machine Transitions**:
   - Run integration tests asserting that `ingestion_jobs.status` transitions from `PENDING` -> `PROCESSING` -> `COMPLETED`.
