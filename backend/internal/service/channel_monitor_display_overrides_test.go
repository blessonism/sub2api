//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorDisplayOverridesValidation(t *testing.T) {
	ptr := func(v float64) *float64 { return &v }
	negativeLatency := -1
	cases := []domain.ChannelMonitorDisplayOverrides{
		{VisibleRateMultiplier: ptr(-0.1)}, {VisibleRateMultiplier: ptr(math.Inf(1))},
		{CacheRate24h: ptr(1.01)}, {CacheRate24h: ptr(-0.1)},
		{OutputTPS5m: ptr(-1)}, {OutputTPS5m: ptr(math.NaN())},
		{Availability7d: ptr(100.1)}, {Availability15d: ptr(-1)}, {Availability30d: ptr(math.Inf(-1))},
		{PrimaryLatencyMs: &negativeLatency},
	}
	for _, o := range cases {
		svc := NewChannelMonitorService(nil, nil)
		_, err := svc.Create(context.Background(), ChannelMonitorCreateParams{DisplayOverrides: o})
		require.Error(t, err)
		_, err = svc.Update(context.Background(), 1, ChannelMonitorUpdateParams{DisplayOverrides: &o})
		require.Error(t, err)
	}
	zero := 0
	require.NoError(t, validateMonitorDisplayOverrides(domain.ChannelMonitorDisplayOverrides{VisibleRateMultiplier: ptr(0), CacheRate24h: ptr(0), OutputTPS5m: ptr(0), PrimaryLatencyMs: &zero, Availability7d: ptr(0), Availability15d: ptr(100), Availability30d: ptr(100)}))
	require.NoError(t, validateMonitorDisplayOverrides(domain.ChannelMonitorDisplayOverrides{}))
}

func TestChannelMonitorDisplayOverridesPreserveRawProbeHistory(t *testing.T) {
	zero, manualAvailability, measured := 0, 0.0, 1500
	overrides := domain.ChannelMonitorDisplayOverrides{PrimaryLatencyMs: &zero, Availability7d: &manualAvailability}
	latest := &ChannelMonitorLatest{LatencyMs: &measured, CheckedAt: time.Now()}
	history := []*ChannelMonitorHistoryEntry{{LatencyMs: &measured, EffectiveStatus: MonitorStatusFailed}}
	summary := MonitorStatusSummary{PrimaryStatus: MonitorStatusFailed, PrimaryLatencyMs: &measured, Availability7d: 75}
	view := buildUserViewFromSummary(&ChannelMonitor{DisplayOverrides: overrides}, summary, latest, history)
	require.Zero(t, *view.PrimaryLatencyMs)
	require.Zero(t, view.Availability7d)
	require.Equal(t, MonitorStatusFailed, view.PrimaryStatus)
	require.Equal(t, measured, *view.Timeline[0].LatencyMs)
	require.Equal(t, measured, *latest.LatencyMs)
	require.Equal(t, measured, *summary.PrimaryLatencyMs)
	require.Equal(t, 75.0, summary.Availability7d)
}

func TestChannelMonitorDisplayOverridesPermissionCacheAndRestore(t *testing.T) {
	automaticCache, automaticTPS, manualRate, manualCache, manualTPS := .2, 10.0, .185, .8, 30.0
	id, deniedID, latency := int64(1), int64(2), 200
	repo := &displayMetricsStub{load: func(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		return map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics{{1, "gpt-test"}: {&automaticCache, &automaticTPS}}, nil
	}}
	svc := displayTestService(repo, time.Now)
	overrides := domain.ChannelMonitorDisplayOverrides{VisibleRateMultiplier: &manualRate, CacheRate24h: &manualCache, OutputTPS5m: &manualTPS, PrimaryLatencyMs: &latency}
	manual := &UserMonitorView{GroupID: &id, PrimaryModel: "gpt-test", DisplayOverrides: overrides}
	denied := &UserMonitorView{GroupID: &deniedID, PrimaryModel: "gpt-test", DisplayOverrides: overrides}
	svc.Enrich(context.Background(), 1, []*UserMonitorView{manual, denied})
	require.Equal(t, manualRate, *manual.VisibleRateMultiplier)
	require.Equal(t, manualCache, *manual.CacheRate24h)
	require.Equal(t, manualTPS, *manual.OutputTPS5m)
	require.Nil(t, denied.VisibleRateMultiplier)
	require.Nil(t, denied.CacheRate24h)
	require.Nil(t, denied.PublicDisplayOverrides().OutputTPS5m)
	require.Equal(t, &latency, denied.PublicDisplayOverrides().PrimaryLatencyMs)
	// 手动值不进入共享聚合缓存；另一个用户仍使用自己的倍率与自动统计。
	auto := &UserMonitorView{GroupID: &id, PrimaryModel: "gpt-test"}
	svc.Enrich(context.Background(), 2, []*UserMonitorView{auto})
	require.Equal(t, .5, *auto.VisibleRateMultiplier)
	require.Equal(t, automaticCache, *auto.CacheRate24h)
	require.Equal(t, automaticTPS, *auto.OutputTPS5m)
	manual.DisplayOverrides = domain.ChannelMonitorDisplayOverrides{}
	svc.Enrich(context.Background(), 1, []*UserMonitorView{manual})
	require.Equal(t, .25, *manual.VisibleRateMultiplier)
	require.Equal(t, automaticCache, *manual.CacheRate24h)
	require.EqualValues(t, 1, repo.calls.Load())
	// 查询失败仍可使用经过授权的手动值；权限查询失败则不能绕过隔离。
	failed := displayTestService(&displayMetricsStub{load: func(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
		return nil, errors.New("unavailable")
	}}, time.Now)
	zero := 0.0
	manual.DisplayOverrides = domain.ChannelMonitorDisplayOverrides{VisibleRateMultiplier: &zero, CacheRate24h: &zero, OutputTPS5m: &zero}
	failed.Enrich(context.Background(), 1, []*UserMonitorView{manual})
	require.Zero(t, *manual.VisibleRateMultiplier)
	require.Zero(t, *manual.CacheRate24h)
	require.Zero(t, *manual.OutputTPS5m)
	failed.groups = displayGroupsStub{groups: func(int64) ([]Group, error) { return nil, errors.New("unavailable") }}
	failed.Enrich(context.Background(), 1, []*UserMonitorView{manual})
	require.Nil(t, manual.VisibleRateMultiplier)
	require.Nil(t, manual.PublicDisplayOverrides().CacheRate24h)
}
