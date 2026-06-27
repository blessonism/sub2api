# 执行文档：OpenAI 对话历史查看与训练数据导出 Phase 1

> 配套文档：同目录 `prd.md`、`design-conversation-history-training-export.md`、`research/tokenrouter-data-sharing-reference.md`。
> 原则：轻量结构化会话主路径；默认关闭；热路径 fail-open；不实现 raw archive；不做复杂审计、审批、多角色权限。

---

## 阶段总览

| 阶段 | 内容 | 产出 | 可独立验证 |
|---|---|---|---|
| P0 | 配置与 settings key | config 结构、默认值、运行时设置 DTO | 配置包构建 |
| P1 | DB schema + 迁移 | `conversation_sessions`、`conversation_turns` | ent 生成 + 迁移幂等 |
| P2 | capture writer 原子能力 | ResponseWriter 包装器 | writer 单测 |
| P3 | 采集 worker 池 | 独立 bounded worker | 队列满 drop 单测 |
| P4 | 采集 service + 解析器 | Decide/Submit、chat parser、session upsert | service/parser 单测 |
| P5 | OpenAI chat/completions 接入 | 路由中间件 + handler 元数据发布 | stream/non-stream 冒烟 |
| P6 | 管理 API | config、sessions、turns、exportable、messages_jsonl 导出 | handler/API 测试 |
| P7 | 前端页面 | 对话历史列表、详情、配置、导出入口 | typecheck + view 测试 |
| P8 | 清理任务与收尾验证 | retention cleanup、wire、构建测试 | 定向 go test / 前端检查 |

---

## P0：配置与 settings key

**目标**：建立轻量采集的运行时配置，不触碰 S3 raw archive 主路径。

**后端配置字段**：

在 `backend/internal/config/config.go` 的 `GatewayConfig` 下新增：

```go
ConversationCapture GatewayConversationCaptureConfig `mapstructure:"conversation_capture"`
```

建议结构：

```go
type GatewayConversationCaptureConfig struct {
    Enabled              bool    `mapstructure:"enabled"`                // 默认 false
    SamplePercent        int     `mapstructure:"sample_percent"`         // 默认 100
    CaptureChatCompletions bool  `mapstructure:"capture_chat_completions"` // 默认 true
    CaptureResponses     bool    `mapstructure:"capture_responses"`      // Phase 1 默认 false
    RawArchiveEnabled    bool    `mapstructure:"raw_archive_enabled"`    // Phase 1 默认 false，不实现 sink
    MaxTurnPayloadBytes  int     `mapstructure:"max_turn_payload_bytes"` // 默认 1048576
    PayloadPreviewChars  int     `mapstructure:"payload_preview_chars"`  // 默认 8000
    SessionWindowMinutes int     `mapstructure:"session_window_minutes"` // 默认 30
    RetentionDays        int     `mapstructure:"retention_days"`         // 默认 30
    ExportEnabled        bool    `mapstructure:"export_enabled"`         // 默认 true
    WorkerCount          int     `mapstructure:"worker_count"`           // 保守默认
    QueueSize            int     `mapstructure:"queue_size"`
    TaskTimeoutSeconds   int     `mapstructure:"task_timeout_seconds"`
}
```

运行时 settings key：

- `conversation_capture_config`
- 存储动态字段：`enabled`、`sample_percent`、`max_turn_payload_bytes`、`payload_preview_chars`、`session_window_minutes`、`retention_days`、`export_enabled`、`excluded_user_ids`、`excluded_api_key_ids`。
- config.yaml 提供默认值，settings 表覆盖。

校验规则：

- `sample_percent` 必须在 `[0,100]`。
- `max_turn_payload_bytes > 0`。
- `payload_preview_chars > 0`。
- `session_window_minutes > 0`。
- `retention_days >= 1`。
- 排除名单只接受正整数 ID。

**验证**：

- 配置默认值加载正确。
- settings 缺失时使用配置文件兜底。
- settings 非法值回落到安全默认值。

---

## P1：DB schema + 迁移

**目标**：新增结构化会话主路径，Phase 1 不新增 raw archive 表。

### conversation_sessions

建议文件：

- `backend/ent/schema/conversation_session.go`
- `backend/migrations/160_conversation_sessions.sql`

核心字段：

```text
id                   int64
session_id           string unique/index
trajectory_id        string optional
user_id              int64
api_key_id           int64
account_id           int64 optional
provider             string default "openai"
model                string
request_path         string
status               string
turn_count           int
source_request_count int
input_tokens         int64
output_tokens        int64
total_tokens         int64
actual_cost          decimal/string 按项目既有金额类型
quality_status       string default "unchecked"
exportable           bool default false
capture_status       string default "captured"
session_source       string
retention_until      time
started_at           time
ended_at             time
created_at           time
updated_at           time
```

索引：

- `session_id`
- `user_id`
- `api_key_id`
- `model`
- `quality_status`
- `exportable`
- `started_at`
- `created_at`
- `(user_id, started_at)`
- `(api_key_id, started_at)`

### conversation_turns

建议文件：

- `backend/ent/schema/conversation_turn.go`
- `backend/migrations/161_conversation_turns.sql`

核心字段：

```text
id                   int64
session_id           string
request_id           string
upstream_request_id  string optional
client_request_id    string optional
turn_index           int
provider             string default "openai"
model                string
request_path         string
request_messages     json
response_messages    json
tools                json optional
usage                json optional
meta                 json optional
input_tokens         int64
output_tokens        int64
total_tokens         int64
actual_cost          decimal/string 按项目既有金额类型
stream               bool
client_disconnect    bool
truncated            bool
quality_status       string default "unchecked"
exportable           bool default false
parse_status         string
parse_error          string optional
dedupe_hash          string
payload_preview      text
payload_compressed   bytes optional
retention_until      time
created_at           time
```

索引：

- `session_id`
- `request_id`
- `client_request_id`
- `dedupe_hash`
- `created_at`
- `(session_id, turn_index)`

约束建议：

- `session_id + turn_index` 唯一，避免同一 session 内 turn 顺序重复。
- `dedupe_hash` 不强制全局唯一，导出时用于去重；避免极端冲突阻塞采集。

**验证**：

- `go generate ./ent/...` 后 ent client 可构建。
- SQL migration 连续执行两次不报错。
- session / turn create + query 基础 repository 测试通过。

---

## P2：capture writer 原子能力

**目标**：只负责捕获客户端实际收到的响应字节，不做解析、不做 DB、不做导出。

建议文件：

- `backend/internal/handler/conversation_capture_middleware.go`

writer 约束：

- 内嵌 `gin.ResponseWriter`。
- 只重写 `Write` / `WriteString`。
- 先写真实 client，再把实际写出的字节追加到 buffer。
- 超过响应捕获上限后停止追加并标记 `truncated`，不影响客户端响应。
- 真实 `Write` 返回 error 时标记 `disconnect=true`，并原样返回 error。
- `Flush`、`Hijack`、`Status`、`Header` 通过内嵌 writer 透传。
- `Snapshot()` 返回字节拷贝，后台任务不得持有 writer 或可变 slice。

**注意**：

- `MaxTurnPayloadBytes` 约束完整 turn payload；writer 只限制响应侧内存。
- request body 与 response raw 的总量裁剪在 service Submit 同步段完成。

**单测**：

- 正常累积。
- 精确边界截断。
- 超限后客户端仍收到完整响应。
- `Write` error 标记 disconnect。
- `Flush` 透传。
- Snapshot 返回拷贝。

---

## P3：采集 worker 池

**目标**：采集与转发解耦，队列满直接 drop。

建议文件：

- `backend/internal/service/conversation_capture_worker_pool.go`

行为：

- 独立于 usage worker。
- bounded queue。
- `TrySubmit` 失败即 drop。
- 统计：`submitted`、`dropped`、`completed`、`failed`。
- panic recover。
- 限频日志不输出正文。
- `Stop()` 支持优雅停止。

**单测**：

- 正常提交执行。
- 队列满 drop。
- Stop 后拒绝提交。
- panic 任务不打崩 worker。

---

## P4：采集 service + 解析器

**目标**：形成采集决策、复制请求态、解析 chat/completions、事务写 session/turn。

建议文件：

- `backend/internal/service/conversation_capture_service.go`
- `backend/internal/service/conversation_capture_parser.go`
- `backend/internal/repository/conversation_session_repo.go`
- `backend/internal/repository/conversation_turn_repo.go`

核心类型：

```go
type ConversationCaptureDecision struct {
    Capture              bool
    MaxTurnPayloadBytes  int
    PayloadPreviewChars  int
    SessionWindowMinutes int
    RetentionDays        int
}

type ConversationCaptureMeta struct {
    RequestID         string
    UpstreamRequestID string
    ClientRequestID   string
    UserID            int64
    APIKeyID          int64
    AccountID         int64
    Model             string
    UpstreamModel     string
    RequestPath       string
    Stream            bool
    Usage             any
    ActualCost        any
    RequestBody       []byte
    ClientDisconnect  bool
}

type ConversationCaptureInput struct {
    Meta             ConversationCaptureMeta
    ResponseRaw      []byte
    ClientDisconnect bool
    Truncated        bool
}
```

`Decide(ctx, subject)`：

- 读取运行时配置。
- `enabled=false` 返回不采集。
- `capture_chat_completions=false` 返回不采集。
- 命中 `excluded_user_ids` / `excluded_api_key_ids` 返回不采集。
- 入口一次性执行采样。
- 返回上限和 session window 快照。

`Submit(decision, input)`：

- 同步复制所有 request-scoped 数据。
- 裁剪 request/response 总 payload，生成 preview。
- 计算 `dedupe_hash`。
- 提交 worker；worker 内不持有 request context。

parser：

- request：解析 `messages`。
- non-stream response：解析 assistant message。
- stream response：解析 SSE `data:` delta，聚合 assistant 文本。
- 解析失败返回 `parse_status=failed`、`parse_error`、preview。
- tool call Phase 1 可保存在 `tools` / `meta`，不要求完整训练样本转换。

session upsert：

- 显式 `X-Conversation-ID` 或 `metadata.conversation_id`：`session_source=explicit`。
- 启发式：`session_source=heuristic`，默认不可导出。
- 无法归并：`session_source=single_turn`。
- 新 turn 插入后更新 session 的 token/cost/turn_count/ended_at。

导出默认状态：

- `parse_status=success && !truncated && !client_disconnect && session_source != heuristic` 时可考虑默认 `exportable=true`。
- 更保守的选择是默认 `exportable=false`，由管理员手动标记。Phase 1 推荐默认 false。

**单测**：

- Decide：开关、采样、排除名单、非法配置。
- parser：non-stream assistant、stream delta 聚合、坏 JSON、缺 assistant。
- payload：preview 截断、payload 超限、dedupe hash 稳定。
- repository：新建 session、追加 turn、更新 session 汇总。
- fail-open：解析/DB 失败只计数和日志。

---

## P5：OpenAI chat/completions 接入

**目标**：只接入 Phase 1 范围，不触碰 responses 主线。

涉及位置：

- `backend/internal/server/routes/gateway.go`
- OpenAI chat/completions handler 所在文件
- middleware / service wire

接入规则：

- 只在 OpenAI 平台分支内包裹 capture middleware。
- 非 OpenAI fallback 不调用 capture，不替换 writer。
- 不挂载到 `responses`、`embeddings`、`images`、WebSocket。
- handler 在 usage 提交点发布 `ConversationCaptureMeta`。
- 中间件在 `c.Next()` 返回后读取 meta 和 response snapshot，有 meta 才 Submit。

成功判定：

- 复用 usage 提交点。
- 不从 HTTP status 或 writer 状态反推。
- 同一请求至多提交一次。

**单测 / 冒烟**：

- `/v1/chat/completions` OpenAI 平台命中 capture。
- `/chat/completions` OpenAI 平台命中 capture。
- 非 OpenAI 平台不命中 capture。
- responses / embeddings / images 不命中 capture。
- stream/non-stream 成功请求写 session + turn。
- 上游错误和 cyber/policy 不写 session + turn。

---

## P6：管理 API

**目标**：支撑后台查看、配置、手动标记、轻量导出。

建议接口：

```text
GET  /api/v1/admin/conversations/config
PUT  /api/v1/admin/conversations/config

GET  /api/v1/admin/conversations/sessions
GET  /api/v1/admin/conversations/sessions/:id
GET  /api/v1/admin/conversations/sessions/:id/turns
GET  /api/v1/admin/conversations/turns/:id

PUT  /api/v1/admin/conversations/sessions/:id/exportable
PUT  /api/v1/admin/conversations/turns/:id/exportable

POST /api/v1/admin/conversations/export/messages-jsonl
```

列表参数：

- `page`
- `page_size`，必须有最大上限
- `user_id`
- `api_key_id`
- `model`
- `request_id`
- `quality_status`
- `exportable`
- `started_at_from`
- `started_at_to`

导出参数：

- 与列表相同的筛选条件。
- `include_heuristic=false` 默认。
- `limit` 必须有最大上限，避免一次导出过大。

响应安全：

- 配置接口不返回正文。
- 列表接口只返回 preview 和元数据。
- turn 详情才返回完整结构化 messages 或解压后的 payload。
- 错误响应不包含正文。

**测试**：

- admin 鉴权。
- 参数校验。
- 分页排序。
- exportable 更新。
- JSONL 导出过滤规则。

---

## P7：前端页面

**目标**：后台可用，不做复杂工作台。

建议文件：

- `frontend/src/api/conversations.ts`
- 后台路由和菜单项
- `ConversationHistoryView.vue`
- `ConversationSessionDetailView.vue` 或同页抽屉/详情面板
- 配置组件可放在同页顶部或独立 tab

页面能力：

- 配置区：
  - 总开关
  - 采样率
  - payload 上限
  - preview 长度
  - 保留天数
  - 排除用户/API Key
- 列表：
  - 用户、API Key、模型、时间、request_id、exportable 筛选。
  - 分页。
  - 最近时间倒序。
- 详情：
  - turn 列表。
  - user / assistant / tool 展示。
  - usage、cost、request_id、parse_status、truncated、client_disconnect。
- 操作：
  - 标记 session / turn 可导出。
  - 小批量导出 `messages_jsonl`。

类型要求：

- 前端 API 类型与后端 JSON 字段保持 snake_case 对齐。
- 不把敏感字段重命名成容易误解的字段。
- loading / empty / error 状态齐全。

---

## P8：清理任务与收尾验证

> Review 修复入口：进入 Phase 2 前先处理 `review-phase1-fix-plan.md` 中的 P1/P2 问题，尤其是导出 `exportable` 语义和显式 session 首轮并发创建。

**Retention cleanup**：

- 新增定时或手动触发的清理逻辑，删除 `retention_until < now` 的 session / turn。
- Phase 1 如果导出为临时文件，也需要清理过期临时导出。
- 日志只记录数量，不记录正文。

**Wire 接线**：

- service provider。
- repository provider。
- handler provider。
- worker 生命周期 Stop。

**验证建议**：

1. `cd backend && go generate ./ent/...`
2. `cd backend && go test ./internal/service/... ./internal/repository/... ./internal/handler/...`
3. 后端构建。
4. 前端 typecheck / 相关 view tests。
5. 手动冒烟：
   - 开关关闭时发送 chat request，不产生数据。
   - 开关开启后 non-stream 产生 session / turn。
   - stream 产生聚合 assistant 文本。
   - 标记 exportable 后导出 `messages_jsonl`。
   - 命中排除 API Key 后不产生数据。

---

## 风险与回滚

| 风险 | 缓解 |
|---|---|
| 影响转发字节 | writer 先写客户端再捕获；单测对比响应字节 |
| 热路径阻塞 | 独立 worker + 队列满 drop |
| DB 膨胀 | payload 上限、preview、可选压缩、保留期 |
| 会话误合并 | 显式会话 ID 优先；启发式默认不导出 |
| 流式解析不完整 | Phase 1 只保证 assistant 文本；失败标记不可导出 |
| 敏感正文泄露 | 默认关闭、排除名单、日志不打印正文、保留期清理 |
| 与旧 S3 raw archive 混淆 | Phase 1 不实现 raw archive；后续如启用必须复用同一 capture 快照 |

## 显式 Out of Scope（与 PRD 对齐）

- 不实现 raw archive。
- 不实现 `responses` 结构化采集。
- 不实现后台大导出任务 `conversation_export_jobs`。
- 不实现 JSONL.zst。
- 不覆盖 Anthropic、Gemini、embeddings、images、WebSocket。
- 不做复杂审计、审批、多角色权限或用户侧授权确认。
