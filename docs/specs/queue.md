# IntelliFinance Technical Specification: Worker Job Schemas, Queue Abstractions & State Machine

- **Document ID**: SPEC-005-QUEUE
- **Status**: Authoritative / Production-Ready
- **Hexagonal Role**: Outbound Port (`internal/ports/queue.go`) & Dual Infrastructure Drivers
- **Drivers**: `AsynqQueueAdapter` (Redis 7 on `lm-claw` for Homelab Dev) & `CloudTasksQueueAdapter` (GCP Cloud Tasks for Serverless Prod)
- **Lifecycle Engine**: Deterministic Job State Machine with Dead-Letter Queue (DLQ) & Exponential Backoff

---

## 1. Dual-Driver Queue Architecture

IntelliFinance employs a dual-driver queue design to satisfy two conflicting infrastructure invariants:
1. **Homelab Dev ($0 Cost, Local Bare-Metal)**: Physical host `lm-claw` runs Redis 7. Background workers (`cmd/worker`) pull tasks via Redis streams and sorted sets with instant delivery, zero cloud latency, and complete offline capability.
2. **GCP Serverless Production ($0 Idle Cost)**: Cloud Run instances scale down to zero when idle. Maintaining managed Redis (Cloud Memorystore) incurs a baseline cost of $35–$50/month, violating our zero-idle-cost invariant. Instead, the production adapter targets **Google Cloud Tasks**, which delivers tasks via authenticated HTTP POST requests (`POST /internal/tasks/{task_type}`) to the Cloud Run service, waking instances on demand and utilizing GCP's **1,000,000 free tasks/month** tier.

```
                                  [ Application Service ]
                                             |
                                             v
                                    [ ports.TaskQueue ]
                                             |
                     +-----------------------+-----------------------+
                     |                                               |
             (ENV = development)                             (ENV = production)
                     |                                               |
                     v                                               v
           [ AsynqQueueAdapter ]                          [ CloudTasksQueueAdapter ]
                     |                                               |
           (Redis LPUSH/ZADD)                              (Cloud Tasks API Call)
                     |                                               |
                     v                                               v
             [ Redis 7 Server ]                             [ Google Cloud Tasks ]
             (lm-claw:6379)                                          |
                     |                                        (HTTP POST Webhook)
                     v                                               v
           [ cmd/worker (Daemon) ]                        [ cmd/api (Cloud Run) ]
        (hibiken/asynq Server)                          (/internal/tasks/{name})
                     |                                               |
                     +-----------------------+-----------------------+
                                             |
                                             v
                                  [ Domain Task Handlers ]
                                  - IngestionStatementHandler
                                  - CategorizeTransactionHandler
                                  - SyncOpenFinanceHandler
```

---

## 2. Core Go Interface Contracts (`internal/ports/queue.go`)

```go
package ports

import (
	"context"
	"time"
)

// TaskType identifies the background job handler
type TaskType string

const (
	TaskTypeIngestStatement     TaskType = "task:ingest_statement"
	TaskTypeCategorizeTransaction TaskType = "task:categorize_transaction"
	TaskTypeSyncOpenFinance     TaskType = "task:sync_open_finance"
)

// Task defines the serializable envelope dispatched through the queue
type Task struct {
	ID         string            `json:"id"`
	Type       TaskType          `json:"type"`
	Payload    []byte            `json:"payload"`
	Headers    map[string]string `json:"headers,omitempty"`
	Delay      time.Duration     `json:"delay,omitempty"`
	MaxRetries int               `json:"max_retries,omitempty"`
	Timeout    time.Duration     `json:"timeout,omitempty"`
}

// QueueOption configures task dispatch parameters
type QueueOption func(*Task)

func WithDelay(d time.Duration) QueueOption {
	return func(t *Task) {
		t.Delay = d
	}
}

func WithMaxRetries(retries int) QueueOption {
	return func(t *Task) {
		t.MaxRetries = retries
	}
}

func WithTimeout(timeout time.Duration) QueueOption {
	return func(t *Task) {
		t.Timeout = timeout
	}
}

// TaskQueue is the outbound port for job submission
type TaskQueue interface {
	Enqueue(ctx context.Context, task *Task, opts ...QueueOption) error
	Close() error
}

// TaskHandler defines the business logic execution contract
type TaskHandler interface {
	ProcessTask(ctx context.Context, task *Task) error
}

// TaskRegistry manages handler bindings across queue drivers
type TaskRegistry interface {
	RegisterHandler(taskType TaskType, handler TaskHandler)
	GetHandler(taskType TaskType) (TaskHandler, bool)
}
```

---

## 3. Concrete Adapters

### 3.1 Homelab Dev Adapter: `AsynqQueueAdapter` (`hibiken/asynq`)

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
		return nil, fmt.Errorf("invalid redis uri for asynq: %w", err)
	}
	client := asynq.NewClient(opt)
	return &AsynqQueueAdapter{client: client}, nil
}

func (a *AsynqQueueAdapter) Enqueue(ctx context.Context, task *ports.Task, opts ...ports.QueueOption) error {
	for _, opt := range opts {
		opt(task)
	}

	var asynqOpts []asynq.Option
	if task.ID != "" {
		asynqOpts = append(asynqOpts, asynq.TaskID(task.ID))
	}
	if task.Delay > 0 {
		asynqOpts = append(asynqOpts, asynq.ProcessIn(task.Delay))
	}
	if task.MaxRetries > 0 {
		asynqOpts = append(asynqOpts, asynq.MaxRetry(task.MaxRetries))
	} else {
		asynqOpts = append(asynqOpts, asynq.MaxRetry(5))
	}
	if task.Timeout > 0 {
		asynqOpts = append(asynqOpts, asynq.Timeout(task.Timeout))
	}

	asynqTask := asynq.NewTask(string(task.Type), task.Payload, asynqOpts...)
	_, err := a.client.EnqueueContext(ctx, asynqTask)
	if err != nil {
		return fmt.Errorf("asynq enqueue failed for %s: %w", task.Type, err)
	}
	return nil
}

func (a *AsynqQueueAdapter) Close() error {
	return a.client.Close()
}
```

### 3.2 GCP Serverless Adapter: `CloudTasksQueueAdapter` (`cloud.google.com/go/cloudtasks/apiv2`)

```go
package queue

import (
	"context"
	"fmt"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"intellifinance/internal/ports"
)

type CloudTasksQueueAdapter struct {
	client          *cloudtasks.Client
	projectID       string
	locationID      string
	queueID         string
	serviceURL      string
	serviceAcctMail string
}

func NewCloudTasksQueueAdapter(ctx context.Context, projectID, locationID, queueID, serviceURL, saEmail string) (*CloudTasksQueueAdapter, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud tasks client: %w", err)
	}
	return &CloudTasksQueueAdapter{
		client:          client,
		projectID:       projectID,
		locationID:      locationID,
		queueID:         queueID,
		serviceURL:      serviceURL,
		serviceAcctMail: saEmail,
	}, nil
}

func (c *CloudTasksQueueAdapter) Enqueue(ctx context.Context, task *ports.Task, opts ...ports.QueueOption) error {
	for _, opt := range opts {
		opt(task)
	}

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.projectID, c.locationID, c.queueID)
	targetURL := fmt.Sprintf("%s/internal/tasks/%s", c.serviceURL, string(task.Type))

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Headers: map[string]string{
						"Content-Type": "application/json",
						"X-Task-ID":    task.ID,
					},
					Body: task.Payload,
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: c.serviceAcctMail,
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
	if task.Timeout > 0 {
		req.Task.DispatchDeadline = durationpb.New(task.Timeout)
	}

	_, err := c.client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("cloud tasks dispatch failed: %w", err)
	}
	return nil
}

func (c *CloudTasksQueueAdapter) Close() error {
	return c.client.Close()
}
```

---

## 4. Job Types & JSON Payload Schemas

### 4.1 Statement Ingestion Job (`task:ingest_statement`)
Triggered when a statement file (OFX, CSV, XLSX, PDF) is uploaded. To maintain serverless statelessness across Cloud Run instances, files are addressed via distributed cloud storage URIs rather than ephemeral local disk paths.

```json
{
  "job_id": "99999999-8888-7777-6666-555555555555",
  "tenant_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "account_id": "22222222-3333-4444-5555-666666666666",
  "storage_uri": "gs://intellifinance-ingestion-prod/tenant-aaa/statement_20260924.ofx",
  "file_type": "OFX",
  "source_filename": "statement_20260924.ofx",
  "user_id": "11111111-2222-3333-4444-555555555555",
  "uploaded_at": "2026-09-24T16:30:00Z"
}
```

> **Note on Storage Providers**:
> - **GCP Production**: `storage_uri` points to Google Cloud Storage (`gs://$INGESTION_BUCKET/...`), readable by any autoscaled Cloud Run instance.
> - **Homelab / Dev**: `storage_uri` can point to local MinIO (`s3://...`) or local shared volumes (`file:///var/data/...`).

### 4.2 AI Transaction Categorization Job (`task:categorize_transaction`)
Triggered after batch transaction creation when confidence scores require asynchronous ML inference.

```json
{
  "tenant_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "transaction_ids": [
    "33333333-4444-5555-6666-777777777777",
    "33333333-4444-5555-6666-777777777778"
  ],
  "model_preference": "gemini-2.0-flash",
  "fallback_allowed": true
}
```

### 4.3 Open Finance Bank Sync Job (`task:sync_open_finance`)
Triggered by Pluggy webhook events or scheduled daily cron sweeps.

```json
{
  "item_id": "8a6e87f1-7391-49b4-b4a1-094982635412",
  "tenant_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "sync_type": "INCREMENTAL", // INCREMENTAL (last 7 days) or FULL (last 365 days)
  "from_date": "2026-09-17",
  "to_date": "2026-09-24",
  "triggered_by": "WEBHOOK"
}
```

---

## 5. Job Lifecycle State Machine & Dead-Letter Queue (DLQ)

```
       +---------------------------------------------+
       |                   PENDING                   |
       |  (Enqueued in Redis / Cloud Tasks Queue)    |
       +---------------------------------------------+
                              |
                     (Worker Dispatches)
                              v
       +---------------------------------------------+
       |                 PROCESSING                  |
       |  (Lock acquired, IngestionJob status set)   |
       +---------------------------------------------+
                              |
              +---------------+---------------+
              |                               |
      (Execution Success)             (Transient Failure)
              v                               v
+---------------------------+     +---------------------------+
|         COMPLETED         |     |         RETRYING          |
|  (Metrics written,        |     |  (Exponential backoff:    |
|   status = 'COMPLETED')   |     |   2^attempt * 5s + jitter)|
+---------------------------+     +---------------------------+
                                              |
                                     (Max Retries Exceeded)
                                              v
                                  +---------------------------+
                                  |          FAILED           |
                                  |  (Moved to DLQ,           |
                                  |   status = 'FAILED',      |
                                  |   alert logged to Sentry) |
                                  +---------------------------+
```

### 5.1 Retry Policy & Backoff Calculation

For any job encountering transient failures (network timeouts, bank API rate limiting, database lock contention), retries follow an exponential backoff with jitter:

$$T_{\text{wait}} = \min(T_{\text{max}}, T_{\text{base}} \times 2^{\text{attempt}}) \pm \text{jitter}$$

- $T_{\text{base}} = 5\text{ seconds}$
- $T_{\text{max}} = 300\text{ seconds (5 minutes)}$
- $\text{MaxRetries} = 5$

### 5.2 Dead-Letter Queue (DLQ) Strategy
1. **Asynq DLQ (Homelab Dev)**:
   - When a task exceeds 5 retry attempts, Asynq automatically moves it to the `dead` set.
   - Developers inspect dead jobs via `asynqmon` UI (`http://192.168.3.10:8088`).
2. **Cloud Tasks DLQ (GCP Prod)**:
   - Tasks that return non-2xx status codes after max attempts trigger a Cloud Monitoring alert.
   - Failed payloads are logged to Google Cloud Logging and dead-lettered into a Google Cloud Pub/Sub topic `projects/$PROJECT_ID/topics/intellifinance-tasks-dlq` for operator investigation.

---

## 6. Worker Concurrency & Idempotency Safeguards

1. **Distributed Locks**: To prevent duplicate concurrent execution of sync jobs for the same Open Finance Item, workers acquire a Redis mutex or database advisory lock (`pg_try_advisory_xact_lock(hashtext(item_id))`).
2. **Idempotent Transaction Inserts**: Even if a worker crashes and restarts mid-batch, the database unique constraint `(tenant_id, fingerprint)` guarantees zero duplicate rows.
3. **Graceful Shutdown**: The worker daemon intercepts `SIGTERM` and `SIGINT`, halts dequeueing, and allows in-flight jobs a 30-second grace window to finish before exiting.
