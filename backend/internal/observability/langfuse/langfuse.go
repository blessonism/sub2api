// Package langfuse 提供模型调用的外部观测上报通道（Langfuse ingestion API）。
//
// 硬约束，与目标单不变量一致：
//   - 业务侧只做一次非阻塞 channel send，返回前不产生网络 I/O、不返回 error、不 panic；
//   - 队列满即丢，丢弃计数只留在内存，不写应用日志、不写 audit_logs；
//   - 配置缺失或 enabled=false 时整体空转：不建队列、不起 goroutine、零出网；
//   - 观测层只承载元数据（检索键、用量、实扣费用、状态），不承载提示词正文。
//     正文取证由 securityaudit 审查台负责，两侧不重复落正文。
package langfuse

import (
	"context"
	"sync/atomic"
	"time"
)

// Call 是一次模型调用在观测层的投影。
//
// 一条 Call 会展开成 Langfuse 上的 1 条 trace + 1 条 generation：
//   - trace 承载 userId / sessionId 检索键与请求级元数据；
//   - generation 承载模型、用量、实扣费用与状态。
type Call struct {
	// UserID 是计费用户 ID，作为 trace 的 userId 维度。
	UserID int64
	// SessionID 是客户端显式会话标识，缺失时留空（Langfuse 侧即无 session 聚合）。
	SessionID string
	// RequestID 是计费请求 ID，用于把观测记录与 usage_logs 对账。
	RequestID string

	Model    string
	Endpoint string

	// Status 取 "ok" / "unbilled" / "billing_error"，映射到 generation 的 level。
	Status    string
	ErrorKind string

	StartedAt time.Time
	Duration  time.Duration

	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheWriteTokens  int
	ImageInputTokens  int
	ImageOutputTokens int

	// Cost 是**实扣金额**（CostBreakdown.ActualCost 落库后的值），不是标价。
	// 观测层不重算价格：倍率、用户级覆盖、峰谷因子都在计费链路内部收敛，
	// 在这里按 token×单价重算必然与 usage_logs 对不上。
	Cost     float64
	Currency string

	RateMultiplier        float64
	VisibleRateMultiplier float64
	AccountRateMultiplier float64
	BillingType           string
	BillingMode           string
	ServiceTier           string
	Stream                bool

	AccountID int64
	APIKeyID  int64
	GroupID   int64

	// Input / Output 是正文层：请求提示词与响应 assistant 文本。
	// 为空表示本次调用未采集正文（采集开关关闭或未安装 recorder）。
	Input  string
	Output string
	// InputUser 是从请求体里单独抽出的用户消息文本；非空时作为展示用 input，
	// Input 退居 metadata 备查。
	InputUser string
	// FirstUserPrompt 是请求里第一条「用户真正说的话」（已跳过 <environment_context>
	// 这类客户端注入块），只用于会话归组与标题，不单独展示。
	FirstUserPrompt string
	// RequestFingerprint 是请求体短指纹：同线程内指纹相同 = 同一步的重试/并行。
	RequestFingerprint string
	// UserTurns 是用户消息条数。
	UserTurns int
	// UserPromptHistoryBytes 是这次请求里完整用户历史的字节数。
	// InputUser 只放最新那句，剩下的历史量用这个数字表达。
	UserPromptHistoryBytes int
	// PromptProtocol / PromptStatus 记录抽取结果，便于排查为什么没抽到。
	PromptProtocol string
	PromptStatus   string
	// OutputChars 是响应正文字符数（rune），用于事后判断"这段回答是不是被切了"。
	OutputChars int
	// FinishReason 是上游给的收尾原因：stop / length / tool_calls / incomplete:*。
	// 它与 OutputTruncated 配合，才能区分"模型自己截断"和"我们缓冲截断"。
	FinishReason string
	// RawRequestTruncated 标记**原始请求样本**被截断（只影响 metadata 里的
	// request_body 抽样，不代表用户消息缺失）。
	RawRequestTruncated bool
	// PromptTruncated 标记**展示用的用户消息**被截断（这才是 input 不全）。
	PromptTruncated bool
	// OutputTruncated 标记 assistant 正文被截断。
	OutputTruncated bool
	// RawResponseTruncated 标记**原始响应字节**被截断（仅影响解析兜底，不代表正文缺失）。
	RawResponseTruncated bool

	// Metadata 是可选的附加检索维度，值必须是已脱敏的短字符串。
	Metadata map[string]string
}

// TotalTokens 返回计入观测的 token 总量（不含图片桶，避免与 input/output 重复计）。
func (c Call) TotalTokens() int {
	return c.InputTokens + c.OutputTokens + c.CacheReadTokens + c.CacheWriteTokens
}

// Sink 是观测上报端。Observe 必须非阻塞且不得 panic。
type Sink interface {
	Observe(Call)
}

// nopSink 是关闭态实现：不分配、不出网。
type nopSink struct{}

func (nopSink) Observe(Call) {}

// Nop 返回关闭态 sink。
func Nop() Sink { return nopSink{} }

// sinkHolder 让 atomic.Pointer 存的是固定具体类型，
// 避免 atomic.Value 在切换不同 Sink 实现时因类型不一致 panic。
type sinkHolder struct{ sink Sink }

// defaultSink 保存进程级上报端。未调用 SetDefault 前为空，
// 此时 Observe 是纯 no-op —— 这就是"配置缺失必须 no-op"的落地点。
var defaultSink atomic.Pointer[sinkHolder]

// active 是给热路径用的快速判定，避免关闭态下构造 Call。
var active atomic.Bool

// SetDefault 安装进程级上报端。传 nil 等价于 Nop。
func SetDefault(s Sink) {
	if s == nil {
		s = Nop()
	}
	defaultSink.Store(&sinkHolder{sink: s})
	_, isNop := s.(nopSink)
	active.Store(!isNop)
}

// Active 报告当前是否装配了真实上报端。
func Active() bool { return active.Load() }

// Observe 投递一条调用记录。未装配上报端时直接返回。
func Observe(c Call) {
	if !active.Load() {
		return
	}
	h := defaultSink.Load()
	if h == nil {
		return
	}
	h.sink.Observe(c)
}

// Shutdown 停止进程级上报端并冲刷剩余批次。
// 未装配真实客户端（关闭态 / 未初始化）时立即返回。
func Shutdown(ctx context.Context) error {
	h := defaultSink.Load()
	if h == nil {
		return nil
	}
	c, ok := h.sink.(*Client)
	if !ok {
		return nil
	}
	return c.Close(ctx)
}
