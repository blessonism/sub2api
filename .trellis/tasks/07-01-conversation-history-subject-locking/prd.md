# 优化对话历史主体锁定

## Goal

优化后台对话历史的主体锁定方式，让管理员不再依赖难以匹配的内部 `user_id` 和 `api_key_id`。对话历史列表、筛选、采集白名单/黑名单配置应支持通过用户邮箱、用户名、Key 名称等可读信息锁定目标；同时明确白名单语义：白名单模式下只选择用户、不选择 API Key 时，应采集该用户所有 Key 的对话。

## What I already know

* 当前对话历史由 `conversation_sessions` / `conversation_turns` 保存，session 级字段包含 `user_id`、`api_key_id`、`account_id`、`model`、`request_path`、token 与质量状态。
* 当前 `ConversationSessionFilters` 只支持 `user_id`、`api_key_id`、`model`、`request_id`、质量状态、导出状态、时间范围。
* 当前前端 `ConversationHistoryView.vue` 使用数字型 `user_id` / `api_key_id` 筛选，并且采集配置中的 subject filter 也是手动输入 ID 列表。
* 项目已有可读主体字段：`users.email`、`users.username`、`api_keys.name`。其他用量统计模块已有 join 出 `api_key_name`、用户邮箱的模式。
* 后端当前白名单判定是 `included_user_ids` 命中或 `included_api_key_ids` 命中即可采集。因此只填用户、不填 Key 时，底层判定已经是“采集该用户所有 Key”。本任务仍需要用测试/文案/UI 行为锁定该语义，避免管理员误解为空 Key 就是不采集。

## Assumptions

* 对话历史仍以现有 ID 字段作为底层持久化关系，不为本任务新增主体映射表。
* API Key 的完整密钥值不应暴露给后台列表搜索；可用 Key 名称、ID、必要时使用已有的安全前缀/掩码字段。
* 第一阶段不做对话内容全文搜索，避免隐私与索引成本扩大。

## Requirements

* 对话历史 session 列表返回用户和 Key 的可读摘要字段，例如 `user_email`、`user_name`、`api_key_name`。
* 对话历史 session 筛选支持主体查询，至少可按用户邮箱、用户名、Key 名称匹配；数字输入仍可兼容 ID。
* 前端筛选区不再以裸 ID 作为主要交互，改为更容易理解的主体搜索/选择方式，ID 只作为辅助信息展示。
* 采集配置的白名单/黑名单不应要求管理员手动拼 ID 列表，应提供可搜索选择器或等价的可读主体选择体验。
* 白名单模式语义必须明确：选择用户即包含该用户所有 Key；选择单独 Key 则只额外包含该 Key；空白名单仍保持不采集，避免误采集全站。
* 导出任务过滤应与列表过滤保持一致，避免列表能锁定而导出无法复用相同条件。
* 前端中英文文案要解释用户级白名单与 Key 级白名单的关系。

## Acceptance Criteria

* [ ] 管理员能在对话历史中通过用户邮箱/用户名或 Key 名称找到对应会话，无需预先知道 `user_id` / `api_key_id`。
* [ ] 对话历史列表显示可读主体信息，并保留 ID 作为排查辅助。
* [ ] 白名单模式下，配置只包含某个用户且不包含任何 API Key 时，该用户任意 Key 发起的请求都会被采集。
* [ ] 白名单模式下，用户列表和 Key 列表都为空时仍不采集任何主体。
* [ ] 黑名单模式下，按用户排除表示排除该用户所有 Key；按 Key 排除表示仅排除该 Key。
* [ ] 导出 job 和即时导出使用与列表一致的主体过滤条件。
* [ ] 覆盖后端白名单语义、主体查询过滤，以及前端 API 类型/关键交互测试。

## Definition of Done

* Tests added/updated for backend repository/service behavior where applicable.
* Frontend API types and view tests updated where applicable.
* Lint/typecheck or targeted checks run and results recorded.
* Downstream fork workflow context remains attached to implement/check jsonl.

## Out of Scope

* 不做全文内容搜索。
* 不引入新的跨系统 trace id。
* 不改变对话采集的保留策略、质量评估、导出文件格式。
* 不暴露完整 API Key 明文。

## Technical Notes

* `backend/internal/service/conversation_capture_models.go` 定义 `ConversationSession`、`ConversationTurnSummary` 和 `ConversationSessionFilters`。
* `backend/internal/repository/conversation_capture_repo.go` 的 `ListSessions` 当前只查询 `conversation_sessions`，需要考虑 join `users` / `api_keys` 并扩展 where。
* `backend/internal/service/conversation_capture_service.go` 的 `conversationCaptureSubjectAllowed` 当前白名单逻辑已经是用户命中或 Key 命中。
* `frontend/src/api/admin/conversations.ts` 需要同步类型字段和筛选参数。
* `frontend/src/views/admin/ConversationHistoryView.vue` 是当前对话历史配置与列表主界面。
