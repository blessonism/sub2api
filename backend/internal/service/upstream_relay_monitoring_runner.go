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
	upstreamRelayMonitoringFinalizeLeaderLockKey       = "upstream:relay:monitoring:finalize:leader"
)

type upstreamRelayMonitoringRunnerService interface {
	GetMonitoringPolicy(ctx context.Context) (*UpstreamRelayMonitoringPolicy, error)
	SyncAllConnectors(ctx context.Context) (*UpstreamRelayBulkOperationResult, error)
	ProbeAllCandidates(ctx context.Context) (*UpstreamRelayBulkOperationResult, error)
	GenerateAndMaybeApplyRecommendations(ctx context.Context) (*UpstreamRelayRecommendationAutomationResult, error)
	FinalizeYesterdayUsage(ctx context.Context) (*UpstreamRelayBulkOperationResult, error)
}

type upstreamRelayMonitoringJobState struct {
	enabled           bool
	nextRunAt         time.Time
	inFlight          bool
	lastFinishedAt    time.Time
	hasLastResult     bool
	lastSucceeded     bool
	lastError         string
	scheduledInterval time.Duration
	lastRunKey        string
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
	finalize       upstreamRelayMonitoringJobState

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
	r.maybeRunFinalize(parentCtx, now, policy)
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

func (r *UpstreamRelayMonitoringRunner) maybeRunFinalize(parentCtx context.Context, now time.Time, policy *UpstreamRelayMonitoringPolicy) {
	targetDate := upstreamRelayPreviousUsageDate(now)
	if !r.shouldRunDaily(now, &r.finalize, policy.AutoSyncEnabled, targetDate, policy.FailureRetryIntervalMinutes) {
		return
	}
	r.runDailyJob(parentCtx, "finalize", upstreamRelayMonitoringFinalizeLeaderLockKey, targetDate, policy.FailureRetryIntervalMinutes, func(ctx context.Context) (*UpstreamRelayBulkOperationResult, error) {
		return r.svc.FinalizeYesterdayUsage(ctx)
	}, &r.finalize)
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
			state.lastError = ""
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

func (r *UpstreamRelayMonitoringRunner) shouldRunDaily(now time.Time, state *upstreamRelayMonitoringJobState, enabled bool, runKey string, failureRetryMinutes int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !enabled {
		state.enabled = false
		if !state.inFlight {
			state.nextRunAt = time.Time{}
			state.lastFinishedAt = time.Time{}
			state.hasLastResult = false
			state.lastSucceeded = false
			state.lastError = ""
			state.scheduledInterval = 0
		}
		return false
	}
	if !state.enabled {
		state.enabled = true
		state.nextRunAt = now
		state.scheduledInterval = 0
	}
	if state.inFlight || state.lastRunKey == runKey || now.Before(state.nextRunAt) {
		return false
	}
	state.scheduledInterval = upstreamRelayMonitoringScheduleInterval(false, 0, failureRetryMinutes)
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

		lockCtx, lockCancel := context.WithTimeout(parentCtx, 2*time.Second)
		release, leader := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, lockKey, r.instanceID, upstreamRelayMonitoringLeaderLockTTL)
		lockCancel()
		if !leader {
			slog.Debug("upstream_relay_monitoring_runner: skip non-leader run", "job", name)
			r.skipJobWithRetry(state, time.Now(), failureRetryMinutes)
			return
		}
		defer release()

		runCtx, cancel := context.WithTimeout(parentCtx, upstreamRelayMonitoringRunTimeout)
		result, err := run(runCtx)
		cancel()
		if err != nil {
			slog.Warn("upstream_relay_monitoring_runner: run failed", "job", name, "error", err)
			r.finishJob(state, false, err.Error(), successIntervalMinutes, failureRetryMinutes, time.Now())
			return
		}
		r.finishJob(state, true, "", successIntervalMinutes, failureRetryMinutes, time.Now())
		if result != nil {
			slog.Info("upstream_relay_monitoring_runner: run completed", "job", name, "total", result.Total, "success", result.Success, "failed", result.Failed)
		} else {
			slog.Info("upstream_relay_monitoring_runner: run completed", "job", name)
		}
	}()
}

func (r *UpstreamRelayMonitoringRunner) runDailyJob(parentCtx context.Context, name string, lockKey string, runKey string, failureRetryMinutes int, run func(context.Context) (*UpstreamRelayBulkOperationResult, error), state *upstreamRelayMonitoringJobState) {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		lockCtx, lockCancel := context.WithTimeout(parentCtx, 2*time.Second)
		release, leader := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, lockKey, r.instanceID, upstreamRelayMonitoringLeaderLockTTL)
		lockCancel()
		if !leader {
			slog.Debug("upstream_relay_monitoring_runner: skip non-leader run", "job", name)
			r.skipJobWithRetry(state, time.Now(), failureRetryMinutes)
			return
		}
		defer release()

		runCtx, cancel := context.WithTimeout(parentCtx, upstreamRelayMonitoringRunTimeout)
		result, err := run(runCtx)
		cancel()
		if err != nil {
			slog.Warn("upstream_relay_monitoring_runner: run failed", "job", name, "date", runKey, "error", err)
			r.finishDailyJob(state, false, err.Error(), runKey, failureRetryMinutes, time.Now())
			return
		}
		if result != nil && result.Failed > 0 {
			errText := "finalize usage completed with partial failures"
			slog.Warn("upstream_relay_monitoring_runner: run partially failed", "job", name, "date", runKey, "total", result.Total, "success", result.Success, "failed", result.Failed)
			r.finishDailyJob(state, false, errText, runKey, failureRetryMinutes, time.Now())
			return
		}
		r.finishDailyJob(state, true, "", runKey, failureRetryMinutes, time.Now())
		if result != nil {
			slog.Info("upstream_relay_monitoring_runner: run completed", "job", name, "date", runKey, "total", result.Total, "success", result.Success, "failed", result.Failed)
		} else {
			slog.Info("upstream_relay_monitoring_runner: run completed", "job", name, "date", runKey)
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

		lockCtx, lockCancel := context.WithTimeout(parentCtx, 2*time.Second)
		release, leader := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, upstreamRelayMonitoringRecommendationLeaderLockKey, r.instanceID, upstreamRelayMonitoringLeaderLockTTL)
		lockCancel()
		if !leader {
			slog.Debug("upstream_relay_monitoring_runner: skip non-leader run", "job", "recommendation")
			r.skipJobWithRetry(&r.recommendation, time.Now(), failureRetryMinutes)
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
			r.finishJob(&r.recommendation, false, err.Error(), successIntervalMinutes, failureRetryMinutes, time.Now())
			return
		}
		r.finishJob(&r.recommendation, true, "", successIntervalMinutes, failureRetryMinutes, time.Now())
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

func (r *UpstreamRelayMonitoringRunner) skipJob(state *upstreamRelayMonitoringJobState, now time.Time) {
	r.skipJobWithRetry(state, now, 0)
}

func (r *UpstreamRelayMonitoringRunner) skipJobWithRetry(state *upstreamRelayMonitoringJobState, now time.Time, failureRetryMinutes int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state.inFlight = false
	if state.enabled && !now.IsZero() {
		interval := upstreamRelayMonitoringScheduleInterval(false, 0, failureRetryMinutes)
		state.scheduledInterval = interval
		state.nextRunAt = now.Add(interval)
	}
}

func (r *UpstreamRelayMonitoringRunner) finishJob(state *upstreamRelayMonitoringJobState, ok bool, errText string, successIntervalMinutes int, failureRetryMinutes int, now time.Time) {
	interval := upstreamRelayMonitoringScheduleInterval(ok, successIntervalMinutes, failureRetryMinutes)
	nextRunAt := now.Add(interval)

	r.mu.Lock()
	defer r.mu.Unlock()
	state.inFlight = false
	state.lastFinishedAt = now
	state.hasLastResult = true
	state.lastSucceeded = ok
	state.lastError = errText
	state.scheduledInterval = interval
	state.nextRunAt = nextRunAt
}

func (r *UpstreamRelayMonitoringRunner) finishDailyJob(state *upstreamRelayMonitoringJobState, ok bool, errText string, runKey string, failureRetryMinutes int, now time.Time) {
	interval := upstreamRelayMonitoringScheduleInterval(false, 0, failureRetryMinutes)
	nextRunAt := now.Add(interval)

	r.mu.Lock()
	defer r.mu.Unlock()
	state.inFlight = false
	state.lastFinishedAt = now
	state.hasLastResult = true
	state.lastSucceeded = ok
	state.lastError = errText
	state.scheduledInterval = interval
	if ok {
		state.lastRunKey = runKey
		state.nextRunAt = time.Time{}
	} else {
		state.nextRunAt = nextRunAt
	}
}

func upstreamRelayMonitoringScheduleInterval(ok bool, successIntervalMinutes int, failureRetryMinutes int) time.Duration {
	intervalMinutes := failureRetryMinutes
	if ok {
		intervalMinutes = successIntervalMinutes
	}
	return time.Duration(positiveOrDefault(intervalMinutes, 1)) * time.Minute
}

// UpstreamRelayMonitoringJobStatus 描述单个自动任务的运行状态，供前端展示。
type UpstreamRelayMonitoringJobStatus struct {
	Name                string     `json:"name"`
	Enabled             bool       `json:"enabled"`
	InFlight            bool       `json:"in_flight"`
	LastFinishedAt      *time.Time `json:"last_finished_at,omitempty"`
	LastSucceeded       *bool      `json:"last_succeeded,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	NextRunAt           *time.Time `json:"next_run_at,omitempty"`
	IntervalMinutes     int        `json:"interval_minutes"`
	FailureRetryMinutes int        `json:"failure_retry_minutes"`
}

// UpstreamRelayMonitoringRunnerStatus 汇总 Runner 中所有作业的状态。
type UpstreamRelayMonitoringRunnerStatus struct {
	ObservedAt     time.Time                        `json:"observed_at"`
	Sync           UpstreamRelayMonitoringJobStatus `json:"sync"`
	Probe          UpstreamRelayMonitoringJobStatus `json:"probe"`
	Recommendation UpstreamRelayMonitoringJobStatus `json:"recommendation"`
	Finalize       UpstreamRelayMonitoringJobStatus `json:"finalize"`
}

// Status 返回当前 Runner 的作业状态快照。
func (r *UpstreamRelayMonitoringRunner) Status(policy *UpstreamRelayMonitoringPolicy) UpstreamRelayMonitoringRunnerStatus {
	now := time.Now()
	status := UpstreamRelayMonitoringRunnerStatus{ObservedAt: now}
	if r == nil {
		return status
	}
	r.mu.Lock()
	sync := r.sync
	probe := r.probe
	recommendation := r.recommendation
	finalize := r.finalize
	r.mu.Unlock()

	var (
		syncInterval, probeInterval, recommendationInterval int
		syncEnabled, probeEnabled, recommendationEnabled    bool
		failureRetry                                        int
	)
	if policy != nil {
		syncInterval = policy.SyncIntervalMinutes
		probeInterval = policy.ProbeIntervalMinutes
		recommendationInterval = policy.RecommendationIntervalMinutes
		syncEnabled = policy.AutoSyncEnabled
		probeEnabled = policy.AutoProbeEnabled
		recommendationEnabled = policy.AutoRecommendationEnabled
		failureRetry = policy.FailureRetryIntervalMinutes
	}
	status.Sync = buildUpstreamRelayMonitoringJobStatus("sync", sync, syncEnabled, syncInterval, failureRetry)
	status.Probe = buildUpstreamRelayMonitoringJobStatus("probe", probe, probeEnabled, probeInterval, failureRetry)
	status.Recommendation = buildUpstreamRelayMonitoringJobStatus("recommendation", recommendation, recommendationEnabled, recommendationInterval, failureRetry)
	status.Finalize = buildUpstreamRelayMonitoringJobStatus("finalize", finalize, syncEnabled, 0, failureRetry)
	return status
}

func buildUpstreamRelayMonitoringJobStatus(name string, state upstreamRelayMonitoringJobState, enabled bool, intervalMinutes int, failureRetryMinutes int) UpstreamRelayMonitoringJobStatus {
	status := UpstreamRelayMonitoringJobStatus{
		Name:                name,
		Enabled:             enabled,
		InFlight:            state.inFlight,
		IntervalMinutes:     intervalMinutes,
		FailureRetryMinutes: failureRetryMinutes,
	}
	if !state.lastFinishedAt.IsZero() {
		t := state.lastFinishedAt
		status.LastFinishedAt = &t
	}
	if state.hasLastResult {
		ok := state.lastSucceeded
		status.LastSucceeded = &ok
	}
	status.LastError = state.lastError
	if enabled && !state.nextRunAt.IsZero() {
		t := state.nextRunAt
		status.NextRunAt = &t
	}
	return status
}
