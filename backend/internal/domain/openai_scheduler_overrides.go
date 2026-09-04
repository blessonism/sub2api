package domain

// GroupOpenAISchedulerOverrides 是分组级 OpenAI/Codex 高级调度覆盖。
// 指针字段为 nil 表示继承全局配置；空对象/NULL 表示全部继承。
type GroupOpenAISchedulerOverrides struct {
	LBTopK                *int     `json:"lb_top_k,omitempty"`
	WeightTTFT            *float64 `json:"weight_ttft,omitempty"`
	WeightErrorRate       *float64 `json:"weight_error_rate,omitempty"`
	WeightLoad            *float64 `json:"weight_load,omitempty"`
	TTFTMaxRatio          *float64 `json:"ttft_max_ratio,omitempty"`
	StickyEscapeTTFTMs    *int     `json:"sticky_escape_ttft_ms,omitempty"`
	StickyEscapeErrorRate *float64 `json:"sticky_escape_error_rate,omitempty"`
}

// IsZero 表示没有任何覆盖项。
func (o GroupOpenAISchedulerOverrides) IsZero() bool {
	return o.LBTopK == nil &&
		o.WeightTTFT == nil &&
		o.WeightErrorRate == nil &&
		o.WeightLoad == nil &&
		o.TTFTMaxRatio == nil &&
		o.StickyEscapeTTFTMs == nil &&
		o.StickyEscapeErrorRate == nil
}
