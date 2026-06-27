# TokenRouter 数据采集实现参考

## 来源

- 仓库：`https://github.com/TokenFlux/TokenRouter.git`
- 本地只读 clone：`/tmp/tokenrouter-reference`
- 调研 commit：`ffc7a71 chore: sync VERSION to 0.1.212 [skip ci]`
- 调研日期：2026-06-27

## 相关实现入口

- `backend/ent/schema/data_share_session.go`
  - `data_share_sessions` 是完整 session 表，存结构化 `messages/tools/usage/meta/session_json`，并支持压缩 payload。
- `backend/internal/service/data_sharing_capture.go`
  - 提供 `CaptureOpenAIRequestAsync` / `CaptureClaudeRequestAsync`，只在分组启用数据共享时提交异步采集。
  - 任务落到独立 worker，再进入缓冲或落库，业务路径 fail-open。
- `backend/internal/service/data_sharing_capture_worker_pool.go`
  - 独立采集 worker 池，非阻塞提交，队列满直接 drop，并保留运行计数和限频日志。
- `backend/internal/service/data_sharing_builder.go`
  - 将请求体、响应体、usage、session id、request path、user agent 等转成结构化 session。
  - OpenAI Responses 和 Messages 走 raw/incremental 模式，避免在热路径做完整质量评估。
- `backend/internal/service/data_sharing_payload.go`
  - 标准化消息、工具、usage、meta，支持后续导出训练数据。
- `backend/internal/service/data_sharing_settings.go`
  - 运行时配置通过 `settings` 表覆盖配置文件默认值，并可立即更新 worker / buffer 参数。
- `backend/internal/handler/openai_gateway_handler.go`
  - WebSocket turn 完成后在 usage 记录附近生成 capture input，与 usage 成功口径靠近。

## 可复用设计点

1. 采集链路必须和计费/转发链路解耦：提交到独立 worker，满队列 drop，失败只计数和限频日志，不影响客户端。
2. 进入 worker 前复制请求体、响应体和必要元数据，避免后台任务持有 `gin.Context`、`http.ResponseWriter` 或可变 slice。
3. 采集运行时配置应有配置文件默认值，也应允许从 `settings` 表动态覆盖。
4. 日志只记录 request id、模型、账号、队列统计等轻量元信息，不记录正文。
5. 请求路径、模型、usage、用户/API key/account 等元数据应在提交点一次性快照，避免后台再回读请求态。

## 不建议照搬的部分

1. TokenRouter 的 `data_share_sessions` 面向更完整的数据共享产品，本任务只借鉴结构化 session/turn 主路径与 fail-open worker；Phase 1 不引入复杂质量评估、审批、多角色权限或后台大导出任务。
2. TokenRouter 以分组 `data_sharing_enabled` 和 API key notice confirm 作为采集准入；本任务需求是管理员全局动态开关 + 采样率，不应改 group/api_key schema。
3. TokenRouter 将较完整 payload 存在主库；本任务的 Phase 1 允许保存结构化 messages、usage、meta 和受限 preview，但必须通过单 turn 大小上限、truncated 标记和 retention_days 控制主库膨胀。
4. TokenRouter 针对 WS turn 和 Responses incremental 做较重的 session 合并；本任务当前不覆盖 `GET /responses` WebSocket 入站，HTTP 路径用 ResponseWriter IO 边界捕获更稳。

## 对本任务的适配结论

- 保持当前 PRD 的 writer 包装方案：捕获客户端实际收到的下游响应字节，比在 OpenAI service 内保存 `ResponseBody` 更能覆盖 `c.JSON`、SSE、SSE 聚合、fallback 等多条写出路径。
- 借鉴 TokenRouter 的 fail-open worker 和元数据快照：`ArchiveMeta` 只在 usage 成功点发布，中间件请求结束后提交归档任务。
- 借鉴运行时配置覆盖模式：`config.yaml` 提供默认值，`settings` 表中的 `conversation_capture_config` 优先。
- Phase 1 主路径是结构化 `conversation_sessions` / `conversation_turns`，从 `request.messages` 与 assistant response / SSE delta 构造 turn；不做一请求一个 S3 JSON 的 raw archive。

## 实现提醒

- 当前工作区分支为 `feature/group-visible-rate-multiplier`，任务要求分支为 `feature/openai-conversation-archive`，且存在多处未提交改动。2026-06-27 用户明确要求“先不切分支”，本轮按用户指令在当前分支继续修复；后续提交前仍需特别核对任务改动边界，避免混合提交。
- 本任务 `implement.jsonl` / `check.jsonl` 已包含 `downstream-fork-workflow.md`，后续实现必须继续遵守下游二开分支边界。
