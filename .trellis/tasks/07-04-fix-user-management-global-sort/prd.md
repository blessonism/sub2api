# 修复用户管理全量排序

## Goal

用户管理列表的排序应作用于全部用户数据，再按分页返回结果，而不是只对当前页数据做前端排序。

## What I already know

- 用户反馈：用户管理的排序不应只针对当前页，应针对全部数据排序。
- 当前仓库已有活动奖励管理相关未提交变更，本任务需避免覆盖无关改动。
- 本仓库是下游二开仓库，任务基线应遵守 custom/main 与 fix/* 分支边界。

## Requirements

- 用户管理列表在切换排序字段或排序方向时，排序结果必须覆盖全量用户集合。
- 分页仍按当前页码与每页条数返回。
- 保持现有筛选、搜索和用户管理操作行为不回退。

## Acceptance Criteria

- [x] 在用户管理页对任意支持排序的字段排序时，第 1 页展示的是全量排序后的第一页，而非当前页局部重排。
- [x] 切换排序后分页参数与查询参数能传到后端或等价的数据源。
- [x] 相关测试或验证覆盖全量排序语义。

## Definition of Done

- Tests added/updated where appropriate.
- Lint/typecheck or scoped tests pass where feasible.
- No unrelated working tree changes are reverted.

## Out of Scope

- 不调整用户管理页面的视觉设计。
- 不重构无关的用户 CRUD、余额、订阅或活动奖励逻辑。

## Technical Notes

- Context: .trellis/spec/guides/downstream-fork-workflow.md
