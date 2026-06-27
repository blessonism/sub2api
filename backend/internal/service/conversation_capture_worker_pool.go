package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const conversationCaptureWorkerFailureLogInterval = 5 * time.Second

type ConversationCaptureTask func(context.Context) error

type ConversationCaptureWorkerStats struct {
	Submitted int64 `json:"submitted"`
	Dropped   int64 `json:"dropped"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

type ConversationCaptureWorkerPool struct {
	queue        chan ConversationCaptureTask
	mu           sync.RWMutex
	wg           sync.WaitGroup
	stopped      atomic.Bool
	submitted    atomic.Int64
	dropped      atomic.Int64
	completed    atomic.Int64
	failed       atomic.Int64
	lastLogNanos atomic.Int64
	taskTimeout  time.Duration
	component    string
}

type ConversationExportWorkerPool struct {
	*ConversationCaptureWorkerPool
}

func NewConversationCaptureWorkerPool(cfg *config.Config) *ConversationCaptureWorkerPool {
	workerCount := 2
	queueSize := 256
	timeout := 10 * time.Second
	if cfg != nil {
		cc := cfg.Gateway.ConversationCapture
		if cc.WorkerCount > 0 {
			workerCount = cc.WorkerCount
		}
		if cc.QueueSize > 0 {
			queueSize = cc.QueueSize
		}
		if cc.TaskTimeoutSeconds > 0 {
			timeout = time.Duration(cc.TaskTimeoutSeconds) * time.Second
		}
	}
	return newConversationTaskWorkerPool(workerCount, queueSize, timeout, "service.conversation_capture")
}

func NewConversationExportWorkerPool(_ *config.Config) *ConversationExportWorkerPool {
	return &ConversationExportWorkerPool{
		ConversationCaptureWorkerPool: newConversationTaskWorkerPool(1, 32, 30*time.Minute, "service.conversation_export"),
	}
}

func newConversationTaskWorkerPool(workerCount, queueSize int, timeout time.Duration, component string) *ConversationCaptureWorkerPool {
	if workerCount <= 0 {
		workerCount = 1
	}
	if queueSize <= 0 {
		queueSize = 1
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if component == "" {
		component = "service.conversation_capture"
	}
	p := &ConversationCaptureWorkerPool{
		queue:       make(chan ConversationCaptureTask, queueSize),
		taskTimeout: timeout,
		component:   component,
	}
	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *ConversationCaptureWorkerPool) TrySubmit(task ConversationCaptureTask) bool {
	if p == nil || task == nil {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stopped.Load() {
		p.dropped.Add(1)
		return false
	}
	select {
	case p.queue <- task:
		p.submitted.Add(1)
		return true
	default:
		p.dropped.Add(1)
		return false
	}
}

func (p *ConversationCaptureWorkerPool) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.stopped.Load() {
		p.mu.Unlock()
		return
	}
	p.stopped.Store(true)
	close(p.queue)
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *ConversationCaptureWorkerPool) Stats() ConversationCaptureWorkerStats {
	if p == nil {
		return ConversationCaptureWorkerStats{}
	}
	return ConversationCaptureWorkerStats{
		Submitted: p.submitted.Load(),
		Dropped:   p.dropped.Load(),
		Completed: p.completed.Load(),
		Failed:    p.failed.Load(),
	}
}

func (p *ConversationCaptureWorkerPool) worker() {
	defer p.wg.Done()
	for task := range p.queue {
		p.runTask(task)
	}
}

func (p *ConversationCaptureWorkerPool) runTask(task ConversationCaptureTask) {
	ctx, cancel := context.WithTimeout(context.Background(), p.taskTimeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			p.failed.Add(1)
			if p.shouldLogFailure() {
				logger.L().With(
					zap.String("component", p.component),
					zap.String("panic_type", fmt.Sprintf("%T", recovered)),
				).Error("conversation_capture.worker_panic_recovered")
			}
		}
	}()
	if err := task(ctx); err != nil {
		p.failed.Add(1)
		if p.shouldLogFailure() {
			logger.L().With(
				zap.String("component", p.component),
				zap.String("error_type", fmt.Sprintf("%T", err)),
			).Warn("conversation_capture.worker_task_failed")
		}
		return
	}
	p.completed.Add(1)
}

func (p *ConversationCaptureWorkerPool) shouldLogFailure() bool {
	now := time.Now().UnixNano()
	last := p.lastLogNanos.Load()
	if now-last < int64(conversationCaptureWorkerFailureLogInterval) {
		return false
	}
	return p.lastLogNanos.CompareAndSwap(last, now)
}
