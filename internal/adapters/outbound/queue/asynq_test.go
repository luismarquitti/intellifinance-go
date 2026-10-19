package queue

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

type clientStub struct {
	enqueue func(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
	close   func() error
}

func (s clientStub) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return s.enqueue(ctx, task, opts...)
}

func (s clientStub) Close() error { return s.close() }

func TestEnqueueRoutesAndLimitsRetries(t *testing.T) {
	for taskType, queueName := range map[ports.TaskType]string{
		ports.TaskTypeIngestStatement:       QueueDefault,
		ports.TaskTypeSyncOpenFinance:       QueueCritical,
		ports.TaskTypeCategorizeTransaction: QueueLow,
	} {
		t.Run(string(taskType), func(t *testing.T) {
			called := false
			adapter := &AsynqQueueAdapter{client: clientStub{enqueue: func(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
				called = true
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
					t.Error("enqueue must use a bounded context")
				}
				if task.Type() != string(taskType) || string(task.Payload()) != "{}" {
					t.Error("task was changed in transport")
				}
				options := make(map[asynq.OptionType]any)
				for _, opt := range opts {
					options[opt.Type()] = opt.Value()
				}
				if options[asynq.QueueOpt] != queueName || options[asynq.MaxRetryOpt] != 5 || options[asynq.TimeoutOpt] != 5*time.Minute {
					t.Errorf("unexpected dispatch options: %v", options)
				}
				return &asynq.TaskInfo{}, nil
			}}}
			if err := adapter.Enqueue(context.Background(), &ports.Task{Type: taskType, Payload: []byte("{}")}); err != nil || !called {
				t.Fatalf("dispatch failed: called=%v err=%v", called, err)
			}
		})
	}
}

func TestEnqueueOptionsDoNotMutateCaller(t *testing.T) {
	task := &ports.Task{ID: "synthetic-task", Type: ports.TaskTypeIngestStatement, Payload: []byte("{}")}
	original := *task
	adapter := &AsynqQueueAdapter{client: clientStub{enqueue: func(_ context.Context, _ *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		options := make(map[asynq.OptionType]any)
		for _, opt := range opts {
			options[opt.Type()] = opt.Value()
		}
		if options[asynq.MaxRetryOpt] != 2 || options[asynq.TimeoutOpt] != time.Minute || options[asynq.ProcessInOpt] != time.Second || options[asynq.TaskIDOpt] != task.ID {
			t.Errorf("explicit options were lost: %v", options)
		}
		return &asynq.TaskInfo{}, nil
	}}}
	if err := adapter.Enqueue(context.Background(), task, ports.WithDelay(time.Second), ports.WithMaxRetries(2), ports.WithTimeout(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, *task) {
		t.Fatal("enqueue mutated the caller's task")
	}
}

func TestEnqueueRejectsInvalidTasks(t *testing.T) {
	adapter := &AsynqQueueAdapter{client: clientStub{enqueue: func(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
		t.Error("invalid task reached Redis")
		return nil, nil
	}}}
	for _, task := range []*ports.Task{
		nil, {Type: "unknown"}, {Type: ports.TaskTypeIngestStatement, MaxRetries: 6},
		{Type: ports.TaskTypeIngestStatement, MaxRetries: -1}, {Type: ports.TaskTypeIngestStatement, Delay: -time.Second},
		{Type: ports.TaskTypeIngestStatement, Timeout: -time.Second},
	} {
		if err := adapter.Enqueue(context.Background(), task); err == nil {
			t.Error("expected invalid dispatch error")
		}
	}
	if err := adapter.Enqueue(context.Background(), &ports.Task{Type: ports.TaskTypeIngestStatement}, nil); err == nil {
		t.Fatal("nil queue option should be rejected")
	}
}

func TestQueuePropagatesErrors(t *testing.T) {
	failure := errors.New("synthetic connection failure")
	adapter := &AsynqQueueAdapter{client: clientStub{
		enqueue: func(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) { return nil, failure },
		close:   func() error { return failure },
	}}
	if err := adapter.Enqueue(context.Background(), &ports.Task{Type: ports.TaskTypeIngestStatement}); !errors.Is(err, failure) {
		t.Fatalf("enqueue error was lost: %v", err)
	}
	if err := adapter.Close(); !errors.Is(err, failure) {
		t.Fatalf("close error was lost: %v", err)
	}
	adapter.client = clientStub{close: func() error { return nil }}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
}
