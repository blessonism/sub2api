package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	upstreamRelayMonitoringRunnerTickInterval = 30 * time.Second
	upstreamRelayMonitoringRunTimeout         = 20 * time.Minute
	upstreamRelayMonitoringLeaderLockTTL      = 25 * time.Minute

	upstreamRelayMonitoringSyncLeaderLockKey           = "upstream:relay:monitoring:sync:leader"
	upstreamRelayMonitoringProbeLeaderLockKey          = "upstream:relay:monitoring:probe:leader"
	upstreamRelayMonitoringRecommendationLeaderLockKey = "upstream:relay:monitoring:recommendation:leader"
)

type upstreamRelayMonitoringRunnerService interface {
	GetMonitoringPolicy(ctx context.Context) (*UpstreamRelayMonitoringPolicy, error)
	SyncAllConnectors(ctx context.Context) (*UpstreamRelayBulkOperationResult, error)
	ProbeAllCandidates(ctx context.Context) (*UpstreamRelayBulkOperationResult, error)
	GenerateAndMaybeApplyRecommendations(ctx context.Context) (*UpstreamRelayRecommendationAutomationResult, error)
}

type upstreamRelayMonitoringJobState struct {
	enabled           bool
	nextRunAt         time.Time
	inFlight          bool
	lastFinishedAt    time.Time
	hasLastResult     bool
	lastSucceeded     bool
	scheduledInterval time.Duration
}

// UpstreamRelayMonitoringRunner 按全局监控策略自动执行上游倍率同步、候选探测和推荐建议。
type UpstreamRelayMonitoringRunner struct {
	svc      upstreamRelayMonitoringRunnerService
	interval time.Duration

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	mu             sync.Mutex
	sync           upstreamRelayMonitoringJobState
	probe          upstreamRelayMonitoringJobState
	recommendation upstreamRelayMonitoringJobState

	parentCtx    context.Context
	parentCancel context.CancelFunc
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewUpstreamRelayMonitoringRunner(svc upstreamRelayMonitoringRunnerService) *UpstreamRelayMonitoringRunner {
	return newUpstreamRelayMonitoringRunner(svc, upstreamRelayMonitoringRunnerTickInterval)
}

func newUpstreamRelayMonitoringRunner(svc upstreamRelayMonitoringRunnerService, interval time.Duration) *UpstreamRelayMonitoringRunner {
	return &UpstreamRelayMonitoringRunner{
		svc:        svc,
		interval:   interval,
		instanceID: uuid.NewString(),
	}
}

func (r *UpstreamRelayMonitoringRunner) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if r == nil {
		return
	}
	r.lockCache = lockCache
	r.db = db
}

func (r *UpstreamRelayMonitoringRunner) Start() {
	if r == nil || r.svc == nil || r.interval <= 0 {
		return
	}
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	parentCtx, parentCancel := context.WithCancel(context.Background())
	stopCh := make(chan struct{})
	r.parentCtx = parentCtx
	r.parentCancel = parentCancel
	r.stopCh = stopCh
	r.running = true
	interval := r.interval
	r.wg.Add(1)
	r.mu.Unlock()

	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.runCycle(time.Now())
			case <-parentCtx.Done():
				return
			case <-stopCh:
				return
			}
		}
	}()
}

func (r *UpstreamRelayMonitoringRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		r.wg.Wait()
		return
	}
	cancel := r.parentCancel
	stopCh := r.stopCh
	r.running = false
	r.parentCancel = nil
	r.parentCtx = nil
	r.stopCh = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if stopCh != nil {
		close(stopCh)
	}
	r.wg.Wait()
}

func (r *UpstreamRelayMonitoringRunner) runCycle(now time.Time) {
	if r == nil || r.svc == nil {
		return
	}
	parentCtx := r.getParentContext()
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	policy, err := r.svc.GetMonitoringPolicy(ctx)
	cancel()
	if err != nil {
		slog.Warn("upstream_relay_monitoring_runner: load policy failed", "error", err)
		return
	}
	if policy == nil {
		return
	}

	r.maybeRunSync(parentCtx, now, policy)
	r.maybeRunProbe(parentCtx, now, policy)
	r.maybeRunRecommendation(parentCtx, now, policy)
}

func (r *UpstreamRelayMonitoringRunner) getParentContext() context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.parentCtx != nil {
		return r.parentCtx
	}
	return context.Background()
}

func (r *UpstreamRelayMonitoringRunner) maybeRunSync(parentCtx context.Context, now time.Time, policy *UpstreamRelayMonitoringPolicy) {
	if !r.shouldRun(now, &r.sync, policy.AutoSyncEnabled, policy.SyncIntervalMinutes, policy.FailureRetryIntervalMinutes) {
		return
	}
	r.runJob(parentCtx, "sync", upstreamRelayMonitoringSyncLeaderLockKey, policy.SyncIntervalMinutes, policy.FailureRetryIntervalMinutes, func(ctx context.Context) (*UpstreamRelayBulkOperationResult, error) {
		return r.svc.SyncAllConnectors(ctx)
	}, &r.sync)
}

func (r *UpstreamRelayMonitoringRunner) maybeRunProbe(parentCtx context.Context, now time.Time, policy *UpstreamRelayMonitoringPolicy) {
	if !r.shouldRun(now, &r.probe, policy.AutoProbeEnabled, policy.ProbeIntervalMinutes, policy.FailureRetryIntervalMinutes) {
		return
	}
	r.runJob(parentCtx, "probe", upstreamRelayMonitoringProbeLeaderLockKey, policy.ProbeIntervalMinutes, policy.FailureRetryIntervalMinutes, func(ctx context.Context) (*UpstreamRelayBulkOperationResult, error) {
		return r.svc.ProbeAllCandidates(ctx)
	}, &r.probe)
}

func (r *UpstreamRelayMonitoringRunner) maybeRunRecommendation(parentCtx context.Context, now time.Time, policy *UpstreamRelayMonitoringPolicy) {
	if !r.shouldRun(now, &r.recommendation, policy.AutoRecommendationEnabled, policy.RecommendationIntervalMinutes, policy.FailureRetryIntervalMinutes) {
		return
	}
	r.runRecommendationJob(parentCtx, policy.RecommendationIntervalMinutes, policy.FailureRetryIntervalMinutes)
}

func (r *UpstreamRelayMonitoringRunner) shouldRun(now time.Time, state *upstreamRelayMonitoringJobState, enabled bool, successIntervalMinutes int, failureRetryMinutes int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !enabled {
		state.enabled = false
		if !state.inFlight {
			state.nextRunAt = time.Time{}
			state.lastFinishedAt = time.Time{}
			state.hasLastResult = false
			state.lastSucceeded = false
			state.scheduledInterval = 0
		}
		if state.inFlight {
			return false
		}
		return false
	}
	if !state.enabled {
		state.enabled = true
		state.nextRunAt = now
		state.scheduledInterval = 0
	}
	if state.inFlight {
		return false
	}
	if state.hasLastResult {
		desiredInterval := upstreamRelayMonitoringScheduleInterval(state.lastSucceeded, successIntervalMinutes, failureRetryMinutes)
		if state.scheduledInterval != desiredInterval {
			state.scheduledInterval = desiredInterval
			state.nextRunAt = state.lastFinishedAt.Add(desiredInterval)
		}
	}
	if now.Before(state.nextRunAt) {
		return false
	}
	state.inFlight = true
	return true
}

func (r *UpstreamRelayMonitoringRunner) runJob(parentCtx context.Context, name string, lockKey string, successIntervalMinutes int, failureRetryMinutes int, run func(context.Context) (*UpstreamRelayBulkOperationResult, error), state *upstreamRelayMonitoringJobState) {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		ok := false
		defer func() {
			r.finishJob(state, ok, successIntervalMinutes, failureRetryMinutes, time.Now())
		}()

		lockCtx, lockCancel := context.WithTimeout(parentCtx, 2*time.Second)
		release, leader := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, lockKey, r.instanceID, upstreamRelayMonitoringLeaderLockTTL)
		lockCancel()
		if !leader {
			slog.Debug("upstream_relay_monitoring_runner: skip non-leader run", "job", name)
			return
		}
		defer release()

		runCtx, cancel := context.WithTimeout(parentCtx, upstreamRelayMonitoringRunTimeout)
		result, err := run(runCtx)
		cancel()
		if err != nil {
			slog.Warn("upstream_relay_monitoring_runner: run failed", "job", name, "error", err)
			return
		}
		ok = true
		if result != nil {
			slog.Info("upstream_relay_monitoring_runner: run completed", "job", name, "total", result.Total, "success", result.Success, "failed", result.Failed)
		} else {
			slog.Info("upstream_relay_monitoring_runner: run completed", "job", name)
		}
	}()
}

func (r *UpstreamRelayMonitoringRunner) runRecommendationJob(parentCtx context.Context, successIntervalMinutes int, failureRetryMinutes int) {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		ok := false
		defer func() {
			r.finishJob(&r.recommendation, ok, successIntervalMinutes, failureRetryMinutes, time.Now())
		}()

		lockCtx, lockCancel := context.WithTimeout(parentCtx, 2*time.Second)
		release, leader := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, upstreamRelayMonitoringRecommendationLeaderLockKey, r.instanceID, upstreamRelayMonitoringLeaderLockTTL)
		lockCancel()
		if !leader {
			slog.Debug("upstream_relay_monitoring_runner: skip non-leader run", "job", "recommendation")
			return
		}
		defer release()

		runCtx, cancel := context.WithTimeout(parentCtx, upstreamRelayMonitoringRunTimeout)
		result, err := r.svc.GenerateAndMaybeApplyRecommendations(runCtx)
		cancel()
		if err != nil {
			if result != nil && result.Run != nil {
				slog.Warn(
					"upstream_relay_monitoring_runner: run failed",
					"job", "recommendation",
					"run_id", result.Run.ID,
					"suggestions", result.Run.SuggestionCount,
					"error", err,
				)
			} else {
				slog.Warn("upstream_relay_monitoring_runner: run failed", "job", "recommendation", "error", err)
			}
			return
		}
		ok = true
		if result != nil && result.Run != nil {
			slog.Info(
				"upstream_relay_monitoring_runner: run completed",
				"job", "recommendation",
				"run_id", result.Run.ID,
				"suggestions", result.Run.SuggestionCount,
				"applied", result.Applied,
				"skipped_reason", result.SkippedReason,
			)
		} else {
			slog.Info("upstream_relay_monitoring_runner: run completed", "job", "recommendation")
		}
	}()
}

func (r *UpstreamRelayMonitoringRunner) finishJob(state *upstreamRelayMonitoringJobState, ok bool, successIntervalMinutes int, failureRetryMinutes int, now time.Time) {
	interval := upstreamRelayMonitoringScheduleInterval(ok, successIntervalMinutes, failureRetryMinutes)
	nextRunAt := now.Add(interval)

	r.mu.Lock()
	defer r.mu.Unlock()
	state.inFlight = false
	state.lastFinishedAt = now
	state.hasLastResult = true
	state.lastSucceeded = ok
	state.scheduledInterval = interval
	state.nextRunAt = nextRunAt
}

func upstreamRelayMonitoringScheduleInterval(ok bool, successIntervalMinutes int, failureRetryMinutes int) time.Duration {
	intervalMinutes := failureRetryMinutes
	if ok {
		intervalMinutes = successIntervalMinutes
	}
	return time.Duration(positiveOrDefault(intervalMinutes, 1)) * time.Minute
}
