//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type displayGroupsStub struct {
	groups func(int64) ([]Group, error)
}

func (s displayGroupsStub) GetAvailableGroups(_ context.Context, id int64) ([]Group, error) {
	return s.groups(id)
}

type displayMetricsStub struct {
	calls atomic.Int32
	load  func(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error)
}

func (s *displayMetricsStub) BatchMetrics(ctx context.Context, keys []ChannelMonitorMetricKey, now time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
	s.calls.Add(1)
	return s.load(ctx, keys, now)
}

func displayTestService(repo *displayMetricsStub, now func() time.Time) *ChannelMonitorDisplayService {
	return &ChannelMonitorDisplayService{repo: repo, now: now,
		cache: make(map[ChannelMonitorMetricKey]channelMonitorDisplayCacheEntry),
		groups: displayGroupsStub{groups: func(userID int64) ([]Group, error) {
			return []Group{{ID: 1, RateMultiplier: float64(userID) / 4}}, nil
		}},
	}
}

func TestChannelMonitorDisplayPermissionRateAndCache(t *testing.T) {
	now := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	cache, tps, id, hiddenID := 0.0, 15.2, int64(1), int64(2)
	repo := &displayMetricsStub{load: func(ctx context.Context, keys []ChannelMonitorMetricKey, at time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		require.Equal(t, []ChannelMonitorMetricKey{{1, "gpt-test"}}, keys)
		require.Equal(t, now, at)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
		return map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics{{1, "gpt-test"}: {&cache, &tps}}, nil
	}}
	svc := displayTestService(repo, func() time.Time { return now })
	view := &UserMonitorView{GroupID: &id, PrimaryModel: "gpt-test", PrimaryStatus: "operational"}
	hidden := &UserMonitorView{GroupID: &hiddenID, PrimaryModel: "gpt-test"}
	quota := &UserMonitorView{GroupID: &id, PrimaryModel: "quota", CheckMode: MonitorCheckModeQuota}
	svc.Enrich(context.Background(), 1, []*UserMonitorView{view, hidden, quota})
	require.Equal(t, 0.25, *view.VisibleRateMultiplier)
	require.Equal(t, 0.0, *view.CacheRate24h)
	require.Equal(t, 15.2, *view.OutputTPS5m)
	require.Nil(t, hidden.VisibleRateMultiplier)
	require.Nil(t, hidden.OutputTPS5m)
	require.Nil(t, quota.CacheRate24h)
	require.NotNil(t, quota.VisibleRateMultiplier)

	// 同一份统计缓存不串个人倍率；权限收回时也不会命中已缓存的统计。
	svc.Enrich(context.Background(), 2, []*UserMonitorView{view})
	require.Equal(t, 0.5, *view.VisibleRateMultiplier)
	require.EqualValues(t, 1, repo.calls.Load())
	svc.groups = displayGroupsStub{groups: func(int64) ([]Group, error) { return nil, nil }}
	svc.Enrich(context.Background(), 2, []*UserMonitorView{view})
	require.Nil(t, view.VisibleRateMultiplier)
	require.Nil(t, view.CacheRate24h)
	require.Equal(t, "operational", view.PrimaryStatus)
}

func TestChannelMonitorDisplayCacheExpiryAndUnknown(t *testing.T) {
	now := time.Now()
	repo := &displayMetricsStub{load: func(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		return map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics{}, nil
	}}
	svc := displayTestService(repo, func() time.Time { return now })
	id := int64(1)
	view := &UserMonitorView{GroupID: &id, PrimaryModel: "gpt-test"}
	for i := 0; i < 2; i++ {
		svc.Enrich(context.Background(), 1, []*UserMonitorView{view})
		require.Nil(t, view.OutputTPS5m)
		require.Nil(t, view.CacheRate24h)
	}
	require.EqualValues(t, 1, repo.calls.Load())
	now = now.Add(30 * time.Second)
	svc.Enrich(context.Background(), 1, []*UserMonitorView{view})
	require.EqualValues(t, 2, repo.calls.Load())
}

func TestChannelMonitorDisplayFailureDoesNotCacheOrBlockProbes(t *testing.T) {
	repo := &displayMetricsStub{load: func(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		return nil, errors.New("database unavailable")
	}}
	svc := displayTestService(repo, time.Now)
	id := int64(1)
	view := &UserMonitorView{GroupID: &id, PrimaryModel: "gpt-test", PrimaryStatus: "degraded"}
	for i := 0; i < 2; i++ {
		svc.Enrich(context.Background(), 1, []*UserMonitorView{view})
		require.Equal(t, "degraded", view.PrimaryStatus)
		require.Nil(t, view.OutputTPS5m)
	}
	require.EqualValues(t, 2, repo.calls.Load())
	svc.groups = displayGroupsStub{groups: func(int64) ([]Group, error) { return nil, errors.New("permission lookup failed") }}
	svc.Enrich(context.Background(), 1, []*UserMonitorView{view})
	require.Nil(t, view.VisibleRateMultiplier)
	require.EqualValues(t, 2, repo.calls.Load())
}

func TestChannelMonitorDisplayCoalescesConcurrentRequests(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	repo := &displayMetricsStub{load: func(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		close(started)
		<-release
		return map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics{}, nil
	}}
	svc := displayTestService(repo, time.Now)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.metrics(context.Background(), []ChannelMonitorMetricKey{{1, "gpt-test"}})
			require.NoError(t, err)
		}()
	}
	<-started
	close(release)
	wg.Wait()
	require.EqualValues(t, 1, repo.calls.Load())
}

func TestChannelMonitorDisplayQuotaAndUnlinkedSkipStatistics(t *testing.T) {
	repo := &displayMetricsStub{load: func(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		t.Error("must not query statistics")
		return nil, nil
	}}
	svc := displayTestService(repo, time.Now)
	id := int64(1)
	svc.Enrich(context.Background(), 1, []*UserMonitorView{{GroupID: &id, PrimaryModel: "quota", CheckMode: MonitorCheckModeQuota}, {PrimaryModel: "primary"}})
	require.Zero(t, repo.calls.Load())
}

func TestChannelMonitorDisplayCancellationDoesNotCancelSharedQuery(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	repo := &displayMetricsStub{load: func(ctx context.Context, _ []ChannelMonitorMetricKey, _ time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		close(started)
		<-release
		require.NoError(t, ctx.Err())
		return nil, nil
	}}
	svc := displayTestService(repo, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := svc.metrics(ctx, []ChannelMonitorMetricKey{{1, "model"}}); done <- err }()
	<-started
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	close(release)
	_, err := svc.metrics(context.Background(), []ChannelMonitorMetricKey{{1, "model"}})
	require.NoError(t, err)
	require.EqualValues(t, 1, repo.calls.Load())
}

func TestChannelMonitorDisplayOverlappingBatches(t *testing.T) {
	firstStarted, secondStarted, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	a, b, c := ChannelMonitorMetricKey{1, "a"}, ChannelMonitorMetricKey{1, "b"}, ChannelMonitorMetricKey{1, "c"}
	repo := &displayMetricsStub{load: func(_ context.Context, keys []ChannelMonitorMetricKey, _ time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		if len(keys) == 2 {
			require.Equal(t, []ChannelMonitorMetricKey{a, b}, keys)
			close(firstStarted)
			<-release
		} else {
			require.Equal(t, []ChannelMonitorMetricKey{c}, keys)
			close(secondStarted)
		}
		return nil, nil
	}}
	svc := displayTestService(repo, time.Now)
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	go func() { _, err := svc.metrics(context.Background(), []ChannelMonitorMetricKey{a, b}); firstDone <- err }()
	<-firstStarted
	go func() {
		_, err := svc.metrics(context.Background(), []ChannelMonitorMetricKey{b, c})
		secondDone <- err
	}()
	<-secondStarted
	close(release)
	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)
	require.EqualValues(t, 2, repo.calls.Load())
}

func TestChannelMonitorDisplayUsesAvailableGroupVisibleRateRules(t *testing.T) {
	visible := 1.4
	group := Group{ID: 1, RateMultiplier: 2, VisibleRateMultiplier: &visible, SubscriptionType: SubscriptionTypeStandard,
		TimeRatePriority: TimeRatePriorityUserFirst, TimeRatePeriods: []GroupTimeRatePeriod{{StartTime: "00:00", EndTime: "24:00", RateMultiplier: .5, VisibleRateMultiplier: .3, Enabled: true}}}
	api := &APIKeyService{userRepo: &visibilityUserRepo{user: &User{ID: 1}}, groupRepo: &visibilityGroupRepo{groups: []Group{group}}, userSubRepo: &visibilitySubRepo{},
		userGroupRateRepo: &effectiveVisibleRateRepoStub{actualRates: map[int64]float64{1: .7}, visibleRates: map[int64]float64{1: .6}}}
	svc := NewChannelMonitorDisplayService(nil, api)
	id := int64(1)
	view := &UserMonitorView{GroupID: &id, PrimaryModel: "model"}
	svc.Enrich(context.Background(), 1, []*UserMonitorView{view})
	require.NotNil(t, view.VisibleRateMultiplier)
	require.Equal(t, .6, *view.VisibleRateMultiplier)
	api.userGroupRateRepo = nil
	svc.Enrich(context.Background(), 1, []*UserMonitorView{view})
	require.Equal(t, .3, *view.VisibleRateMultiplier, "time period visible rate is resolved again, not cached")
}
