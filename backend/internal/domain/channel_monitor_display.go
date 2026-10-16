package domain

// ChannelMonitorDisplayOverrides 仅覆盖用户卡片展示，不改计费、探测或用量记录。
// nil 使用自动值；非 nil 的 0 是有效手动值。
type ChannelMonitorDisplayOverrides struct {
	VisibleRateMultiplier *float64 `json:"visible_rate_multiplier,omitempty"`
	CacheRate24h          *float64 `json:"cache_rate_24h,omitempty"`
	OutputTPS5m           *float64 `json:"output_tps_5m,omitempty"`
	PrimaryLatencyMs      *int     `json:"primary_latency_ms,omitempty"`
	Availability7d        *float64 `json:"availability_7d,omitempty"`
	Availability15d       *float64 `json:"availability_15d,omitempty"`
	Availability30d       *float64 `json:"availability_30d,omitempty"`
}
