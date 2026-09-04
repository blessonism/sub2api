package service

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// MonitorBodyOverrideMode 自定义请求体处理模式。
//
//   - off     使用 adapter 默认 body（忽略 BodyOverride）
//   - merge   adapter 默认 body 与 BodyOverride 浅合并（用户优先；
//     model/messages/contents 等关键字段在 checker 黑名单内会被静默丢弃）
//   - replace 完全用 BodyOverride 作为 body；跳过 challenge 校验，
//     改成 HTTP 2xx + 响应非空即视为可用（用户负责构造 body）
const (
	MonitorBodyOverrideModeOff     = "off"
	MonitorBodyOverrideModeMerge   = "merge"
	MonitorBodyOverrideModeReplace = "replace"
)

// MonitorAPIMode 描述 OpenAI provider 的请求协议。
//
//   - chat_completions  OpenAI-compatible Chat Completions: /v1/chat/completions + messages
//   - responses         OpenAI Responses API: /v1/responses + instructions/input
//
// 非 OpenAI provider 固定使用 chat_completions 作为占位默认值，避免为每个 provider 单独扩表。
const (
	MonitorAPIModeChatCompletions = "chat_completions"
	MonitorAPIModeResponses       = "responses"
)

// ChannelMonitor 渠道监控配置（service 层模型，不直接暴露 ent 类型）。
type ChannelMonitor struct {
	ID              int64
	Name            string
	Provider        string
	APIMode         string
	Endpoint        string
	APIKey          string // 解密后的明文 API Key（仅在 service 内部使用，handler 层不应直接序列化返回）
	PrimaryModel    string
	ExtraModels     []string
	GroupName       string
	Enabled         bool
	IntervalSeconds int
	JitterSeconds   int // 每次调度 ± [0, jitter] 的随机偏移（秒），0 = 固定间隔
	LastCheckedAt   *time.Time
	CreatedBy       int64
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// 配额模式（check_mode = quota / quota_probe）：
	// 关联已有账号复用账号侧用量服务，Endpoint/APIKey 可为空（quota 模式）。
	CheckMode string // probe（默认）/ quota / quota_probe；空串按 probe 处理
	AccountID *int64 // 关联账号 ID；账号删除后被 DB 置空（监控保留并报「账号未关联」）

	// TargetKind 探测目标形态：endpoint（默认，直连外部上游）/
	// gateway_group（本站网关入口 + 分组绑定 key，探测结果即分组级可用性）。
	// 纯展示标注，不影响探测引擎行为；空串按 endpoint 处理。
	TargetKind string

	// 请求自定义快照（来自模板拷贝 or 用户手填，运行时直接读取）
	TemplateID       *int64            // 仅用于 UI 分组 + 一键应用，运行时不用
	ExtraHeaders     map[string]string // 与 adapter 默认 headers 合并，用户优先
	BodyOverrideMode string            // off / merge / replace
	BodyOverride     map[string]any    // 仅 mode != off 时使用

	// DuplicateOperationID is internal persistence metadata used to recover an
	// already committed duplicate after an ambiguous idempotency-store failure.
	// Repository implementations must keep it out of ExtraHeaders so it can
	// never be serialized to clients or forwarded to an upstream provider.
	DuplicateOperationID string

	// APIKeyDecryptFailed 表示 APIKey 字段无法解密（密钥不一致或损坏）。
	// 此时 APIKey 为空字符串，runner / RunCheck 必须跳过该监控并提示重填。
	APIKeyDecryptFailed bool
}

// ChannelMonitorListParams 列表查询过滤参数。
type ChannelMonitorListParams struct {
	Page     int
	PageSize int
	Provider string
	Enabled  *bool
	Search   string
}

// ChannelMonitorCreateParams 创建参数。
type ChannelMonitorCreateParams struct {
	Name             string
	Provider         string
	APIMode          string
	Endpoint         string
	APIKey           string
	PrimaryModel     string
	ExtraModels      []string
	GroupName        string
	Enabled          bool
	IntervalSeconds  int
	JitterSeconds    int
	CreatedBy        int64
	TemplateID       *int64
	ExtraHeaders     map[string]string
	BodyOverrideMode string
	BodyOverride     map[string]any

	// 配额模式：CheckMode 空串默认 probe；quota/quota_probe 必须关联账号。
	CheckMode string
	AccountID *int64

	// TargetKind 探测目标形态（空串按 endpoint 处理）。
	TargetKind string
}

// ChannelMonitorUpdateParams 更新参数（指针字段表示"未提供则不更新"）。
type ChannelMonitorUpdateParams struct {
	Name            *string
	Provider        *string
	APIMode         *string
	Endpoint        *string
	APIKey          *string // 空字符串表示不修改；非空字符串覆盖
	PrimaryModel    *string
	ExtraModels     *[]string
	GroupName       *string
	Enabled         *bool
	IntervalSeconds *int
	JitterSeconds   *int
	// 自定义快照字段：指针为 nil 表示不更新，非 nil 覆盖
	// TemplateID *(*int64)：用 ** 表达三态：nil=不更新；&nil=清空；&&id=设为 id。
	// 简化处理：用 ClearTemplate 显式标志 + TemplateID（普通指针）
	TemplateID       *int64
	ClearTemplate    bool // true 时无视 TemplateID，把监控的 template_id 置空
	ExtraHeaders     *map[string]string
	BodyOverrideMode *string
	BodyOverride     *map[string]any

	// TargetKind 探测目标形态；nil 表示不更新。
	TargetKind *string

	// 配额模式：CheckMode nil = 不更新；AccountID nil = 不更新，
	// 指向 0 = 清空关联（退回 probe 模式时由 CheckMode 分支兜底）。
	CheckMode *string
	AccountID *int64
}

// CheckResult 单个模型一次检测的结果。
type CheckResult struct {
	Model         string
	Status        string // operational / degraded / failed / error
	LatencyMs     *int
	PingLatencyMs *int
	Message       string
	CheckedAt     time.Time
	// ErrorCategory 探测错误归类（v2 taxonomy 类别名，仅 status=error 且
	// 可识别时有值，如 rate_or_capacity）；空串表示未归类/不适用。
	ErrorCategory string
	// Quota 配额模式附带快照（quota 模式唯一数据；quota_probe 挂在主模型行）。
	Quota *domain.MonitorQuotaSnapshot
}

// UserMonitorView 用户只读视图：监控概览（含主模型最近状态 + 7d 可用率 + 附加模型最近状态）。
type UserMonitorView struct {
	ID                   int64
	Name                 string
	Provider             string
	GroupName            string
	PrimaryModel         string
	PrimaryStatus        string
	PrimaryLatencyMs     *int
	PrimaryPingLatencyMs *int    // 主模型最近一次 ping 延迟
	Availability7d       float64 // 0-100
	ExtraModels          []ExtraModelStatus
	Timeline             []UserMonitorTimelinePoint // 主模型最近 N 个历史点（按 checked_at DESC，最新在前）
	// TargetKind 探测目标形态：gateway_group 时前端卡片展示"分组"徽标
	// （该卡片代表经网关 failover 的分组级状态，而非单一上游）。
	TargetKind string
	// PrimaryErrorCategory 主模型最近一次 error 的归类（rate_or_capacity 等），
	// 前端卡片据此把"限流/拥挤"与真故障区分展示；空串表示非 error 或未归类。
	PrimaryErrorCategory string
	// LatestQuota 主模型最近一次配额快照；channel_monitor_show_quota=false
	// 时由 handler 服务端剥离。
	LatestQuota *domain.MonitorQuotaSnapshot
}

// UserMonitorTimelinePoint 用户视图 timeline 单点数据（去除 message 以减小响应体）。
type UserMonitorTimelinePoint struct {
	Status        string    `json:"status"`
	LatencyMs     *int      `json:"latency_ms"`
	PingLatencyMs *int      `json:"ping_latency_ms"`
	CheckedAt     time.Time `json:"checked_at"`
	// ErrorCategory 错误归类（rate_or_capacity 等）；空串表示未归类。
	ErrorCategory string `json:"error_category,omitempty"`
}

// ExtraModelStatus 附加模型最近一次状态。
type ExtraModelStatus struct {
	Model     string
	Status    string
	LatencyMs *int
	// ErrorCategory 最近一次 error 的归类（rate_or_capacity 等），空串 = 未归类。
	ErrorCategory string
}

// UserMonitorDetail 用户只读视图：监控详情（含全部模型 7d/15d/30d 可用率与平均延迟）。
type UserMonitorDetail struct {
	ID        int64
	Name      string
	Provider  string
	GroupName string
	Models    []ModelDetail
}

// ModelDetail 单个模型的可用率/延迟统计。
type ModelDetail struct {
	Model           string
	LatestStatus    string
	LatestLatencyMs *int
	Availability7d  float64 // 0-100
	Availability15d float64
	Availability30d float64
	AvgLatency7dMs  *int
	// LatestErrorCategory 最近一次 error 的归类（rate_or_capacity 等），空串 = 未归类。
	LatestErrorCategory string
}

// ChannelMonitorHistoryRow 历史记录入库行（service 层向 repository 提交的数据）。
type ChannelMonitorHistoryRow struct {
	MonitorID     int64
	Model         string
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	Message       string
	CheckedAt     time.Time
	Quota         *domain.MonitorQuotaSnapshot
	// ErrorCategory 错误归类（仅 status=error 且可识别时有值）。
	ErrorCategory string
}

// ChannelMonitorHistoryEntry 历史记录查询返回行（含 ent 主键 ID）。
type ChannelMonitorHistoryEntry struct {
	ID              int64
	Model           string
	Status          string
	OverrideStatus  *string
	EffectiveStatus string
	LatencyMs       *int
	PingLatencyMs   *int
	Message         string
	CheckedAt       time.Time
	Quota           *domain.MonitorQuotaSnapshot
	// ErrorCategory 错误归类（rate_or_capacity 等）；空串表示未归类。
	// ListRecentHistoryForMonitors（timeline）与单监控 ListHistory 均填充。
	ErrorCategory string
}

// ChannelMonitorLatest 最近一次检测的简明信息（用于 UserMonitorView 聚合）。
type ChannelMonitorLatest struct {
	Model           string
	Status          string
	OverrideStatus  *string
	EffectiveStatus string
	LatencyMs       *int
	PingLatencyMs   *int
	CheckedAt       time.Time
	Quota           *domain.MonitorQuotaSnapshot
	ErrorCategory   string
}

// ChannelMonitorAvailability 单个模型在某窗口内的可用率与平均延迟（用于 UserMonitorDetail 聚合）。
type ChannelMonitorAvailability struct {
	Model             string
	WindowDays        int
	TotalChecks       int
	OperationalChecks int // operational + degraded 视为可用
	AvailabilityPct   float64
	AvgLatencyMs      *int
}

// MonitorStatusSummary 监控状态聚合（admin list 用，单次 repo 查询消除前端 N+1）。
// PrimaryStatus / PrimaryLatencyMs 描述主模型最近状态；Availability7d 是主模型 7 天可用率；
// ExtraModels 描述附加模型最近状态（用于 hover 展示）。
type MonitorStatusSummary struct {
	PrimaryStatus    string // 空字符串表示无历史
	PrimaryLatencyMs *int
	Availability7d   float64 // 0-100，无历史时为 0
	ExtraModels      []ExtraModelStatus
	LatestQuota      *domain.MonitorQuotaSnapshot // 主模型最近配额快照（配额模式）
	// PrimaryErrorCategory 主模型最近一次 error 的归类（rate_or_capacity 等），
	// 空串表示最近状态非 error 或未归类。
	PrimaryErrorCategory string
}
