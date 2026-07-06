package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type upstreamRelayMonitoringRunnerStub struct {
	mu     sync.Mutex
	policy *UpstreamRelayMonitoringPolicy

	syncCalls                 int
	probeCalls                int
	recommendationCalls       int
	finalizeCalls             int
	syncErr                   error
	probeErr                  error
	recommendationErr         error
	finalizeErr               error
	finalizeResult            *UpstreamRelayBulkOperationResult
	recommendationResultOnErr *UpstreamRelayRecommendationAutomationResult

	policyStarted chan struct{}
	syncStarted   chan struct{}
	syncRelease   chan struct{}
}

func (s *upstreamRelayMonitoringRunnerStub) GetMonitoringPolicy(ctx context.Context) (*UpstreamRelayMonitoringPolicy, error) {
	s.mu.Lock()
	started := s.policyStarted
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	defer s.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if s.policy == nil {
		return nil, nil
	}
	policy := *s.policy
	return &policy, nil
}

func (s *upstreamRelayMonitoringRunnerStub) SyncAllConnectors(ctx context.Context) (*UpstreamRelayBulkOperationResult, error) {
	s.mu.Lock()
	s.syncCalls++
	started := s.syncStarted
	release := s.syncRelease
	err := s.syncErr
	s.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	return &UpstreamRelayBulkOperationResult{Total: 1, Success: 1}, nil
}

func (s *upstreamRelayMonitoringRunnerStub) ProbeAllCandidates(context.Context) (*UpstreamRelayBulkOperationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probeCalls++
	if s.probeErr != nil {
		return nil, s.probeErr
	}
	return &UpstreamRelayBulkOperationResult{Total: 1, Success: 1}, nil
}

func (s *upstreamRelayMonitoringRunnerStub) GenerateAndMaybeApplyRecommendations(context.Context) (*UpstreamRelayRecommendationAutomationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recommendationCalls++
	if s.recommendationErr != nil {
		return s.recommendationResultOnErr, s.recommendationErr
	}
	return &UpstreamRelayRecommendationAutomationResult{
		Run: &UpstreamRelayRecommendationRun{
			ID:              int64(s.recommendationCalls),
			Status:          UpstreamRelayRunStatusSuccess,
			SuggestionCount: 1,
		},
	}, nil
}

func (s *upstreamRelayMonitoringRunnerStub) FinalizeYesterdayUsage(context.Context) (*UpstreamRelayBulkOperationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finalizeCalls++
	if s.finalizeErr != nil {
		return nil, s.finalizeErr
	}
	if s.finalizeResult != nil {
		result := *s.finalizeResult
		return &result, nil
	}
	return &UpstreamRelayBulkOperationResult{Total: 1, Success: 1}, nil
}

func (s *upstreamRelayMonitoringRunnerStub) counts() (int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncCalls, s.probeCalls, s.recommendationCalls
}

func (s *upstreamRelayMonitoringRunnerStub) setPolicy(policy UpstreamRelayMonitoringPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = &policy
}

func waitRunnerIdle(t *testing.T, r *UpstreamRelayMonitoringRunner) {
	t.Helper()
	waitUpstreamRelayMonitoringRunnerFor(t, 2*time.Second, "upstream relay monitoring runner idle", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return !r.sync.inFlight && !r.probe.inFlight && !r.recommendation.inFlight && !r.finalize.inFlight
	})
}

func waitUpstreamRelayMonitoringRunnerFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("wait timed out: %s", msg)
	}
}

func runnerJobNextRunAt(runner *UpstreamRelayMonitoringRunner, job string) time.Time {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if job == "sync" {
		return runner.sync.nextRunAt
	}
	if job == "probe" {
		return runner.probe.nextRunAt
	}
	if job == "finalize" {
		return runner.finalize.nextRunAt
	}
	return runner.recommendation.nextRunAt
}

func runnerJobLastFinishedAt(runner *UpstreamRelayMonitoringRunner, job string) time.Time {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if job == "sync" {
		return runner.sync.lastFinishedAt
	}
	if job == "probe" {
		return runner.probe.lastFinishedAt
	}
	if job == "finalize" {
		return runner.finalize.lastFinishedAt
	}
	return runner.recommendation.lastFinishedAt
}

func runnerJobLastError(runner *UpstreamRelayMonitoringRunner, job string) string {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if job == "sync" {
		return runner.sync.lastError
	}
	if job == "probe" {
		return runner.probe.lastError
	}
	if job == "finalize" {
		return runner.finalize.lastError
	}
	return runner.recommendation.lastError
}

func runnerFinalizeState(runner *UpstreamRelayMonitoringRunner) upstreamRelayMonitoringJobState {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.finalize
}

func TestUpstreamRelayMonitoringRunnerRunsEnabledJobsOnFirstCycle(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{policy: &UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         60,
		AutoProbeEnabled:            true,
		ProbeIntervalMinutes:        15,
		FailureRetryIntervalMinutes: 5,
	}}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)

	now := time.Now()
	runner.runCycle(now)
	waitRunnerIdle(t, runner)

	syncCalls, probeCalls, _ := svc.counts()
	require.Equal(t, 1, syncCalls)
	require.Equal(t, 1, probeCalls)
	require.WithinDuration(t, now.Add(60*time.Minute), runnerJobNextRunAt(runner, "sync"), time.Second)
	require.WithinDuration(t, now.Add(15*time.Minute), runnerJobNextRunAt(runner, "probe"), time.Second)
}

func TestUpstreamRelayMonitoringRunnerStopCancelsRunningJob(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{
		policy: &UpstreamRelayMonitoringPolicy{
			AutoSyncEnabled:             true,
			SyncIntervalMinutes:         60,
			FailureRetryIntervalMinutes: 5,
		},
		syncStarted: make(chan struct{}, 1),
		syncRelease: make(chan struct{}),
	}
	runner := newUpstreamRelayMonitoringRunner(svc, 10*time.Millisecond)
	runner.Start()

	select {
	case <-svc.syncStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected sync job to start")
	}

	stopped := make(chan struct{})
	go func() {
		runner.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("expected Stop to cancel running sync job")
	}
	runner.Stop()
}

func TestUpstreamRelayMonitoringRunnerSkipsDisabledJobs(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{policy: &UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             false,
		SyncIntervalMinutes:         60,
		AutoProbeEnabled:            false,
		ProbeIntervalMinutes:        15,
		FailureRetryIntervalMinutes: 5,
	}}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)

	runner.runCycle(time.Now())

	syncCalls, probeCalls, recommendationCalls := svc.counts()
	require.Zero(t, syncCalls)
	require.Zero(t, probeCalls)
	require.Zero(t, recommendationCalls)
}

func TestUpstreamRelayMonitoringRunnerSkipsWhileInFlight(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{
		policy: &UpstreamRelayMonitoringPolicy{
			AutoSyncEnabled:             true,
			SyncIntervalMinutes:         60,
			FailureRetryIntervalMinutes: 5,
		},
		syncStarted: make(chan struct{}, 1),
		syncRelease: make(chan struct{}),
	}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	runner.runCycle(now)
	select {
	case <-svc.syncStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected sync job to start")
	}

	runner.runCycle(now.Add(time.Second))
	syncCalls, _, _ := svc.counts()
	require.Equal(t, 1, syncCalls)

	close(svc.syncRelease)
	waitRunnerIdle(t, runner)
}

func TestUpstreamRelayMonitoringRunnerDisabledWhileInFlightDoesNotReleaseSlot(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{
		policy: &UpstreamRelayMonitoringPolicy{
			AutoSyncEnabled:             true,
			SyncIntervalMinutes:         60,
			FailureRetryIntervalMinutes: 5,
		},
		syncStarted: make(chan struct{}, 1),
		syncRelease: make(chan struct{}),
	}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	now := time.Now()

	runner.runCycle(now)
	select {
	case <-svc.syncStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected sync job to start")
	}

	svc.setPolicy(UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             false,
		SyncIntervalMinutes:         60,
		FailureRetryIntervalMinutes: 5,
	})
	runner.runCycle(now.Add(time.Second))

	svc.setPolicy(UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         60,
		FailureRetryIntervalMinutes: 5,
	})
	runner.runCycle(now.Add(2 * time.Second))

	syncCalls, _, _ := svc.counts()
	require.Equal(t, 1, syncCalls)

	close(svc.syncRelease)
	waitRunnerIdle(t, runner)
}

func TestUpstreamRelayMonitoringRunnerSchedulesRetryAfterFailure(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{
		policy: &UpstreamRelayMonitoringPolicy{
			AutoSyncEnabled:             true,
			SyncIntervalMinutes:         60,
			FailureRetryIntervalMinutes: 5,
		},
		syncErr: errors.New("sync failed"),
	}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	now := time.Now()

	runner.runCycle(now)
	waitRunnerIdle(t, runner)

	require.WithinDuration(t, now.Add(5*time.Minute), runnerJobNextRunAt(runner, "sync"), time.Second)
	status := runner.Status(svc.policy)
	require.NotNil(t, status.Sync.LastSucceeded)
	require.False(t, *status.Sync.LastSucceeded)
	require.Contains(t, status.Sync.LastError, "sync failed")
}

func TestUpstreamRelayMonitoringRunnerReschedulesAfterSuccessIntervalShortened(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{policy: &UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         60,
		FailureRetryIntervalMinutes: 5,
	}}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	first := time.Now()

	runner.runCycle(first)
	waitRunnerIdle(t, runner)
	firstNextRunAt := runnerJobNextRunAt(runner, "sync")
	firstFinishedAt := runnerJobLastFinishedAt(runner, "sync")

	svc.setPolicy(UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         5,
		FailureRetryIntervalMinutes: 5,
	})
	runner.runCycle(firstFinishedAt.Add(6 * time.Minute))
	waitRunnerIdle(t, runner)

	syncCalls, _, _ := svc.counts()
	require.Equal(t, 2, syncCalls)
	require.WithinDuration(t, firstFinishedAt.Add(60*time.Minute), firstNextRunAt, time.Second)
	require.WithinDuration(t, runnerJobLastFinishedAt(runner, "sync").Add(5*time.Minute), runnerJobNextRunAt(runner, "sync"), time.Second)
}

func TestUpstreamRelayMonitoringRunnerReschedulesProbeAfterSuccessIntervalShortened(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{policy: &UpstreamRelayMonitoringPolicy{
		AutoProbeEnabled:            true,
		ProbeIntervalMinutes:        30,
		FailureRetryIntervalMinutes: 5,
	}}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	first := time.Now()

	runner.runCycle(first)
	waitRunnerIdle(t, runner)
	firstFinishedAt := runnerJobLastFinishedAt(runner, "probe")

	svc.setPolicy(UpstreamRelayMonitoringPolicy{
		AutoProbeEnabled:            true,
		ProbeIntervalMinutes:        5,
		FailureRetryIntervalMinutes: 5,
	})
	runner.runCycle(firstFinishedAt.Add(6 * time.Minute))
	waitRunnerIdle(t, runner)

	_, probeCalls, _ := svc.counts()
	require.Equal(t, 2, probeCalls)
	require.WithinDuration(t, runnerJobLastFinishedAt(runner, "probe").Add(5*time.Minute), runnerJobNextRunAt(runner, "probe"), time.Second)
}

func TestUpstreamRelayMonitoringRunnerReschedulesAfterSuccessIntervalLengthened(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{policy: &UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         5,
		FailureRetryIntervalMinutes: 5,
	}}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	first := time.Now()

	runner.runCycle(first)
	waitRunnerIdle(t, runner)
	firstFinishedAt := runnerJobLastFinishedAt(runner, "sync")

	svc.setPolicy(UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         60,
		FailureRetryIntervalMinutes: 5,
	})
	runner.runCycle(firstFinishedAt.Add(6 * time.Minute))
	waitRunnerIdle(t, runner)

	syncCalls, _, _ := svc.counts()
	require.Equal(t, 1, syncCalls)
	require.WithinDuration(t, firstFinishedAt.Add(60*time.Minute), runnerJobNextRunAt(runner, "sync"), time.Second)
}

func TestUpstreamRelayMonitoringRunnerReschedulesAfterFailureRetryIntervalChanged(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{
		policy: &UpstreamRelayMonitoringPolicy{
			AutoSyncEnabled:             true,
			SyncIntervalMinutes:         60,
			FailureRetryIntervalMinutes: 30,
		},
		syncErr: errors.New("sync failed"),
	}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	first := time.Now()

	runner.runCycle(first)
	waitRunnerIdle(t, runner)
	firstFinishedAt := runnerJobLastFinishedAt(runner, "sync")

	svc.setPolicy(UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         60,
		FailureRetryIntervalMinutes: 5,
	})
	runner.runCycle(firstFinishedAt.Add(6 * time.Minute))
	waitRunnerIdle(t, runner)

	syncCalls, _, _ := svc.counts()
	require.Equal(t, 2, syncCalls)
	require.WithinDuration(t, runnerJobLastFinishedAt(runner, "sync").Add(5*time.Minute), runnerJobNextRunAt(runner, "sync"), time.Second)
}

func TestUpstreamRelayMonitoringRunnerSkipsWhenLeaderLockHeld(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	_, _ = cache.TryAcquireLeaderLock(context.Background(), upstreamRelayMonitoringSyncLeaderLockKey, "peer", time.Minute)
	svc := &upstreamRelayMonitoringRunnerStub{policy: &UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         60,
		FailureRetryIntervalMinutes: 5,
	}}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	runner.SetLeaderLock(cache, nil)

	now := time.Now()
	runner.runCycle(now)
	waitRunnerIdle(t, runner)

	syncCalls, _, _ := svc.counts()
	require.Zero(t, syncCalls)
	require.WithinDuration(t, now.Add(5*time.Minute), runnerJobNextRunAt(runner, "sync"), time.Second)
	status := runner.Status(svc.policy)
	require.Nil(t, status.Sync.LastSucceeded)
	require.Empty(t, status.Sync.LastError)
}

func TestUpstreamRelayMonitoringRunnerDailyFinalizePartialFailureRetries(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{
		policy: &UpstreamRelayMonitoringPolicy{
			AutoSyncEnabled:             true,
			FailureRetryIntervalMinutes: 5,
		},
		finalizeResult: &UpstreamRelayBulkOperationResult{Total: 2, Success: 1, Failed: 1},
	}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	runner.runCycle(now)
	waitRunnerIdle(t, runner)

	state := runnerFinalizeState(runner)
	require.Equal(t, 1, svc.finalizeCalls)
	require.True(t, state.lastRunKey == "", "partial finalize failure must not mark the daily run key complete")
	require.False(t, state.lastSucceeded)
	require.Contains(t, state.lastError, "partial failures")
	require.WithinDuration(t, state.lastFinishedAt.Add(5*time.Minute), runnerJobNextRunAt(runner, "finalize"), time.Second)
}

func TestUpstreamRelayMonitoringRunnerRunsAfterDisabledThenEnabled(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{policy: &UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             false,
		SyncIntervalMinutes:         60,
		FailureRetryIntervalMinutes: 5,
	}}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	first := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	runner.runCycle(first)

	svc.setPolicy(UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         60,
		FailureRetryIntervalMinutes: 5,
	})
	runner.runCycle(first.Add(30 * time.Second))
	waitRunnerIdle(t, runner)

	syncCalls, _, _ := svc.counts()
	require.Equal(t, 1, syncCalls)
}

func TestUpstreamRelayMonitoringRunnerRunsRecommendationOnFirstCycle(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{policy: &UpstreamRelayMonitoringPolicy{
		AutoRecommendationEnabled:     true,
		RecommendationIntervalMinutes: 45,
		FailureRetryIntervalMinutes:   5,
	}}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	now := time.Now()

	runner.runCycle(now)
	waitRunnerIdle(t, runner)

	_, _, recommendationCalls := svc.counts()
	require.Equal(t, 1, recommendationCalls)
	require.WithinDuration(t, now.Add(45*time.Minute), runnerJobNextRunAt(runner, "recommendation"), time.Second)
}

func TestUpstreamRelayMonitoringRunnerSchedulesRecommendationRetryAfterFailure(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{
		policy: &UpstreamRelayMonitoringPolicy{
			AutoRecommendationEnabled:     true,
			RecommendationIntervalMinutes: 60,
			FailureRetryIntervalMinutes:   7,
		},
		recommendationErr: errors.New("recommendation failed"),
	}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	now := time.Now()

	runner.runCycle(now)
	waitRunnerIdle(t, runner)

	require.WithinDuration(t, now.Add(7*time.Minute), runnerJobNextRunAt(runner, "recommendation"), time.Second)
}

func TestUpstreamRelayMonitoringRunnerSchedulesRecommendationRetryWhenApplyFailsAfterRunCreated(t *testing.T) {
	svc := &upstreamRelayMonitoringRunnerStub{
		policy: &UpstreamRelayMonitoringPolicy{
			AutoRecommendationEnabled:     true,
			RecommendationIntervalMinutes: 60,
			FailureRetryIntervalMinutes:   9,
		},
		recommendationErr: errors.New("apply failed"),
		recommendationResultOnErr: &UpstreamRelayRecommendationAutomationResult{
			Run: &UpstreamRelayRecommendationRun{
				ID:              77,
				Status:          UpstreamRelayRunStatusSuccess,
				SuggestionCount: 2,
			},
		},
	}
	runner := newUpstreamRelayMonitoringRunner(svc, time.Hour)
	now := time.Now()

	runner.runCycle(now)
	waitRunnerIdle(t, runner)

	_, _, recommendationCalls := svc.counts()
	require.Equal(t, 1, recommendationCalls)
	require.WithinDuration(t, now.Add(9*time.Minute), runnerJobNextRunAt(runner, "recommendation"), time.Second)
	require.Contains(t, runnerJobLastError(runner, "recommendation"), "apply failed")
}
