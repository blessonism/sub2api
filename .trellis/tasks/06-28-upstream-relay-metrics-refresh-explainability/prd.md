# brainstorm: 刷新用量余额结果可解释化

## Goal

让管理员在【上游分组倍率监控】手动刷新用量/余额后，能明确看到每个连接器的余额刷新、Usage 刷新、未更新 Group 和失败原因，减少只靠单条错误提示排障的成本。

## What I already know

- 轻量刷新接口是 `POST /api/v1/admin/upstream-relay-group-monitors/connectors/:id/metrics/refresh`。
- 当前接口已返回 `balance_available`、`balance_error`、`usage_available`、`usage_error`、`snapshots` 和 `refreshed_at`。
- 前端批量刷新由页面逐个调用单连接器刷新接口聚合完成。
- 自动刷新使用 `silent: true`，不应打扰当前页面操作。

## Requirements

- 后端保留现有字段，并新增结构化 `status`、`balance_detail`、`usage_detail`。
- 手动刷新后，前端展示页面内摘要条和可展开详情。
- 连接器行内展示最近一次刷新状态短提示。
- 自动静默刷新只更新数据，不覆盖手动刷新结果摘要。
- 不新增数据库迁移，不改变 usage 归因算法。

## Acceptance Criteria

- [ ] 余额和 Usage 都成功时返回 `success`。
- [ ] 余额成功但 Usage 失败时返回 `partial`，并列出未更新 Group。
- [ ] 无快照或无候选绑定时返回可解释的 skipped/failed 明细。
- [ ] 前端手动刷新展示摘要和详情。
- [ ] 前端自动静默刷新不展示摘要。

## Definition of Done

- 后端 service 测试覆盖结构化刷新结果。
- 前端 API 和页面测试覆盖摘要、详情、静默刷新。
- 针对性 Go / Vitest 检查通过。

## Out of Scope

- 不持久化刷新结果。
- 不做右侧抽屉。
- 不新增新的批量刷新后端接口。

## Technical Notes

- 遵循 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 遵循 `.trellis/spec/backend/quality-guidelines.md` 中 Admin upstream relay group monitoring 场景。
- 遵循 `.trellis/spec/frontend/type-safety.md` 中 Upstream Relay Monitoring API Types 场景。
