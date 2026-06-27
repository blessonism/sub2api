package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestConversationCaptureWorkerPoolRunsTask(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.ConversationCapture.WorkerCount = 1
	cfg.Gateway.ConversationCapture.QueueSize = 2
	cfg.Gateway.ConversationCapture.TaskTimeoutSeconds = 1
	pool := NewConversationCaptureWorkerPool(cfg)
	defer pool.Stop()

	done := make(chan struct{})
	ok := pool.TrySubmit(func(context.Context) error {
		close(done)
		return nil
	})

	require.True(t, ok)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task did not run")
	}
}

func TestConversationCaptureWorkerPoolRejectsAfterStop(t *testing.T) {
	pool := NewConversationCaptureWorkerPool(&config.Config{})
	pool.Stop()

	ok := pool.TrySubmit(func(context.Context) error { return nil })

	require.False(t, ok)
	require.Equal(t, int64(1), pool.Stats().Dropped)
}

func TestConversationCaptureWorkerPoolDropsWhenQueueFull(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.ConversationCapture.WorkerCount = 1
	cfg.Gateway.ConversationCapture.QueueSize = 1
	cfg.Gateway.ConversationCapture.TaskTimeoutSeconds = 1
	pool := NewConversationCaptureWorkerPool(cfg)
	defer pool.Stop()

	block := make(chan struct{})
	started := make(chan struct{})
	secondDone := make(chan struct{})

	require.True(t, pool.TrySubmit(func(context.Context) error {
		close(started)
		<-block
		return nil
	}))
	<-started

	require.True(t, pool.TrySubmit(func(context.Context) error {
		close(secondDone)
		return nil
	}))
	require.False(t, pool.TrySubmit(func(context.Context) error { return nil }))
	require.Equal(t, int64(1), pool.Stats().Dropped)

	close(block)
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("queued task did not run")
	}
}

func TestConversationCaptureWorkerPoolStopDrainsSubmittedTasks(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.ConversationCapture.WorkerCount = 1
	cfg.Gateway.ConversationCapture.QueueSize = 2
	cfg.Gateway.ConversationCapture.TaskTimeoutSeconds = 1
	pool := NewConversationCaptureWorkerPool(cfg)

	block := make(chan struct{})
	started := make(chan struct{})
	secondDone := make(chan struct{})
	stopped := make(chan struct{})

	require.True(t, pool.TrySubmit(func(context.Context) error {
		close(started)
		<-block
		return nil
	}))
	<-started
	require.True(t, pool.TrySubmit(func(context.Context) error {
		close(secondDone)
		return nil
	}))

	go func() {
		pool.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("stop returned before submitted tasks drained")
	case <-time.After(50 * time.Millisecond):
	}

	close(block)
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("queued task did not drain during stop")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop did not finish after drain")
	}
}

func TestConversationCaptureWorkerPoolCountsFailedTask(t *testing.T) {
	pool := NewConversationCaptureWorkerPool(&config.Config{})
	defer pool.Stop()

	require.True(t, pool.TrySubmit(func(context.Context) error {
		return errors.New("synthetic failure")
	}))

	require.Eventually(t, func() bool {
		stats := pool.Stats()
		return stats.Failed == 1 && stats.Completed == 0
	}, time.Second, 10*time.Millisecond)
}

func TestConversationCaptureWorkerPoolRecoversPanicAndContinues(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.ConversationCapture.WorkerCount = 1
	cfg.Gateway.ConversationCapture.QueueSize = 2
	cfg.Gateway.ConversationCapture.TaskTimeoutSeconds = 1
	pool := NewConversationCaptureWorkerPool(cfg)
	defer pool.Stop()

	done := make(chan struct{})
	require.True(t, pool.TrySubmit(func(context.Context) error {
		panic("payload must not reach logs")
	}))
	require.True(t, pool.TrySubmit(func(context.Context) error {
		close(done)
		return nil
	}))

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not continue after panic")
	}
	require.Eventually(t, func() bool {
		stats := pool.Stats()
		return stats.Failed == 1 && stats.Completed == 1
	}, time.Second, 10*time.Millisecond)
}

func TestConversationCaptureWorkerPoolFailureLogIsRateLimited(t *testing.T) {
	pool := &ConversationCaptureWorkerPool{}

	require.True(t, pool.shouldLogFailure())
	require.False(t, pool.shouldLogFailure())

	pool.lastLogNanos.Store(time.Now().Add(-conversationCaptureWorkerFailureLogInterval - time.Millisecond).UnixNano())
	require.True(t, pool.shouldLogFailure())
}
