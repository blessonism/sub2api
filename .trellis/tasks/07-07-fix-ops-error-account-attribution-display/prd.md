# fix ops error account attribution display

## Goal

优化管理端“错误请求”中账号列的可排查性：当错误日志没有 `account_id` 时，不再显示含糊的 `-`，而是明确区分“请求尚未分配账号”和“上游错误未记录账号归因”；同时保护已选中上游账号的错误日志必须携带账号 ID。

## What I already know

- 用户反馈：使用记录 / 错误请求中账号偶尔显示为 `-`，影响检查。
- 当前列表组件在 `row.account_id` 为空时直接显示 `-`。
- 当前详情弹窗在上游错误且 `account_id` 为空时显示 `—`。
- 后端错误请求查询从 `ops_error_logs.account_id` 读取账号归因，并通过 `LEFT JOIN accounts` 补充账号名。
- 已观察到部分上游错误落库路径显式传入 `AccountID` / `AccountName`，但仍需要检查是否存在遗漏路径。

## Requirements

- 账号列有 `account_id` 时，显示账号名；账号名缺失时显示 `#账号ID`。
- 非上游归因错误且没有 `account_id` 时，显示“未分配账号”，说明请求在账号调度前失败。
- 上游错误但没有 `account_id` 时，显示“账号未记录”，提示这是归因缺失而不是普通空值。
- 错误详情弹窗与列表采用一致的账号归因文案。
- 增加回归测试覆盖上述展示语义；如发现后端已选账号错误存在漏写账号 ID 的路径，同步补强。

## Acceptance Criteria

- [ ] 错误请求列表不再用 `-` 表示账号缺失。
- [ ] 上游错误缺少 `account_id` 时可以一眼识别为“账号未记录”。
- [ ] 调度前失败缺少 `account_id` 时可以一眼识别为“未分配账号”。
- [ ] 账号名缺失但 `account_id` 存在时仍显示 `#账号ID`。
- [ ] 相关前端测试通过；若后端归因逻辑变更，相关 Go 测试通过。

## Definition of Done

- Tests added/updated for affected behavior.
- Lint/type constraints respected for touched frontend/backend code.
- No unrelated dirty working-tree changes are modified.
- Trellis task context includes downstream fork workflow for implement/check.

## Out of Scope

- 不迁移或回填历史错误日志数据。
- 不重构错误请求表格整体列设置。
- 不改变错误日志数据库 schema。

## Technical Notes

- Primary UI files:
  - `frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`
  - `frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue`
- Current table test:
  - `frontend/src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts`
- Backend query/reference:
  - `backend/internal/repository/ops_repo_request_details.go`
  - `backend/internal/service/gateway_service.go`
- Required fork context:
  - `.trellis/spec/guides/downstream-fork-workflow.md`
