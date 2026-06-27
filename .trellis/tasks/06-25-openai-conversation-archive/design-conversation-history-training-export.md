# OpenAI 对话历史查看与训练数据导出轻量方案

## 背景与结论

当前 `06-25-openai-conversation-archive` PRD 的主方案是：成功请求按采样率写入 S3/R2，一次请求一个原始 JSON，主库只保存轻量索引。该方案适合审计底稿和后续离线清洗，但不适合作为“查看用户对话历史”和“导出训练数据”的最终产品形态。

新的产品目标应调整为一个适合个人维护运营的轻量能力：

- 管理员可以按用户、API Key、模型、时间、请求 ID 查看用户对话历史。
- 对话按 session / turn 组织，而不是按单次 request 文件组织。
- 管理员可以筛选、手动标记、导出训练数据。
- 导出数据支持 JSONL / JSONL.zst，产物可上传到 S3/R2。
- 网关热路径继续 fail-open，不能因为采集、解析、导出影响请求转发。
- 不建设企业级审计系统，只保留个人运营所需的安全护栏：默认关闭、排除名单、正文大小上限、保留期、日志不打印正文。

因此推荐采用：

> 数据库结构化会话存储作为主路径，S3/R2 作为导出产物存储和可选原始排查层。

当前一请求一 S3 JSON 的实现不应继续扩展成查看系统。它可以保留为排查解析问题时才开启的 raw archive，但默认数据产品应围绕结构化 DB 表建设。不要为了单人运营场景引入复杂的多级授权、下载审计、策略版本和撤销链路。

## 参考项目对照

TokenRouter 的数据共享实现提供了更接近目标的参考：

- `data_share_sessions` 存结构化 session。
- 表内包含 `session_id`、`trajectory_id`、`provider`、`model`、`messages`、`tools`、`usage`、`meta`、`session_json`、`quality_status`、`exportable`。
- 支持用户侧和管理员侧查看数据共享 session。
- 支持筛选、单条导出、批量导出、预生成导出 artifact。
- 支持 JSON / JSONL / zstd JSONL。
- 导出 artifact 可以上传到独立 S3/R2。
- 采集使用独立 worker，队列满 drop，业务路径 fail-open。

本项目不建议完全照搬 TokenRouter：

- TokenRouter 将较完整 payload 存在主库 JSONB / 压缩字段中；本项目需要控制主库膨胀。
- TokenRouter 使用 group 级数据共享和 API key notice confirm；本项目由单人维护运营，更适合管理员全局策略、采样率、排除名单和手动导出确认。
- 本项目已有 OpenAI gateway 成功提交点、ResponseWriter 捕获、usage request_id 对齐等实现基础，可以复用这些热路径安全设计。

## 目标架构

本节描述完整目标架构。Phase 1 只落地 OpenAI `chat/completions`、结构化会话查看和小批量 `messages_jsonl` 导出；`responses` 结构化解析、`conversation_export_jobs` 后台导出任务、JSONL.zst 和 raw archive sink 均放到后续阶段。

### 分层

1. 采集层
   - 最终在 OpenAI `chat/completions` 与 `responses` 成功请求到达 usage 提交点后发布采集快照；Phase 1 只接入 `chat/completions`。
   - 继续使用 ResponseWriter 捕获客户端实际收到的响应字节，覆盖流式、SSE、fallback、聚合 JSON。
   - 只复制必要数据，不持有 `gin.Context`、`http.ResponseWriter`、request context 或可变 slice。
   - 采集 worker fail-open，满队列 drop，只记录计数和限频日志。

2. 结构化层
   - worker 解析 request/response，写入 `conversation_sessions` 和 `conversation_turns`。
   - DB 保存可查询、可筛选、可展示、可导出的结构化数据。
   - 大字段采用 JSONB 或压缩 payload，具体取舍见“正文存储策略”。

3. 查看层
   - 管理后台提供对话历史列表、会话详情、turn 详情。
   - 支持按用户、API Key、模型、时间、request_id、质量状态筛选。
   - 支持手动标记是否可导出。

4. 导出层
   - 从结构化 DB 读取筛选结果生成 JSONL / JSONL.zst。
   - 后台大导出任务写入 `conversation_export_jobs`；Phase 1 可先做小批量同步 `messages_jsonl` 导出。
   - 导出文件写本地临时目录或 S3/R2；推荐 S3/R2 作为长期产物存储。
   - 导出前必须由管理员明确点击确认；默认只导出 `exportable=true` 且未截断、解析成功的数据。

5. 原始排查层
   - 一请求一 S3 JSON 降级为可选高级开关：`raw_archive_enabled`。
   - 默认关闭，不作为训练数据查看主路径。
   - 仅用于排查采集解析 bug、保留原始下游响应底稿、复盘特殊请求。

6. 轻量安全护栏
   - 采集默认关闭，只能由管理员开启。
   - 支持排除用户 ID / API Key ID，避免采集指定客户或特殊 key。
   - 正文有单 turn 大小上限，超限只存 preview 或压缩 payload，并标记不可默认导出。
   - 支持简单保留期 `retention_days`，定期删除过期会话、turn 和导出产物。
   - 日志、错误信息、配置接口不输出对话正文。

## 数据模型

### conversation_sessions

用途：会话级索引、列表、筛选、统计和导出选择。

建议字段：

- `id`
- `session_id`
- `trajectory_id`
- `user_id`
- `api_key_id`
- `account_id`
- `provider`
- `model`
- `request_path`
- `status`
- `turn_count`
- `source_request_count`
- `input_tokens`
- `output_tokens`
- `total_tokens`
- `actual_cost`
- `quality_status`
- `quality_errors`
- `exportable`
- `capture_status`
- `retention_until`
- `started_at`
- `ended_at`
- `created_at`
- `updated_at`

索引：

- `session_id`
- `trajectory_id`
- `user_id`
- `api_key_id`
- `account_id`
- `provider`
- `model`
- `request_path`
- `quality_status`
- `exportable`
- `started_at`
- `created_at`
- `(user_id, started_at)`
- `(api_key_id, started_at)`

### conversation_turns

用途：会话详情展示和训练样本构造。

建议字段：

- `id`
- `session_id`
- `request_id`
- `upstream_request_id`
- `client_request_id`
- `turn_index`
- `provider`
- `model`
- `request_path`
- `request_messages`
- `response_messages`
- `tools`
- `usage`
- `meta`
- `input_tokens`
- `output_tokens`
- `total_tokens`
- `actual_cost`
- `stream`
- `client_disconnect`
- `truncated`
- `quality_status`
- `exportable`
- `parse_status`
- `parse_error`
- `dedupe_hash`
- `raw_archive_key`
- `payload_preview`
- `payload_compressed`
- `retention_until`
- `created_at`

索引：

- `session_id`
- `request_id`
- `client_request_id`
- `dedupe_hash`
- `created_at`
- `(session_id, turn_index)`

说明：

- `request_messages`、`response_messages`、`tools`、`usage`、`meta` 优先保存为结构化 JSONB，用于详情页和导出。
- `payload_preview` 保存有限长度正文预览，列表和详情首屏优先读取 preview。
- `payload_compressed` 是可选增强，用于保存较大的完整结构化 payload，避免无限制 JSONB 膨胀。
- `dedupe_hash` 用于同一请求重试、双写或手工补采时去重。
- `parse_status` 建议取值：`success`、`partial`、`failed`。默认只有 `success` 且未截断的数据可导出。
- `capture_status` 建议取值：`captured`、`dropped`、`truncated`、`parse_failed`，用于后台快速识别数据是否完整。

### conversation_export_jobs

用途：后台导出任务与导出产物管理。

建议字段：

- `id`
- `status`
- `filters`
- `format`
- `encoding`
- `session_count`
- `turn_count`
- `file_size`
- `s3_key`
- `download_url_expires_at`
- `expires_at`
- `error_message`
- `created_by`
- `created_at`
- `started_at`
- `completed_at`

索引：

- `status`
- `created_by`
- `created_at`

## 正文存储策略

推荐默认：

- DB 存结构化 `request_messages`、`response_messages`、`tools`、`usage`、`meta`。
- 对单条 turn 做大小上限，超过上限则：
  - DB 存截断 preview。
  - 如启用压缩 payload，则把完整结构化 payload zstd 压缩后存 `payload_compressed`。
  - 如启用 raw archive，则完整 raw payload 可选写入 S3/R2。
  - turn 标记 `truncated=true`，默认 `exportable=false`。
- 列表页和筛选查询不读取大正文，只读普通列、preview、token/cost 和状态字段。
- `retention_days` 到期后清理 session、turn、压缩 payload 和导出产物。

可选增强：

- 引入 `payload_compressed` bytea 存完整结构化 payload 的 zstd 压缩版本。
- 常用筛选字段和 preview 保持普通列 / JSONB。
- 大正文只在详情页按需解压读取。

不推荐：

- 默认把每次请求完整 raw JSON 存 S3 后再依赖 S3 人工查看。
- 默认把所有原始响应正文无限制塞入主库普通 JSONB。
- 为单人运营场景实现复杂下载审计、审批流或多角色权限矩阵。

## 会话识别规则

优先级：

1. OpenAI Responses
   - 当前响应的 `response.id` 更适合作为 turn 级标识，不直接等同于 session。
   - 优先用 `previous_response_id` 查找上一轮所属 session；找到则把当前 turn 追加到同一 session。
   - 找不到上一轮时创建新 session，并标记 `session_source=responses_orphan`，后续允许人工或离线任务合并。
   - 建议在 turn 表保存 `response_id`、`previous_response_id`，并对 `response_id` 建唯一索引。

2. 客户端显式传入
   - 支持 `X-Conversation-ID`。
   - 支持请求 `metadata.conversation_id`。
   - 这是最稳定的接入方式，建议在文档中推荐内部客户端接入。
   - 显式会话 ID 的 `session_source=explicit`，默认允许进入训练导出候选。

3. Chat Completions 启发式兜底
   - 使用 `user_id + api_key_id + model + messages_prefix_hash + 时间窗口`。
   - 时间窗口建议默认 30 分钟，可配置。
   - 该规则可能误合并或误拆分，只作为兜底，默认 `session_source=heuristic`。
   - 训练导出默认排除 heuristic session，除非管理员在筛选条件中显式包含。

4. 无法归并
   - 单请求形成一个 session。
   - `session_source=single_turn`。
   - 后续可通过离线任务或人工标记合并。

## 采集流程

成功请求到达 usage 提交点后：

1. handler 生成 `ConversationCaptureMeta`。
   - `request_id`
   - `upstream_request_id`
   - `client_request_id`
   - `user_id`
   - `api_key_id`
   - `account_id`
   - `model`
   - `upstream_model`
   - `request_path`
   - `stream`
   - `usage`
   - `actual_cost`
   - `request_body`

2. ResponseWriter 中间件提供响应快照。
   - `response_raw`
   - `client_disconnect`
   - `truncated`

3. 采集服务 `Submit(decision, input)` 同步复制数据。

4. worker 解析协议。
   - `chat/completions`：从 request `messages` 和 response assistant message / SSE delta 聚合结果构造 turn。
   - `responses`：从 request `input` 和 response output items 构造 turn。
   - 解析失败不丢弃记录：保存 preview、raw archive key（如有）和 `parse_status=failed`，默认不可导出。
   - client disconnect 或 truncated 的 turn 默认不可导出，除非管理员手动改为可导出。

5. repository 在事务中 upsert session 并 insert turn。

6. 任意失败只记录计数和日志，不影响 gateway 响应。

采集决策：

- `enabled=false` 时完全不替换 writer。
- `sample_percent` 在请求入口一次性判定，后续复用快照。
- 命中 `excluded_user_ids` 或 `excluded_api_key_ids` 时不采集。
- raw archive 与结构化采集共用同一份 request/response 快照，不能各自重新捕获或二次采样。

## API 与前端

### 管理 API

建议接口：

- `GET /api/v1/admin/conversations/sessions`
- `GET /api/v1/admin/conversations/sessions/:id`
- `GET /api/v1/admin/conversations/sessions/:id/turns`
- `PUT /api/v1/admin/conversations/sessions/:id/exportable`
- `PUT /api/v1/admin/conversations/turns/:id/exportable`
- `POST /api/v1/admin/conversations/export-jobs`
- `GET /api/v1/admin/conversations/export-jobs`
- `GET /api/v1/admin/conversations/export-jobs/:id`
- `POST /api/v1/admin/conversations/export-jobs/:id/download-ticket`
- `DELETE /api/v1/admin/conversations/export-jobs/:id`
- `GET /api/v1/admin/conversations/config`
- `PUT /api/v1/admin/conversations/config`

### 前端页面

新增后台页面：`对话历史`。

列表页：

- 筛选：用户、API Key、模型、时间、request_id、quality_status、exportable。
- 列：用户、API Key、模型、turn 数、token、cost、质量状态、可导出、最近时间。
- 操作：查看详情、单条导出、批量导出、标记可导出。
- 列表必须分页，默认按最近时间倒序。
- 默认隐藏正文，只展示 preview 和元数据，详情页再加载完整 turn。

详情页：

- 按 turn 展示 user / assistant / tool。
- 展示 usage、cost、request_id、client_request_id、client_disconnect、truncated。
- 提供原始 meta 查看。
- 支持复制训练样本 JSON。

导出任务页：

- 展示任务状态、筛选条件、session 数、turn 数、文件大小。
- 支持下载、删除、上传 S3/R2、复制远端 URL。
- 下载链接使用短期 ticket，不长期暴露公开 URL。
- 删除导出任务时同步删除本地临时文件或 S3/R2 产物。

## 导出格式

### messages_jsonl

用于通用 SFT。

```json
{"messages":[{"role":"user","content":"..."},{"role":"assistant","content":"..."}],"metadata":{"session_id":"...","user_id":1,"model":"..."}}
```

默认导出条件：

- `exportable=true`
- `parse_status=success`
- `truncated=false`
- `client_disconnect=false`
- 不包含 `session_source=heuristic`，除非管理员显式勾选包含

### raw_turn_jsonl

用于二次清洗和排查。

```json
{"session_id":"...","request_id":"...","request_messages":[...],"response_messages":[...],"tools":[...],"usage":{},"meta":{}}
```

### session_json

用于完整会话导出。

```json
{"session_id":"...","turns":[{"request_messages":[...],"response_messages":[...]}],"usage":{},"meta":{}}
```

导出任务应保存 filter snapshot，确保同一个任务的结果可复现。导出前不做复杂自动质量评分，但需要做基础去重：同一 `dedupe_hash` 默认只导出一次。

## 配置

建议配置项：

- `enabled`
- `sample_percent`
- `capture_chat_completions`
- `capture_responses`
- `raw_archive_enabled`
- `max_turn_payload_bytes`
- `payload_preview_chars`
- `session_window_minutes`
- `retention_days`
- `export_enabled`
- `export_s3_enabled`
- `export_s3_prefix`
- `excluded_user_ids`
- `excluded_api_key_ids`
- worker 参数：
  - `worker_count`
  - `queue_size`
  - `task_timeout_seconds`

动态配置：

- `enabled`
- `sample_percent`
- `raw_archive_enabled`
- `max_turn_payload_bytes`
- `payload_preview_chars`
- `session_window_minutes`
- `retention_days`
- `export_enabled`
- `export_s3_enabled`
- `export_s3_prefix`
- `excluded_user_ids`
- `excluded_api_key_ids`

启动配置：

- worker 并发、队列、任务超时。

推荐默认值：

- `enabled=false`
- `sample_percent=100`
- `capture_chat_completions=true`
- `capture_responses=false`（第二阶段再完整支持）
- `raw_archive_enabled=false`
- `max_turn_payload_bytes=1MB`
- `payload_preview_chars=8000`
- `session_window_minutes=30`
- `retention_days=30`
- `export_enabled=true`
- `export_s3_enabled=true`

## 与当前 S3 原始归档方案的关系

当前 S3 原始归档方案调整为：

- 不作为默认主路径。
- 不作为后台查看和训练导出的数据源。
- 保留为可选原始排查层。
- 配置名建议改为 `raw_archive_enabled`，避免和结构化会话采集混淆。
- 原 `conversation_archives` 可保留为 raw archive 索引表。
- 新增结构化表作为主要查询和导出来源。
- raw archive 不单独做第二套采样和捕获；如果开启，必须复用结构化采集的同一份快照。
- 如果现有代码已实现一请求一 S3 JSON，应降级为 raw sink，并由同一个采集服务统一调度，避免重复 worker、重复复制响应体和双写口径漂移。

## 分阶段落地

### Phase 1：轻量结构化会话 MVP

- 建表：`conversation_sessions`、`conversation_turns`。
- 采集 OpenAI `chat/completions`。
- 支持 stream / non-stream 的 assistant 输出解析。
- 后台按用户查看会话和 turn。
- 支持手动标记 `exportable`。
- 支持 `messages_jsonl` 小批量直接导出。
- 支持总开关、采样率、正文大小上限、保留期、排除用户/API Key。
- 保持 fail-open。
- 不做 raw archive。
- 不做复杂审计、审批、多角色权限。

### Phase 2：Responses 与导出任务

- 完善 OpenAI `responses` item/tool 解析。
- 增加 `conversation_export_jobs`。
- 支持 JSONL.zst。
- 支持导出任务后台生成和下载票据。
- 支持导出产物上传 S3/R2。
- 支持导出 filter snapshot 和 dedupe_hash 去重。

### Phase 3：质量与可选增强

- 支持 `quality_status`、`quality_errors`、`exportable`。
- 支持简单敏感字段脱敏规则。
- 支持重复上下文去重和会话合并优化。
- 如确实需要排查解析问题，再启用 raw archive sink。

## 风险与处理

- 主库膨胀
  - 限制 turn payload 大小。
  - 大 payload 压缩或转 raw archive。
  - 增加保留策略。

- 会话归并不准
  - 优先支持显式 `X-Conversation-ID`。
  - `chat/completions` 启发式只作为兜底。

- 流式解析复杂
  - 第一阶段只保证 assistant 文本聚合。
  - tool call / responses item 在第二阶段完善。

- 数据合规
  - 本项目按单人运营场景处理，不建设企业级审计系统。
  - 采集默认关闭。
  - 支持用户/API Key 排除名单。
  - 导出需管理员显式点击确认。
  - 默认只导出解析成功、未截断、非启发式归并的数据。
  - 日志不记录正文。
  - 通过 `retention_days` 清理过期数据。

## 审查问题

1. 是否接受 DB 存结构化对话正文，而不是仅存 S3 原始文件？
2. 是否接受 raw archive 默认不做，只预留 `raw_archive_enabled` 作为以后排查解析问题的可选 sink？
3. 是否要求用户侧也能查看自己的数据共享记录，还是仅管理员查看？
4. 是否接受不做复杂授权确认，只做全局开关 + 用户/API Key 排除名单？
5. 导出产物是否复用备份桶，还是使用独立 S3/R2 配置？
6. 第一阶段是否只做 `chat/completions`，把 `responses` 完整结构化放到第二阶段？
7. 是否强制推荐客户端传 `X-Conversation-ID`？
8. 默认保留期是否使用 30 天，还是按运营需要改成 7 / 90 天？
