package langfuse

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/observability/sessiongroup"
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
//
// 归组在这里完成而不是在业务侧：worker 是单消费者，会话/线程/步骤的推进天然
// 串行，业务同步路径仍然只有一次 channel send。
func buildBatch(calls []Call, release string) batch {
	out := batch{Batch: make([]event, 0, len(calls)*2)}
	for _, c := range calls {
		id := newID()
		traceID := "tr-" + id
		genID := "gn-" + id
		ts := formatTime(c.StartedAt)
		grp := sessiongroup.Resolve(sessiongroup.Input{
			ClientSessionID:    c.SessionID,
			UserID:             c.UserID,
			APIKeyID:           c.APIKeyID,
			FirstUserPrompt:    c.FirstUserPrompt,
			RequestFingerprint: c.RequestFingerprint,
			Now:                c.StartedAt,
		})
		out.Batch = append(out.Batch,
			event{
				ID:        "ev-" + id + "-trace",
				Type:      eventTypeTraceCreate,
				Timestamp: ts,
				Body:      traceBody(c, traceID, release, grp),
			},
			event{
				ID:        "ev-" + id + "-generation",
				Type:      eventTypeGenerationCreate,
				Timestamp: ts,
				Body:      generationBody(c, traceID, genID, grp),
			},
		)
	}
	return out
}

func traceBody(c Call, traceID, release string, grp sessiongroup.Group) map[string]any {
	body := map[string]any{
		"id":        traceID,
		"name":      traceDisplayName(grp),
		"timestamp": formatTime(c.StartedAt),
		"metadata":  metadata(c, grp),
	}
	// 会话视图里每张卡片只显示 trace 名，靠 tag 才能过滤；
	// 线程做成 tag 之后可以在 traces 表里按线程筛。
	if grp.ThreadID != "" {
		body["tags"] = []string{"thread:" + grp.ThreadID}
	}
	// Langfuse 的检索维度是字符串；userId 用十进制串落位，空值不落键。
	if c.UserID != 0 {
		body["userId"] = strconv.FormatInt(c.UserID, 10)
	}
	// 会话聚合键：客户端显式给的就用它，没给时用推导值，
	// 两者都没有则不落 sessionId（Langfuse 侧即不聚合）。
	if grp.SessionID != "" {
		body["sessionId"] = grp.SessionID
	}
	if release != "" {
		body["release"] = release
	}
	if display := c.displayInput(); display != "" {
		body["input"] = display
	}
	if c.Output != "" {
		body["output"] = c.Output
	}
	return body
}

// traceDisplayName 让会话视图里的每张卡片可区分：同一个会话有几十上百次调用，
// 全都叫 sub2api.model-call 时，卡片列表没法扫。
func traceDisplayName(grp sessiongroup.Group) string {
	name := traceName
	if short := shortThreadID(grp.ThreadID); short != "" {
		name += " · " + short
	}
	if grp.StepIndex > 0 {
		name += " · #" + strconv.Itoa(grp.StepIndex)
	}
	if grp.AttemptIndex > 1 {
		name += "·try" + strconv.Itoa(grp.AttemptIndex)
	}
	return name
}

// shortThreadID 把 th-7d0b4011 压成 7d0b：trace 名在每个卡片上都要显示，
// 全量线程 id 会把名字撑长；4 位足够在单个会话内区分线程。
func shortThreadID(threadID string) string {
	trimmed := strings.TrimPrefix(threadID, "th-")
	if len(trimmed) <= 4 {
		return trimmed
	}
	return trimmed[:4]
}

func generationBody(c Call, traceID, genID string, grp sessiongroup.Group) map[string]any {
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
		"metadata":     metadata(c, grp),
	}
	if c.Status == "billing_error" && c.ErrorKind != "" {
		body["statusMessage"] = c.ErrorKind
	}
	// 正文挂在 generation 上：UI 的输入/输出面板与全文检索都走这两个字段。
	if display := c.displayInput(); display != "" {
		body["input"] = display
	}
	if c.Output != "" {
		body["output"] = c.Output
	}
	return body
}

func metadata(c Call, grp sessiongroup.Group) map[string]any {
	meta := map[string]any{
		"cost_source": costSourceActual,
		"status":      c.Status,
	}
	if c.RequestID != "" {
		meta["request_id"] = c.RequestID
	}
	// 会话结构：一个会话 = 多条 trace；会话内按 thread 分线程、按 thread_step 排序、
	// 按 step_attempt 标记同一步的重试。三者配合才能在 session 视图里把
	// "一次对话的多次请求"读成一条有序时间线。
	if grp.SessionSource != "" && grp.SessionSource != sessiongroup.SourceNone {
		meta["session_source"] = grp.SessionSource
	}
	if grp.Title != "" {
		meta["session_title"] = grp.Title
	}
	if grp.ThreadID != "" {
		meta["thread_id"] = grp.ThreadID
	}
	if grp.StepIndex > 0 {
		meta["thread_step"] = grp.StepIndex
	}
	if grp.AttemptIndex > 0 {
		meta["step_attempt"] = grp.AttemptIndex
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
	// 原始请求体退到 metadata 备查，避免把 system/tools 顶到 UI 首屏。
	if c.InputUser != "" && c.Input != "" {
		meta["request_body"] = truncateBytes(c.Input, rawBodyMetaLimit)
		meta["request_body_truncated"] = len(c.Input) > rawBodyMetaLimit
	}
	if c.UserTurns > 0 {
		meta["user_prompt_turns"] = c.UserTurns
	}
	if c.PromptStatus != "" {
		meta["user_prompt_status"] = c.PromptStatus
	}
	if c.PromptProtocol != "" {
		meta["user_prompt_protocol"] = c.PromptProtocol
	}
	// 只有正文真的触顶才标 output_truncated；原始字节被截只影响解析兜底，
	// 之前两者混用导致每条 trace 都带 output_truncated=true，反而看不出真截断。
	if c.OutputTruncated {
		meta["output_truncated"] = true
	}
	if c.RawResponseTruncated {
		meta["raw_stream_truncated"] = true
	}
	if c.OutputChars > 0 {
		meta["output_chars"] = c.OutputChars
	}
	if c.FinishReason != "" {
		meta["finish_reason"] = c.FinishReason
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

// rawBodyMetaLimit 是 metadata 里原始请求体的字节上限。
const rawBodyMetaLimit = 8 * 1024

// displayInput 选择展示用 input：抽到用户文本就用它，否则退回原始请求体。
func (c Call) displayInput() string {
	if c.InputUser != "" {
		return c.InputUser
	}
	return c.Input
}

func truncateBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
