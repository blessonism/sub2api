package service

import (
	"context"
	"database/sql"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// modelIQProbeRunTimeout 单轮探测的执行超时（探测目标数大、单次请求慢时兜底）。
	modelIQProbeRunTimeout = 40 * time.Minute
	// modelIQProbeLeaderLockTTL leader lock 生存期，须覆盖单轮最长执行时间。
	modelIQProbeLeaderLockTTL = 45 * time.Minute
	// modelIQProbeLeaderLockKey 多实例互斥锁 key。
	modelIQProbeLeaderLockKey = "model_iq_probe:runner:leader"
	// modelIQProbeIdleDelay 探针未启用时的轮询间隔（等待配置启用或 Reload 唤醒）。
	modelIQProbeIdleDelay = time.Minute
)

// ModelIQProbeRunner 智商探针定时调度器：按配置间隔触发探测轮次。
// 与 channel_monitor_runner 不同，全局只有一个任务（非每 monitor 一个），
// 多实例互斥靠 leader lock；配置保存后通过 Reload 立即重排下一次执行。
type ModelIQProbeRunner struct {
	svc *ModelIQProbeService

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	parentCtx    context.Context
	parentCancel context.CancelFunc
	reloadCh     chan struct{}
	stopCh       chan struct{}
	wg           sync.WaitGroup

	mu      sync.Mutex
	started bool
	stopped bool
}

// NewModelIQProbeRunner 构造调度器；Start 在 wire Provider 中调用一次。
func NewModelIQProbeRunner(svc *ModelIQProbeService) *ModelIQProbeRunner {
	return &ModelIQProbeRunner{
		svc:        svc,
		instanceID: uuid.NewString(),
		reloadCh:   make(chan struct{}, 1),
	}
}

// SetLeaderLock 注入多实例互斥依赖（测试可不注入：nil 时视为单实例）。
func (r *ModelIQProbeRunner) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if r == nil {
		return
	}
	r.lockCache = lockCache
	r.db = db
}

// Start 启动定时循环。调用方需保证只调一次。
func (r *ModelIQProbeRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	if r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.started = true
	ctx, cancel := context.WithCancel(context.Background())
	stopCh := make(chan struct{})
	r.parentCtx = ctx
	r.parentCancel = cancel
	r.stopCh = stopCh
	r.wg.Add(1)
	r.mu.Unlock()

	go func() {
		defer r.wg.Done()
		r.loop(ctx, stopCh)
	}()
	slog.Info("model_iq_probe: runner started")
}

// Stop 停止调度循环并等待退出。
func (r *ModelIQProbeRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if !r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	r.mu.Unlock()

	if r.parentCancel != nil {
		r.parentCancel()
	}
	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
	r.wg.Wait()
}

// Reload 通知循环按新配置重排下一次执行（配置保存后调用）。
func (r *ModelIQProbeRunner) Reload() {
	if r == nil {
		return
	}
	select {
	case r.reloadCh <- struct{}{}:
	default:
	}
}

// loop 定时主循环：读配置 → 计算 delay → 触发一轮探测。
func (r *ModelIQProbeRunner) loop(ctx context.Context, stopCh chan struct{}) {
	timer := time.NewTimer(r.nextDelay())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-r.reloadCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(r.nextDelay())
			continue
		case <-timer.C:
		}

		r.runScheduledCycle(ctx)
		timer.Reset(r.nextDelay())
	}
}

// nextDelay 计算下一次触发等待：启用时 interval ±10% jitter，未启用时用空闲轮询间隔。
func (r *ModelIQProbeRunner) nextDelay() time.Duration {
	cfg, err := r.svc.GetConfig(context.Background())
	if err != nil {
		slog.Warn("model_iq_probe: read config for schedule failed", "error", err)
		return modelIQProbeIdleDelay
	}
	if !cfg.Enabled {
		return modelIQProbeIdleDelay
	}
	base := cfg.Interval()
	// ±10% 的均匀抖动，避免多实例同一时刻唤醒（leader lock 之外的第二道缓解）。
	jitter := time.Duration(float64(base) * (rand.Float64()*0.2 - 0.1))
	delay := base + jitter
	if delay < time.Minute {
		delay = time.Minute
	}
	return delay
}

// runScheduledCycle 定时触发的探测轮次：仅 leader 实例执行。
func (r *ModelIQProbeRunner) runScheduledCycle(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		lockCtx, lockCancel := context.WithTimeout(ctx, 2*time.Second)
		release, leader := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, modelIQProbeLeaderLockKey, r.instanceID, modelIQProbeLeaderLockTTL)
		lockCancel()
		if !leader {
			slog.Debug("model_iq_probe: skip non-leader scheduled run")
			return
		}
		defer release()

		runCtx, runCancel := context.WithTimeout(ctx, modelIQProbeRunTimeout)
		defer runCancel()
		summary, err := r.svc.RunProbeCycle(runCtx)
		if err != nil {
			slog.Warn("model_iq_probe: scheduled cycle failed", "error", err)
			return
		}
		if summary != nil && summary.Total > 0 {
			slog.Info("model_iq_probe: scheduled cycle completed", "total", summary.Total, "pass", summary.Pass, "wrong_answer", summary.WrongAnswer, "error", summary.Error)
		}
	}()
}

// RunOnce 手动触发一轮探测（管理端"立即探测"）。
// leader lock 已被占用（其他实例正在跑）时返回进行中错误；
// 进程内重入由 service 的 atomic 防护兜底。
func (r *ModelIQProbeRunner) RunOnce(ctx context.Context) (*ModelIQProbeCycleSummary, error) {
	if r == nil || r.svc == nil {
		return nil, ErrModelIQProbeUnavailable
	}
	lockCtx, lockCancel := context.WithTimeout(ctx, 2*time.Second)
	release, leader := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, modelIQProbeLeaderLockKey, r.instanceID, modelIQProbeLeaderLockTTL)
	lockCancel()
	if !leader {
		return nil, ErrModelIQProbeRunInProgress
	}
	defer release()

	runCtx, runCancel := context.WithTimeout(ctx, modelIQProbeRunTimeout)
	defer runCancel()
	return r.svc.RunProbeCycle(runCtx)
}
