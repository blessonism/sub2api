package langfuse

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"strconv"
	"time"
)

const (
	eventTypeTraceCreate      = "trace-create"
	eventTypeGenerationCreate = "generation-create"

	// traceName 是所有观测 trace 的固定名，便于在 UI 上按 name 过滤。
	traceName = "sub2api.model-call"

	// tsLayout 是 Langfuse ingestion 接受的时间格式（毫秒精度，UTC）。
	tsLayout = "2006-01-02T15:04:05.000Z"

	// costSourceActual 标记费用来源为业务库实扣值，供 UI/导出侧区分口径。
	costSourceActual = "actual_charge"

	levelDefault = "DEFAULT"
	levelError   = "ERROR"
)

// event 是 ingestion 批量里的单条事件。
type event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Body      map[string]any `json:"body"`
}

// batch 是 ingestion 请求体。
type batch struct {
	Batch []event `json:"batch"`
}

// buildBatch 把 N 条 Call 展开成 2N 条事件：每条形如 trace-create + generation-create。
func buildBatch(calls []Call, release string) batch {
	out := batch{Batch: make([]event, 0, len(calls)*2)}
	for _, c := range calls {
		id := newID()
		traceID := "tr-" + id
		genID := "gn-" + id
		ts := formatTime(c.StartedAt)
		out.Batch = append(out.Batch,
			event{
				ID:        "ev-" + id + "-trace",
				Type:      eventTypeTraceCreate,
				Timestamp: ts,
				Body:      traceBody(c, traceID, release),
			},
			event{
				ID:        "ev-" + id + "-generation",
				Type:      eventTypeGenerationCreate,
				Timestamp: ts,
				Body:      generationBody(c, traceID, genID),
			},
		)
	}
	return out
}

func traceBody(c Call, traceID, release string) map[string]any {
	body := map[string]any{
		"id":        traceID,
		"name":      traceName,
		"timestamp": formatTime(c.StartedAt),
		"metadata":  metadata(c),
	}
	// Langfuse 的检索维度是字符串；userId 用十进制串落位，空值不落键。
	if c.UserID != 0 {
		body["userId"] = strconv.FormatInt(c.UserID, 10)
	}
	if c.SessionID != "" {
		body["sessionId"] = c.SessionID
	}
	if release != "" {
		body["release"] = release
	}
	if c.Input != "" {
		body["input"] = c.Input
	}
	if c.Output != "" {
		body["output"] = c.Output
	}
	return body
}

func generationBody(c Call, traceID, genID string) map[string]any {
	end := c.StartedAt.Add(c.Duration)
	usage := map[string]any{
		"input":  c.InputTokens,
		"output": c.OutputTokens,
		"total":  c.TotalTokens(),
	}
	if c.CacheReadTokens > 0 {
		usage["cache_read"] = c.CacheReadTokens
	}
	if c.CacheWriteTokens > 0 {
		usage["cache_write"] = c.CacheWriteTokens
	}
	if c.ImageInputTokens > 0 {
		usage["image_input"] = c.ImageInputTokens
	}
	if c.ImageOutputTokens > 0 {
		usage["image_output"] = c.ImageOutputTokens
	}

	body := map[string]any{
		"id":      genID,
		"traceId": traceID,
		"name":    c.Model,
		"model":   c.Model,
		// costDetails 是 Langfuse 的权威费用字段：给了它就不再按价格表推算，
		// 这正是"cost 等于实扣而非标价"的落地方式。
		"costDetails": map[string]any{
			"total": clampFloat(c.Cost),
		},
		"usageDetails": usage,
		"startTime":    formatTime(c.StartedAt),
		"endTime":      formatTime(end),
		"level":        level(c.Status),
		"metadata":     metadata(c),
	}
	if c.Status == "billing_error" && c.ErrorKind != "" {
		body["statusMessage"] = c.ErrorKind
	}
	// 正文挂在 generation 上：UI 的输入/输出面板与全文检索都走这两个字段。
	if c.Input != "" {
		body["input"] = c.Input
	}
	if c.Output != "" {
		body["output"] = c.Output
	}
	return body
}

func metadata(c Call) map[string]any {
	meta := map[string]any{
		"cost_source": costSourceActual,
		"status":      c.Status,
	}
	if c.RequestID != "" {
		meta["request_id"] = c.RequestID
	}
	if c.Endpoint != "" {
		meta["endpoint"] = c.Endpoint
	}
	if c.BillingType != "" {
		meta["billing_type"] = c.BillingType
	}
	if c.BillingMode != "" {
		meta["billing_mode"] = c.BillingMode
	}
	if c.ServiceTier != "" {
		meta["service_tier"] = c.ServiceTier
	}
	if c.Stream {
		meta["stream"] = true
	}
	if c.InputTruncated {
		meta["input_truncated"] = true
	}
	if c.OutputTruncated {
		meta["output_truncated"] = true
	}
	if c.AccountID != 0 {
		meta["account_id"] = strconv.FormatInt(c.AccountID, 10)
	}
	if c.APIKeyID != 0 {
		meta["api_key_id"] = strconv.FormatInt(c.APIKeyID, 10)
	}
	if c.GroupID != 0 {
		meta["group_id"] = strconv.FormatInt(c.GroupID, 10)
	}
	if c.RateMultiplier != 0 {
		meta["rate_multiplier"] = clampFloat(c.RateMultiplier)
	}
	if c.VisibleRateMultiplier != 0 {
		meta["visible_rate_multiplier"] = clampFloat(c.VisibleRateMultiplier)
	}
	if c.AccountRateMultiplier != 0 {
		meta["account_rate_multiplier"] = clampFloat(c.AccountRateMultiplier)
	}
	for k, v := range c.Metadata {
		if k == "" || v == "" {
			continue
		}
		meta[k] = v
	}
	return meta
}

func level(status string) string {
	if status == "billing_error" {
		return levelError
	}
	return levelDefault
}

// clampFloat 保证 JSON 可编码：NaN/Inf 无法序列化，会把整批上报打掉。
func clampFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format(tsLayout)
}

func newID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// 熵源异常时退化为时间戳派生，仍然保持唯一性可用。
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf[:])
}
