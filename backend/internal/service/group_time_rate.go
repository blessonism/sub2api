package service

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type GroupTimeRatePeriod = domain.GroupTimeRatePeriod

const (
	TimeRatePriorityScheduleFirst = domain.TimeRatePriorityScheduleFirst
	TimeRatePriorityUserFirst     = domain.TimeRatePriorityUserFirst
	TimeRatePriorityProportional  = domain.TimeRatePriorityProportional
)

func parseTimeRateMinutes(value string, allow24 bool) (int, bool) {
	if allow24 && value == "24:00" {
		return 24 * 60, true
	}
	if len(value) != 5 || value[2] != ':' {
		return 0, false
	}
	h1, h2, m1, m2 := value[0]-'0', value[1]-'0', value[3]-'0', value[4]-'0'
	if h1 > 9 || h2 > 9 || m1 > 9 || m2 > 9 {
		return 0, false
	}
	hour, minute := int(h1)*10+int(h2), int(m1)*10+int(m2)
	if hour > 23 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

func NormalizeAndValidateTimeRateStrategy(priority string, periods []GroupTimeRatePeriod) (string, []GroupTimeRatePeriod, error) {
	if priority == "" {
		priority = TimeRatePriorityScheduleFirst
	}
	switch priority {
	case TimeRatePriorityScheduleFirst, TimeRatePriorityUserFirst, TimeRatePriorityProportional:
	default:
		return "", nil, fmt.Errorf("invalid time_rate_priority %q", priority)
	}

	normalized := append([]GroupTimeRatePeriod(nil), periods...)
	type enabledWindow struct{ start, end int }
	windows := make([]enabledWindow, 0, len(normalized))
	for i := range normalized {
		period := &normalized[i]
		start, ok := parseTimeRateMinutes(period.StartTime, false)
		if !ok {
			return "", nil, fmt.Errorf("time_rate_periods[%d].start_time must be HH:MM", i)
		}
		end, ok := parseTimeRateMinutes(period.EndTime, true)
		if !ok || end == 0 {
			return "", nil, fmt.Errorf("time_rate_periods[%d].end_time must be HH:MM or 24:00", i)
		}
		if start >= end {
			return "", nil, fmt.Errorf("time_rate_periods[%d].end_time must be later than start_time", i)
		}
		if math.IsNaN(period.RateMultiplier) || math.IsInf(period.RateMultiplier, 0) || period.RateMultiplier < 0 {
			return "", nil, fmt.Errorf("time_rate_periods[%d].rate_multiplier must be >= 0", i)
		}
		if math.IsNaN(period.VisibleRateMultiplier) || math.IsInf(period.VisibleRateMultiplier, 0) || period.VisibleRateMultiplier < 0 {
			return "", nil, fmt.Errorf("time_rate_periods[%d].visible_rate_multiplier must be >= 0", i)
		}
		if period.Enabled {
			windows = append(windows, enabledWindow{start: start, end: end})
		}
	}

	sort.Slice(normalized, func(i, j int) bool { return normalized[i].StartTime < normalized[j].StartTime })
	sort.Slice(windows, func(i, j int) bool { return windows[i].start < windows[j].start })
	for i := 1; i < len(windows); i++ {
		if windows[i].start < windows[i-1].end {
			return "", nil, errors.New("enabled time_rate_periods must not overlap")
		}
	}
	return priority, normalized, nil
}

func (g *Group) TimeRatePeriodAt(now time.Time) *GroupTimeRatePeriod {
	if g == nil {
		return nil
	}
	local := now.In(timezone.Location())
	minute := local.Hour()*60 + local.Minute()
	for i := range g.TimeRatePeriods {
		period := &g.TimeRatePeriods[i]
		if !period.Enabled {
			continue
		}
		start, okStart := parseTimeRateMinutes(period.StartTime, false)
		end, okEnd := parseTimeRateMinutes(period.EndTime, true)
		if okStart && okEnd && start < end && minute >= start && minute < end {
			return period
		}
	}
	return nil
}

func applyTimeRatePriority(priority string, base, scheduled float64, hasUserOverride bool) float64 {
	switch priority {
	case TimeRatePriorityUserFirst:
		if hasUserOverride {
			return base
		}
	case TimeRatePriorityProportional:
		return scheduled
	}
	return scheduled
}

func proportionalTimeRate(base, groupBase, scheduled float64) float64 {
	if groupBase <= 0 {
		return scheduled
	}
	return base * scheduled / groupBase
}

func (g *Group) EffectiveTimeRate(actual float64, actualOverride bool, visible float64, visibleOverride bool, now time.Time) (float64, float64) {
	period := g.TimeRatePeriodAt(now)
	if period == nil {
		return actual, visible
	}
	if g.TimeRatePriority == TimeRatePriorityProportional {
		return proportionalTimeRate(actual, g.RateMultiplier, period.RateMultiplier),
			proportionalTimeRate(visible, g.VisibleEffectiveRateMultiplier(), period.VisibleRateMultiplier)
	}
	return applyTimeRatePriority(g.TimeRatePriority, actual, period.RateMultiplier, actualOverride),
		applyTimeRatePriority(g.TimeRatePriority, visible, period.VisibleRateMultiplier, visibleOverride)
}

func (g *Group) CurrentVisibleRate(now time.Time) float64 {
	base := g.VisibleEffectiveRateMultiplier()
	_, visible := g.EffectiveTimeRate(g.RateMultiplier, false, base, false, now)
	return visible
}

func (g *Group) CurrentActualRate(base float64, hasUserOverride bool, now time.Time) float64 {
	actual, _ := g.EffectiveTimeRate(base, hasUserOverride, g.VisibleEffectiveRateMultiplier(), false, now)
	return actual
}

func computeTimeRateAwareMultipliers(apiKey *APIKey, base float64, hasUserOverride bool, now time.Time) (text, image float64) {
	text, image, _ = computeTimeRateAwareRates(apiKey, base, hasUserOverride, base, hasUserOverride, now)
	return
}

func computeTimeRateAwareRates(apiKey *APIKey, actual float64, actualOverride bool, visible float64, visibleOverride bool, now time.Time) (text, image, currentVisible float64) {
	text, currentVisible = actual, visible
	if apiKey != nil && apiKey.Group != nil {
		text, currentVisible = apiKey.Group.EffectiveTimeRate(actual, actualOverride, visible, visibleOverride, now)
	}
	image = resolveImageRateMultiplier(apiKey, text)
	return
}
