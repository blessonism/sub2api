package service

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func validateMonitorDisplayOverrides(o domain.ChannelMonitorDisplayOverrides) error {
	for _, field := range []struct {
		name  string
		value *float64
		max   float64
	}{
		{"visible_rate_multiplier", o.VisibleRateMultiplier, 0},
		{"cache_rate_24h", o.CacheRate24h, 1},
		{"output_tps_5m", o.OutputTPS5m, 0},
		{"availability_7d", o.Availability7d, 100},
		{"availability_15d", o.Availability15d, 100},
		{"availability_30d", o.Availability30d, 100},
	} {
		if field.value == nil {
			continue
		}
		v := *field.value
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || (field.max > 0 && v > field.max) {
			return infraerrors.BadRequest("INVALID_MONITOR_DISPLAY_OVERRIDE", "Invalid display override: "+field.name)
		}
	}
	if o.PrimaryLatencyMs != nil && *o.PrimaryLatencyMs < 0 {
		return infraerrors.BadRequest("INVALID_MONITOR_DISPLAY_OVERRIDE", "Response time must be nonnegative")
	}
	return nil
}

// PublicDisplayOverrides 不向无分组权限的用户暴露该分组的手动倍率和统计值。
func (v *UserMonitorView) PublicDisplayOverrides() domain.ChannelMonitorDisplayOverrides {
	o := v.DisplayOverrides
	if v.VisibleRateMultiplier == nil {
		o.VisibleRateMultiplier, o.CacheRate24h, o.OutputTPS5m = nil, nil, nil
	}
	return o
}
