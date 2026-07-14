package domain

const (
	TimeRatePriorityScheduleFirst = "schedule_first"
	TimeRatePriorityUserFirst     = "user_first"
	TimeRatePriorityProportional  = "proportional"
)

// GroupTimeRatePeriod 是分组每日循环的分时倍率配置。
type GroupTimeRatePeriod struct {
	StartTime             string  `json:"start_time"`
	EndTime               string  `json:"end_time"`
	RateMultiplier        float64 `json:"rate_multiplier"`
	VisibleRateMultiplier float64 `json:"visible_rate_multiplier"`
	Enabled               bool    `json:"enabled"`
}
