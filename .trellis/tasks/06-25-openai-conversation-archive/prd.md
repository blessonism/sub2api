# OpenAI 对话历史查看与训练数据导出

## Goal

为 OpenAI 网关入口新增轻量对话采集能力：把成功请求解析成可查看的 `session / turn` 结构，管理员可以按用户、API Key、模型、时间和 request_id 查看对话历史，并手动筛选导出训练数据。

本任务不再以“一请求一个 S3/R2 原始 JSON”为默认主路径。默认主路径是主库结构化会话存储；S3/R2 只用于导出产物，以及后续可选的 raw archive 排查层。

Phase 1 聚焦可落地的最小产品能力：

- 只采集 OpenAI `chat/completions`。
- 支持 stream / non-stream assistant 文本聚合。
- 管理后台可查看会话和 turn。
- 支持手动标记 `exportable`。
- 支持小批量 `messages_jsonl` 导出。
- 保持网关热路径 fail-open。

## Background

- 本仓库是 `Wei-Shaw/sub2api` 下游二开，是一个把 OpenAI/Anthropic/Gemini 协议请求转发到上游渠道的 API 网关。
- 旧 PRD 的目标是“先全量留存原始数据，后期离线清洗”，主路径为 S3/R2 原始归档 + 主库轻量索引。
- 当前真实需求已经变化为“可查看用户对话历史 + 可筛选导出训练数据”。该目标需要在线查询、筛选、分页、会话聚合和训练样本构造，不能只依赖 S3 原始文件。
- TokenRouter 的 data sharing/session/export 实现证明结构化 session 表更贴近该产品目标，但本项目由单人维护运营，不需要引入复杂审计、审批、多角色权限或 API Key notice confirm。

## Requirements

### 功能需求

- 覆盖范围：
  - Phase 1 只覆盖 OpenAI `chat/completions` HTTP 入口。
  - 支持 `/v1/chat/completions` 与 `/chat/completions`。
  - 不覆盖 OpenAI `responses`，该路径放到 Phase 2。
  - 不覆盖 Anthropic、Gemini、embeddings、images、`GET /responses` WebSocket 入站。
- 成功边界：
  - 只采集到达现有 usage 提交点的成功请求。
  - 成功判定复用 handler 中 `err == nil && result != nil && !cyberBlocked` 的口径。
  - 上游错误、failover 耗尽、policy/cyber 拒绝、流超时、失败响应不采集。
- 捕获机制：
  - 继续使用 ResponseWriter 包装器在 IO 边界捕获客户端实际收到的响应字节。
  - 捕获 writer 只在采集命中时替换 `c.Writer`。
  - `enabled=false`、采样未命中、命中排除名单时，不替换 writer。
  - writer 只重写 `Write` / `WriteString`，不改变发给客户端的字节，不影响 `Flush`。
- 结构化解析：
  - 从请求体 `messages` 和响应 assistant message / SSE delta 聚合结果构造 turn。
  - 解析成功写入 `conversation_sessions` 和 `conversation_turns`。
  - 解析失败不影响转发；记录 `parse_status=failed` 和 preview，默认不可导出。
- 会话识别：
  - 优先使用客户端显式 `X-Conversation-ID` 或请求 `metadata.conversation_id`。
  - Chat Completions 启发式归并只作为兜底，使用 `user_id + api_key_id + model + messages_prefix_hash + 时间窗口`。
  - 启发式 session 默认不进入训练导出，除非管理员显式包含。
  - 无法归并时单请求形成一个 session。
- 导出：
  - 支持 `messages_jsonl` 小批量导出。
  - 默认只导出 `exportable=true`、`parse_status=success`、`truncated=false`、`client_disconnect=false` 的 turn。
  - 同一 `dedupe_hash` 默认只导出一次。
  - 导出由管理员显式点击触发，不做自动导出。
- 轻量安全护栏：
  - 采集默认关闭。
  - 支持 `excluded_user_ids` 和 `excluded_api_key_ids`。
  - 正文有单 turn 大小上限，超限后只存 preview 或压缩 payload，并默认不可导出。
  - 支持 `retention_days`，用于清理过期会话、turn 和导出产物。
  - 日志、错误信息、配置接口不输出请求或响应正文。

### 管理需求

- 提供管理员配置入口，可动态调整：
  - `enabled`
  - `sample_percent`
  - `max_turn_payload_bytes`
  - `payload_preview_chars`
  - `session_window_minutes`
  - `retention_days`
  - `excluded_user_ids`
  - `excluded_api_key_ids`
  - `export_enabled`
- 配置持久化到 `settings` 表，配置文件只提供默认兜底。
- 管理后台新增“对话历史”页面：
  - 列表页按用户、API Key、模型、时间、request_id、质量状态、可导出状态筛选。
  - 列表分页，默认按最近时间倒序。
  - 详情页按 turn 展示 user / assistant / tool 内容、usage、cost、request_id、截断状态。
  - 支持手动标记 session / turn 是否可导出。
  - 支持小批量导出 `messages_jsonl`。

### 数据需求

- 主库新增 `conversation_sessions`：
  - 用于会话级索引、列表、筛选、统计和导出选择。
  - 核心字段：`session_id, user_id, api_key_id, account_id, provider, model, request_path, status, turn_count, source_request_count, input_tokens, output_tokens, total_tokens, actual_cost, quality_status, exportable, capture_status, session_source, retention_until, started_at, ended_at, created_at, updated_at`。
  - 核心索引：`session_id, user_id, api_key_id, model, quality_status, exportable, started_at, created_at, (user_id, started_at), (api_key_id, started_at)`。
- 主库新增 `conversation_turns`：
  - 用于会话详情展示和训练样本构造。
  - 核心字段：`session_id, request_id, upstream_request_id, client_request_id, turn_index, provider, model, request_path, request_messages, response_messages, tools, usage, meta, input_tokens, output_tokens, total_tokens, actual_cost, stream, client_disconnect, truncated, quality_status, exportable, parse_status, parse_error, dedupe_hash, payload_preview, payload_compressed, retention_until, created_at`。
  - 核心索引：`session_id, request_id, client_request_id, dedupe_hash, created_at, (session_id, turn_index)`。
- Phase 1 不要求新增 `conversation_export_jobs`。小批量导出可同步生成下载响应或短期临时文件。
- `conversation_export_jobs` 放到 Phase 2，用于后台大任务、JSONL.zst、S3/R2 导出产物和下载票据。
- raw archive：
  - Phase 1 不实现。
  - 旧方案中的 `conversation_archives` 如已存在或后续需要，可降级为可选 raw archive 索引表。
  - raw archive 不能单独做第二套采样和捕获，必须复用结构化采集的同一份快照。

### 默认配置

- `enabled=false`
- `sample_percent=100`
- `capture_chat_completions=true`
- `capture_responses=false`
- `raw_archive_enabled=false`
- `max_turn_payload_bytes=1048576`
- `payload_preview_chars=8000`
- `session_window_minutes=30`
- `retention_days=30`
- `export_enabled=true`
- worker 参数使用保守默认值，队列满直接 drop。

## Non-Goals / Out of Scope

- Phase 1 不采集 OpenAI `responses`。
- Phase 1 不做 raw archive，不写一请求一个 S3/R2 原始 JSON。
- Phase 1 不做 `conversation_export_jobs` 后台导出任务。
- Phase 1 不做 JSONL.zst。
- Phase 1 不做复杂自动质量评分。
- Phase 1 不做复杂审计、审批、多角色权限或用户侧授权确认。
- 不覆盖 Anthropic、Gemini、embeddings、images、`GET /responses` WebSocket 入站。
- 不改动现有转发判定逻辑、计费逻辑、生产配置和密钥。

## Acceptance Criteria

- [ ] 采集开关关闭时，不替换 `c.Writer`，不产生 session / turn。
- [ ] 命中排除用户或 API Key 时，不替换 `c.Writer`，不产生 session / turn。
- [ ] 采样率在请求入口一次性判定，同一请求生命周期内不二次判定。
- [ ] OpenAI `chat/completions` non-stream 成功请求可写入 `conversation_sessions` 和 `conversation_turns`。
- [ ] OpenAI `chat/completions` stream 成功请求可聚合 assistant 文本并写入 turn。
- [ ] 上游错误、failover 耗尽、cyber/policy 拒绝、流超时等失败路径不写入 session / turn。
- [ ] writer 捕获不改变客户端收到的响应字节，流式 `Flush` 行为不变。
- [ ] client disconnect 的 turn 标记 `client_disconnect=true`，默认不可导出。
- [ ] 超过 `max_turn_payload_bytes` 的 turn 标记 `truncated=true`，默认不可导出。
- [ ] 解析失败的 turn 保留 preview 和错误摘要，标记 `parse_status=failed`，默认不可导出。
- [ ] 显式 `X-Conversation-ID` 或 `metadata.conversation_id` 可把多次请求归并到同一 session。
- [ ] 无显式会话 ID 时，Chat Completions 启发式兜底不会影响转发，且默认不进入训练导出。
- [ ] 管理员可分页查看对话 session 列表，并按用户、API Key、模型、时间筛选。
- [ ] 管理员可查看单个 session 的 turn 列表和 turn 详情。
- [ ] 管理员可手动切换 session / turn 的 `exportable` 状态。
- [ ] 管理员可导出 `messages_jsonl`，默认只包含解析成功、未截断、未断连、可导出的 turn。
- [ ] 日志、错误信息和管理配置接口不输出请求或响应正文。
- [ ] `retention_days` 清理任务可删除过期 session / turn。
- [ ] 后端定向单测通过；涉及前端页面和 API 类型的测试通过。

## Technical Approach

### 采集入口

- 新增 Conversation Capture 中间件，挂载在 OpenAI `chat/completions` 路由的 OpenAI 平台分支内。
- 中间件入口调用 capture service `Decide(ctx)`，得到本次请求快照：
  - 是否启用
  - 是否采样命中
  - payload 上限
  - preview 长度
  - 排除名单
- 未命中时直接 `c.Next()`，不替换 writer。
- 命中时用 capture writer 包装 `c.Writer`，`c.Next()` 返回后从 handler 发布的 `ConversationCaptureMeta` 判断是否成功并提交。

### 元数据发布

- handler 仍只在现有 usage 提交点发布采集元数据。
- 元数据包括：`request_id, upstream_request_id, client_request_id, user_id, api_key_id, account_id, model, upstream_model, request_path, stream, usage, actual_cost, request_body, client_disconnect`。
- 中间件请求结束后读取元数据和响应快照，提交到采集 service。
- worker task 不持有 `gin.Context`、`http.ResponseWriter`、request context 或可变 slice。

### 结构化解析

- worker 解析 request `messages`。
- non-stream 响应解析 assistant message。
- stream 响应解析 SSE delta 并聚合 assistant 文本。
- tool call 可先保存到 `tools` / `meta`，Phase 1 不要求完整训练格式展开。
- 解析失败写入失败状态，不影响转发。

### 存储策略

- session 表保存列表和筛选字段。
- turn 表保存结构化消息、preview、状态、token/cost 和导出控制字段。
- 列表查询不读取大正文。
- 大 payload 超限时保存 preview，完整 payload 可选压缩到 `payload_compressed`。
- Phase 1 不写 S3 raw archive。

### 导出策略

- Phase 1 支持小批量 `messages_jsonl`。
- 导出从 turn 表读取已解析结构化 messages。
- 默认过滤 `exportable=false`、`parse_status!=success`、`truncated=true`、`client_disconnect=true`、`session_source=heuristic` 的数据。
- 同一 `dedupe_hash` 默认只导出一次。

## Risks & Mitigations

- **主库膨胀**：限制单 turn payload，列表不读大字段，提供保留期清理。
- **热路径性能**：开关关、采样未命中、命中排除名单时不替换 writer；采集进入独立 worker，队列满 drop。
- **流式解析复杂**：Phase 1 只保证 assistant 文本聚合，tool call 和 Responses 结构化放到后续阶段。
- **会话归并不准**：优先显式会话 ID；启发式 session 默认不进入训练导出。
- **敏感正文泄露**：日志不打印正文；采集默认关闭；支持排除名单；默认保留期清理。

## Definition of Done

- `prd.md`、`execute.md`、`design-conversation-history-training-export.md` 口径一致。
- 后端完成配置、capture writer、采集 service、worker、session/turn repository、迁移、管理 API。
- 前端完成对话历史列表、详情、配置、导出入口和 API 类型。
- 定向单测覆盖采样、排除名单、writer 捕获、stream/non-stream 解析、失败不采集、导出过滤。
- 不提交生产密钥、真实数据、数据库备份或运行时文件。
