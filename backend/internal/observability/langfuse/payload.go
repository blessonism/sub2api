package langfuse

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/observability/sessiongroup"
)

const (
	eventTypeTraceCreate      = "trace-create"
	eventTypeGenerationCreate = "generation-create"

	// traceName 是所有观测 trace 的固定前缀，便于在 UI 上按 name 过滤。
	// 一条 trace = 一轮对话（用户提问 → 若干步骤 → 回答），不是一次上游调用。
	traceName = "sub2api.turn"

	// tsLayout 是 Langfuse ingestion 接受的时间格式（毫秒精度，UTC）。
	tsLayout = "2006-01-02T15:04:05.000Z"

	// costSourceActual 标记费用来源为业务库实扣值，供 UI/导出侧区分口径。
	costSourceActual = "actual_charge"

	levelDefault = "DEFAULT"
	levelError   = "ERROR"

	// rawBodyMetaLimit 是 metadata 里原始请求体的字节上限。
	rawBodyMetaLimit = 8 * 1024
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

// maxCapturedPerCall 是单条调用可外发的正文总量上限。
//
// Langfuse ingestion 的单请求上限是 4.5MB；一条调用现在只外发
// generation 一份正文（卡片在轮末单独发），所以这条闸门主要防的是
// 单步响应异常巨大。2MB 留足余量。
const maxCapturedPerCall = 2 << 20

// clampCaptured 是载荷侧的最后一道闸：配置写多大都不能把 ingestion 撑爆。
func clampCaptured(c Call) Call {
	const (
		outputBudget = maxCapturedPerCall / 2 // 1MB
		rawBudget    = 256 << 10              // metadata 抽样，不需要更多
	)
	// 原始请求样本只是取证抽样，不该挤占正文预算。
	if len(c.Input) > rawBudget {
		c.Input = truncateBytes(c.Input, rawBudget)
		c.RawRequestTruncated = true
	}
	if len(c.Output) > outputBudget {
		c.Output = truncateBytes(c.Output, outputBudget)
		c.OutputTruncated = true
		c.OutputChars = utf8.RuneCountInString(c.Output)
	}
	// 用户消息是卡片的 input，只需够读完整一次追问。
	if len(c.InputUser) > turnInputLimit {
		c.InputUser = tailBytes(c.InputUser, turnInputLimit)
		c.PromptTruncated = true
	}
	return c
}

// tailBytes 保留尾部并回退到 rune 边界。省略标记算在 limit 之内，
// 否则截断后的长度会比预算多出标记本身的字节数，把载荷预算算穿。
func tailBytes(s string, limit int) string {
	const marker = "…（前文已省略）\n\n"
	if len(s) <= limit {
		return s
	}
	keep := limit - len(marker)
	if keep <= 0 {
		return marker[:limit]
	}
	cut := len(s) - keep
	for cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut++
	}
	return marker + s[cut:]
}

// generationBody 是单步的观测投影：模型、用量、实扣、耗时，以及这一步自己的输出。
//
// 刻意不再带 input：同一轮里每一步的「用户最新消息」完全相同，
// 之前每步都发一份，在 Langfuse 上表现为几十条一模一样的输入，
// 既没有增量信息，又把 payload 撑大一倍。用户消息由轮次卡片统一承载。
func generationBody(c Call, traceID, genID string, grp sessiongroup.Group) map[string]any {
	end := c.StartedAt.Add(c.Duration)
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
		"usageDetails": usageDetails(c),
		"startTime":    formatTime(c.StartedAt),
		"endTime":      formatTime(end),
		"level":        level(c.Status),
		"metadata":     stepMetadata(c, grp),
	}
	if c.Status == "billing_error" && c.ErrorKind != "" {
		body["statusMessage"] = c.ErrorKind
	}
	// 这一步说了什么，只有它自己知道：卡片只保留整轮最后一条，
	// 中间过程（尤其是被后续步骤覆盖掉的解释性文字）必须挂在 generation 上。
	if c.Output != "" {
		body["output"] = c.Output
	}
	return body
}

func usageDetails(c Call) map[string]any {
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
	return usage
}

// stepMetadata 是单步 metadata：只放与这一步有关的维度。
func stepMetadata(c Call, grp sessiongroup.Group) map[string]any {
	meta := map[string]any{
		"cost_source": costSourceActual,
		"status":      c.Status,
	}
	if c.RequestID != "" {
		meta["request_id"] = c.RequestID
	}
	if c.Currency != "" {
		meta["cost_currency"] = c.Currency
	}
	// 轮内定位：thread_id + turn_index 定到哪一轮，thread_step 定到轮内第几步，
	// step_attempt 标记同一步的重试。四者配合才能在 session 视图里把
	// "一次提问的几十次调用"读成一条有序时间线。
	if grp.SessionSource != "" && grp.SessionSource != sessiongroup.SourceNone {
		meta["session_source"] = grp.SessionSource
	}
	if grp.ThreadID != "" {
		meta["thread_id"] = grp.ThreadID
	}
	if grp.TurnIndex > 0 {
		meta["turn_index"] = grp.TurnIndex
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
	if c.RawRequestTruncated {
		meta["raw_request_truncated"] = true
	}
	if c.PromptTruncated {
		meta["user_prompt_truncated"] = true
	}
	if c.UserTurns > 0 {
		meta["user_prompt_turns"] = c.UserTurns
	}
	if c.UserPromptHistoryBytes > 0 {
		meta["user_prompt_history_bytes"] = c.UserPromptHistoryBytes
	}
	if c.PromptStatus != "" {
		meta["user_prompt_status"] = c.PromptStatus
	}
	if c.PromptFallback {
		meta["user_prompt_fallback"] = true
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
	// 零值之外还要挡「零值 + Duration」这种伪时间：它不满足 IsZero，
	// 但年份是 0001，Langfuse 会算出负 latency 甚至判为非法事件。
	if t.IsZero() || t.Year() < 1970 {
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

// shortHash 是 4 字节短指纹，用于把轮次键压成可读的 trace id 片段。
func shortHash(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:4])
}

// eventBytes 估一条事件在 ingestion 请求体里占的字节数。
//
// 必须按 **JSON 转义后**的口径估：Go 的 encoder 默认把 `<` `>` `&` 编成
// `\u003c` 这类 6 字节转义，控制字符同理。按原始字节估会让「估 1MB、实发 6MB」
// 的批次直接撞上 Langfuse 的 4.5MB 上限，整批（含卡片）被 413 掉。
func eventBytes(e event) int {
	const overhead = 512
	n := overhead + len(e.ID) + len(e.Type) + len(e.Timestamp)
	n += bodyBytes(e.Body)
	return n
}

// jsonStrBytes 是字符串在 JSON 编码后的字节数（含两侧引号）。
//
// 必须覆盖 encoding/json 的全部转义：`"` `\` `\n` `\r` `\t` 编成 2 字节；
// `<` `>` `&`（HTML 安全）、`<0x20`、以及 U+2028/U+2029（JSONP 防护）编成 6 字节；
// 非法 UTF-8 每个坏字节变成一个 6 字节的 \ufffd。
func jsonStrBytes(s string) int {
	if !utf8.ValidString(s) {
		// 整串按最坏情况算：每个字节都可能被替换成 6 字节。
		return 2 + 6*len(s)
	}
	n := 2
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\' || c == '\n' || c == '\r' || c == '\t':
			n += 2
		case c < 0x20 || c == '<' || c == '>' || c == '&':
			n += 6
		case c == 0xe2 && i+2 < len(s) && s[i+1] == 0x80 && (s[i+2] == 0xa8 || s[i+2] == 0xa9):
			n += 6 // U+2028 / U+2029
			i += 2
		default:
			n++
		}
	}
	return n
}

func bodyBytes(v any) int {
	switch t := v.(type) {
	case nil:
		return 4
	case string:
		return jsonStrBytes(t)
	case map[string]any:
		n := 2
		for k, item := range t {
			n += jsonStrBytes(k) + 2 + bodyBytes(item)
		}
		return n
	case []any:
		n := 2
		for _, item := range t {
			n += bodyBytes(item) + 1
		}
		return n
	case []string:
		n := 2
		for _, item := range t {
			n += jsonStrBytes(item) + 1
		}
		return n
	default:
		return 16
	}
}

// eventsBytes 汇总一批事件的估算字节数。
func eventsBytes(evs []event) int {
	n := 0
	for _, e := range evs {
		n += eventBytes(e)
	}
	return n
}
