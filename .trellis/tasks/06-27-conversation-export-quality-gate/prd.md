# 对话捕捉导出质量安全门

## Goal

为 OpenAI 对话捕捉新增第一版保守质量门：采集入库时自动判断 `quality_status`、`quality_errors`、`exportable`，导出时再做 service 层二次校验，确保训练 JSONL 不包含解析失败、截断、断连、启发式归并或结构明显不完整的数据。

## Requirements

- 后端新增可复用质量评估器，输出 `clean`、`needs_review`、`rejected`。
- 固定质量错误码：`parse_failed`、`truncated_payload`、`client_disconnect`、`missing_request_messages`、`missing_response_messages`、`heuristic_session`、`incomplete_tool_call_chain`、`orphan_tool_result`、`empty_assistant_output`。
- 采集入库时不再默认 `unchecked + exportable=false`；普通完整 user/assistant 对话自动 `clean + exportable=true`。
- `needs_review` 与 `rejected` 默认不可导出；`heuristic` session 默认 `needs_review`。
- 同步 JSONL 导出与后台 export job 共用同一套二次质量校验。
- 默认排除 heuristic；只有 `include_heuristic=true` 且通过其余质量门时才允许导出。
- 前端对话历史页增加 `quality_status` 筛选，并展示 session/turn 的质量错误原因。

## Acceptance Criteria

- [ ] clean 普通对话自动可导出。
- [ ] parse failed、truncated、client disconnect 均 rejected 且不可导出。
- [ ] heuristic session 默认 needs_review 且不可导出。
- [ ] request/response messages 缺失时 rejected。
- [ ] tool call 链路不完整时 needs_review。
- [ ] 管理员误标失败/截断/断连数据为 exportable 时，导出仍会拦截。
- [ ] `include_heuristic=false` 默认排除 heuristic。
- [ ] `include_heuristic=true` 只放行通过其余质量门的数据。
- [ ] 前端筛选参数与后端 `quality_status` 字段一致。

## Definition of Done

- 后端单测覆盖质量评估器和导出二次校验。
- 前端测试覆盖筛选参数和质量错误展示。
- 定向测试通过。
- 不改变数据库 schema，不引入 AI 语义评分或自动周期导出。

## Technical Approach

- 在 service 层新增质量评估与导出判定函数，作为采集和导出的单一事实源。
- `ConversationCaptureService.capture` 在构造 turn record 时调用质量评估器。
- `ExportMessagesJSONL` 与 `executeExportJob` 在构造 JSONL payload 前应用 `CanExportConversationTurn`。
- 前端复用现有 `ConversationSessionFilters` / `ConversationTurnSummary` / `ConversationTurn` 类型扩展展示，不做页面重构。

## Decision (ADR-lite)

**Context**: 当前对话捕捉已经能采集、查看和导出，但默认数据仍需要人工判断是否可训练。

**Decision**: 第一版采用保守结构质量门，只自动放行结构完整、解析成功、未截断、未断连、非 heuristic 的普通对话。

**Consequences**: 初期可导出数据会偏少，但能显著降低脏数据误导出的风险；复杂语义评分、人工审核队列和自动周期导出留到后续任务。

## Out of Scope

- 不新增数据库字段或迁移。
- 不实现 AI 语义质量评分。
- 不实现自动周期导出。
- 不照搬 TokenRouter 的用户侧 notice confirm 或 group 级数据共享授权。

## Technical Notes

- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 本任务跨 backend service/repository、admin API 类型和前端页面。
- 当前工作区已有用户未提交改动，实施时只修改本任务相关文件。
