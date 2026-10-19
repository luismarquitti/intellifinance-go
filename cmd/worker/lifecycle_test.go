package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

func TestStartAfterShutdownIsRejected(t *testing.T) {
	var starts atomic.Int64
	server := &WorkerServer{logger: testLogger(), server: lifecycleStub{
		start: func(asynq.Handler) error { starts.Add(1); return nil },
		stop:  func() {}, shutdown: func() {},
	}}
	server.Shutdown()
	if err := server.Start(); !errors.Is(err, asynq.ErrServerClosed) {
		t.Fatalf("a drained worker must remain closed, got %v", err)
	}
	if starts.Load() != 0 {
		t.Fatal("closed worker started fetching tasks again")
	}
}

func TestStartRejectsAlreadyRunningWorker(t *testing.T) {
	var starts atomic.Int64
	server := &WorkerServer{logger: testLogger(), server: lifecycleStub{
		start: func(asynq.Handler) error { starts.Add(1); return nil },
		stop:  func() {}, shutdown: func() {},
	}}
	t.Cleanup(server.Shutdown)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err == nil {
		t.Fatal("a running worker must reject another start")
	}
	if starts.Load() != 1 {
		t.Fatal("worker started more than one set of processors")
	}
}

func TestConcurrentWorkerLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var starts, stops, shutdowns atomic.Int64
	server := &WorkerServer{logger: testLogger(), server: lifecycleStub{
		start: func(asynq.Handler) error { starts.Add(1); return nil },
		stop:  func() { stops.Add(1) }, shutdown: func() { shutdowns.Add(1) },
	}}
	t.Cleanup(server.Shutdown)
	results := make(chan error, 64)
	for range 64 {
		go func() {
			if err := ctx.Err(); err != nil {
				results <- err
				return
			}
			// A competing start may succeed once or be rejected. Every caller
			// must still be able to wait for the same completed shutdown.
			if err := server.Start(); err != nil {
				server.Shutdown()
				results <- nil
				return
			}
			server.Shutdown()
			results <- nil
		}()
	}
	for range 64 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("concurrent lifecycle calls did not finish")
		}
	}
	if starts.Load() != 1 || stops.Load() != 1 || shutdowns.Load() != 1 {
		t.Fatalf("want one start, stop and shutdown, got %d, %d, %d", starts.Load(), stops.Load(), shutdowns.Load())
	}
	if err := server.Start(); !errors.Is(err, asynq.ErrServerClosed) {
		t.Fatalf("worker reopened after concurrent shutdown: %v", err)
	}
}

func TestStartWhileDrainingIsRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	draining, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var starts atomic.Int64
	server := &WorkerServer{logger: testLogger(), server: lifecycleStub{
		start: func(asynq.Handler) error { starts.Add(1); return nil },
		stop:  func() {},
		shutdown: func() {
			close(draining)
			select {
			case <-release:
			case <-ctx.Done():
			}
		},
	}}
	t.Cleanup(server.Shutdown)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		server.Shutdown()
		close(done)
	}()
	select {
	case <-draining:
	case <-ctx.Done():
		t.Fatal("worker did not start draining")
	}
	result := make(chan error, 1)
	go func() { result <- server.Start() }()
	select {
	case err := <-result:
		if !errors.Is(err, asynq.ErrServerClosed) {
			t.Fatalf("draining worker must reject starts: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("start blocked on in-flight tasks instead of rejecting a closed worker")
	}
	close(release)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("worker did not finish draining")
	}
	if starts.Load() != 1 {
		t.Fatal("worker restarted while draining")
	}
}

func TestFailedStartCanBeRetried(t *testing.T) {
	failure := errors.New("synthetic startup failure")
	var starts atomic.Int64
	server := &WorkerServer{logger: testLogger(), server: lifecycleStub{
		start: func(asynq.Handler) error {
			if starts.Add(1) == 1 {
				return failure
			}
			return nil
		},
		stop: func() {}, shutdown: func() {},
	}}
	t.Cleanup(server.Shutdown)
	if err := server.Start(); !errors.Is(err, failure) {
		t.Fatalf("startup failure was lost: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("failed startup must allow another attempt: %v", err)
	}
	if starts.Load() != 2 {
		t.Fatal("startup was not retried")
	}
}
