package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func init() { _ = timezone.Init("UTC") }

func timeRateAt(hour, minute int) time.Time {
	return time.Date(2026, 7, 13, hour, minute, 0, 0, time.UTC)
}

func TestNormalizeAndValidateTimeRateStrategy(t *testing.T) {
	valid := []GroupTimeRatePeriod{
		{StartTime: "22:00", EndTime: "24:00", RateMultiplier: 0.02, VisibleRateMultiplier: 0.2, Enabled: true},
		{StartTime: "00:00", EndTime: "06:00", RateMultiplier: 0, VisibleRateMultiplier: 0, Enabled: true},
		{StartTime: "05:00", EndTime: "07:00", RateMultiplier: 1, VisibleRateMultiplier: 1, Enabled: false},
	}
	priority, periods, err := NormalizeAndValidateTimeRateStrategy("", valid)
	require.NoError(t, err)
	require.Equal(t, TimeRatePriorityScheduleFirst, priority)
	require.Equal(t, "00:00", periods[0].StartTime)

	cases := []struct {
		name     string
		priority string
		periods  []GroupTimeRatePeriod
	}{
		{"invalid priority", "other", valid},
		{"invalid start", "", []GroupTimeRatePeriod{{StartTime: "0:00", EndTime: "01:00"}}},
		{"invalid end", "", []GroupTimeRatePeriod{{StartTime: "00:00", EndTime: "00:00"}}},
		{"cross midnight", "", []GroupTimeRatePeriod{{StartTime: "22:00", EndTime: "02:00"}}},
		{"negative actual", "", []GroupTimeRatePeriod{{StartTime: "00:00", EndTime: "01:00", RateMultiplier: -1}}},
		{"negative visible", "", []GroupTimeRatePeriod{{StartTime: "00:00", EndTime: "01:00", VisibleRateMultiplier: -1}}},
		{"overlap", "", []GroupTimeRatePeriod{
			{StartTime: "00:00", EndTime: "06:00", Enabled: true},
			{StartTime: "05:59", EndTime: "07:00", Enabled: true},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := NormalizeAndValidateTimeRateStrategy(tc.priority, tc.periods)
			require.Error(t, err)
		})
	}
}

func TestGroupTimeRatePeriodAtBoundaries(t *testing.T) {
	g := &Group{TimeRatePeriods: []GroupTimeRatePeriod{
		{StartTime: "00:00", EndTime: "06:00", RateMultiplier: 0.5, Enabled: true},
		{StartTime: "22:00", EndTime: "24:00", RateMultiplier: 0.2, Enabled: true},
	}}
	require.Equal(t, 0.5, g.TimeRatePeriodAt(timeRateAt(0, 0)).RateMultiplier)
	require.Nil(t, g.TimeRatePeriodAt(timeRateAt(6, 0)))
	require.Equal(t, 0.2, g.TimeRatePeriodAt(timeRateAt(23, 59)).RateMultiplier)
}

func TestGroupTimeRatePeriodAtUsesServerTimezone(t *testing.T) {
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	t.Cleanup(func() { require.NoError(t, timezone.Init("UTC")) })
	g := &Group{TimeRatePeriods: []GroupTimeRatePeriod{{
		StartTime: "08:00", EndTime: "09:00", RateMultiplier: 0.5, Enabled: true,
	}}}
	require.NotNil(t, g.TimeRatePeriodAt(time.Date(2026, 7, 13, 0, 30, 0, 0, time.UTC)))
	require.Nil(t, g.TimeRatePeriodAt(time.Date(2026, 7, 13, 1, 0, 0, 0, time.UTC)))
}

func TestGroupEffectiveTimeRatePriorities(t *testing.T) {
	visibleBase := 1.5
	period := GroupTimeRatePeriod{StartTime: "10:00", EndTime: "12:00", RateMultiplier: 0.5, VisibleRateMultiplier: 0.3, Enabled: true}
	g := &Group{RateMultiplier: 2, VisibleRateMultiplier: &visibleBase, TimeRatePeriods: []GroupTimeRatePeriod{period}}

	g.TimeRatePriority = TimeRatePriorityScheduleFirst
	actual, visible := g.EffectiveTimeRate(0.8, true, 0.6, true, timeRateAt(11, 0))
	require.Equal(t, 0.5, actual)
	require.Equal(t, 0.3, visible)

	g.TimeRatePriority = TimeRatePriorityUserFirst
	actual, visible = g.EffectiveTimeRate(0.8, true, 0.6, true, timeRateAt(11, 0))
	require.Equal(t, 0.8, actual)
	require.Equal(t, 0.6, visible)
	actual, visible = g.EffectiveTimeRate(2, false, 1.5, false, timeRateAt(11, 0))
	require.Equal(t, 0.5, actual)
	require.Equal(t, 0.3, visible)

	g.TimeRatePriority = TimeRatePriorityProportional
	actual, visible = g.EffectiveTimeRate(0.8, true, 0.6, true, timeRateAt(11, 0))
	require.InDelta(t, 0.2, actual, 1e-9)
	require.InDelta(t, 0.12, visible, 1e-9)
}

func TestGroupEffectiveTimeRatePreservesVisibleFallback(t *testing.T) {
	visibleBase := 1.5
	g := &Group{
		RateMultiplier:        2,
		VisibleRateMultiplier: &visibleBase,
		TimeRatePriority:      TimeRatePriorityUserFirst,
		TimeRatePeriods: []GroupTimeRatePeriod{{
			StartTime: "10:00", EndTime: "12:00", RateMultiplier: 0.5, VisibleRateMultiplier: 0.3, Enabled: true,
		}},
	}

	// 未配置专属可见倍率时，现有规则会把专属真实倍率同时作为可见倍率。
	actual, visible := g.EffectiveTimeRate(0.8, true, 0.8, true, timeRateAt(11, 0))
	require.Equal(t, 0.8, actual)
	require.Equal(t, 0.8, visible)

	// 独立可见倍率仍按同一优先模式独立计算。
	actual, visible = g.EffectiveTimeRate(0.8, true, 0.6, true, timeRateAt(11, 0))
	require.Equal(t, 0.8, actual)
	require.Equal(t, 0.6, visible)
}

func TestMigratedPeakRateKeepsProportionalBilling(t *testing.T) {
	const groupBase = 2.0
	const oldPeakFactor = 1.5
	g := &Group{
		RateMultiplier:   groupBase,
		TimeRatePriority: TimeRatePriorityProportional,
		TimeRatePeriods: []GroupTimeRatePeriod{{
			StartTime: "10:00", EndTime: "12:00",
			RateMultiplier: groupBase * oldPeakFactor, VisibleRateMultiplier: groupBase * oldPeakFactor, Enabled: true,
		}},
	}

	actual, visible := g.EffectiveTimeRate(0.8, true, 0.8, true, timeRateAt(11, 0))
	require.InDelta(t, 0.8*oldPeakFactor, actual, 1e-9)
	require.InDelta(t, 0.8*oldPeakFactor, visible, 1e-9)
}

func TestTimeRateInheritedAndIndependentImageMultipliers(t *testing.T) {
	g := &Group{
		RateMultiplier:   1,
		TimeRatePriority: TimeRatePriorityScheduleFirst,
		TimeRatePeriods: []GroupTimeRatePeriod{{
			StartTime: "10:00", EndTime: "12:00", RateMultiplier: 0.4, VisibleRateMultiplier: 0.4, Enabled: true,
		}},
	}
	text, image := computeTimeRateAwareMultipliers(&APIKey{Group: g}, 1, false, timeRateAt(11, 0))
	require.Equal(t, 0.4, text)
	require.Equal(t, 0.4, image)
	require.Equal(t, 0.4, resolveVideoRateMultiplier(&APIKey{Group: g}, text))

	g.ImageRateIndependent = true
	g.ImageRateMultiplier = 0.7
	g.VideoRateIndependent = true
	g.VideoRateMultiplier = 0.8
	_, image = computeTimeRateAwareMultipliers(&APIKey{Group: g}, 1, false, timeRateAt(11, 0))
	require.Equal(t, 0.7, image)
	require.Equal(t, 0.8, resolveVideoRateMultiplier(&APIKey{Group: g}, text))
}

func TestTimeRateSnapshotRoundTrip(t *testing.T) {
	apiKey := &APIKey{User: &User{ID: 1, Status: StatusActive, Role: RoleUser}, Group: &Group{
		TimeRatePriority: TimeRatePriorityProportional,
		TimeRatePeriods: []GroupTimeRatePeriod{{
			StartTime: "10:00", EndTime: "12:00", RateMultiplier: 0.4, VisibleRateMultiplier: 0.3, Enabled: true,
		}},
	}}
	svc := &APIKeyService{}
	restored := svc.snapshotToAPIKey("k", svc.snapshotFromAPIKey(context.Background(), apiKey))
	require.Equal(t, TimeRatePriorityProportional, restored.Group.TimeRatePriority)
	require.Equal(t, apiKey.Group.TimeRatePeriods, restored.Group.TimeRatePeriods)
}
