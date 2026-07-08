# 历史用量隐藏零消费分组

## Goal

让管理员在「上游分组倍率监控 / 历史用量」中默认只看到选中时间范围内有消费或使用的上游分组，并提供开关按需显示零消费分组，避免大量未使用分组掩盖真实消费记录。

## What I already know

- 用户明确希望历史用量默认只显示所选时间内有消费的分组。
- 需要一个开关选择是否显示无消费/无使用分组。
- 历史用量分页必须与筛选结果一致，不能只在前端当前页过滤。
- 现有历史用量支持日期、连接器、分组、关键词、异常过滤和分页。

## Assumptions

- “有消费或使用”定义为 `actual_cost > 0 OR total_tokens > 0`。
- “无消费分组”定义为 `actual_cost <= 0 AND total_tokens <= 0`。
- 默认隐藏无消费分组；开关开启后包含无消费分组。

## Requirements

- 历史用量默认隐藏零消费/零使用分组。
- 开关开启时显示零消费/零使用分组，并且分页、总数、顶部汇总一起按开关状态变化。
- 开关属于历史用量筛选条件，改动后需要与现有筛选“待应用/应用筛选”行为一致。
- API 查询参数显式表达是否包含零消费记录。

## Acceptance Criteria

- [x] 默认请求历史用量时不包含零消费分组。
- [x] 开启“显示无消费分组”后，请求会包含零消费分组。
- [x] 后端列表与汇总在默认状态下都只统计 `actual_cost > 0 OR total_tokens > 0` 的记录。
- [x] 历史用量分页不会因为零消费记录被隐藏而产生总数/页数错位。
- [x] 前端视图和 API 类型测试覆盖新开关与查询参数。

## Definition of Done

- Tests added/updated where appropriate.
- Targeted frontend/backend checks pass or known unrelated failures are documented.
- Downstream fork workflow constraints are respected.

## Out of Scope

- 不改变历史用量写入逻辑，仍允许后台记录零消费快照。
- 不新增数据库字段或迁移。
- 不改变“仅看异常”的当前页前端过滤语义。

## Technical Approach

后端在历史用量查询过滤器中新增 `IncludeZeroUsage`，默认 false；列表与汇总 SQL 在 false 时追加消费条件。前端新增 `usageHistoryIncludeZeroUsage` 开关，纳入草稿/已应用筛选状态，并通过 `include_zero_usage=true` 传给 API。

## Technical Notes

- `backend/internal/service/upstream_relay_group_monitoring.go`
- `backend/internal/handler/admin/upstream_relay_group_monitoring_handler.go`
- `backend/internal/repository/upstream_relay_group_monitoring_repo.go`
- `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`
- `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
- `.trellis/spec/guides/downstream-fork-workflow.md`

## Implementation Notes

- 后端新增 `include_zero_usage` 查询参数；默认 false 时，列表和汇总都过滤为 `actual_cost > 0 OR total_tokens > 0`。
- 前端新增“显示无消费分组”开关，默认关闭；开启后传 `include_zero_usage=true` 并按现有“应用筛选”流程刷新。
- 内部定格检查显式包含零消费记录，避免默认筛选影响后台补定格判断。
