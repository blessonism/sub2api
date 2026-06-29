# brainstorm: 历史用量页 UI 打磨

## Goal

优化管理员上游中继监控页的历史用量 tab，让管理员能更快判断当前筛选范围内的成本、token、连接器/分组覆盖、采集新鲜度和异常行，减少在明细表中手动心算与排查的成本。

## What I already know

- 用户希望继续打磨历史用量页面细节和 UI。
- 当前页面已有基础筛选、当前页成本/token/连接器汇总、连接器分组表格、加载/空状态。
- 当前任务优先做无需后端改造的前端优化：
  - 顶部汇总增强：日期范围、分组数、最新采集时间、平均成本/百万 token。
  - 快捷日期筛选：今天、昨天、近 7 天、近 30 天。
  - 连接器分组标题显示小计：成本、tokens、分组数、最新采集时间。
  - 明细行标记异常：有成本无 token、有 token 无成本、采集时间过旧。
- 前端规范要求历史用量接口字段保持 `usage_date`、`connector_id`、`connector_name`、`upstream_group_id`、`group_name`、`platform`、`actual_cost`、`total_tokens`、`checked_at` 等后端 JSON 名称，不改契约。

## Assumptions

- 汇总基于当前已加载页数据，不引入后端聚合接口。
- “采集过旧”默认按距当前时间超过 24 小时提示。
- 快捷日期按钮只写入筛选输入并触发重新加载。

## Requirements

- 历史用量顶部汇总应更完整，支持快速判断范围、分组覆盖、最新采集时间和单位 token 成本。
- 筛选区应支持常用时间范围快捷按钮。
- 连接器分组标题应展示当前分组内的小计，避免逐行心算。
- 明细行应对明显异常或过旧数据给出轻量提示，不打断表格阅读。
- 所有新增 UI 文案必须补充 zh/en i18n 和测试。

## Acceptance Criteria

- [ ] 历史用量 tab 展示日期范围、分组数、最新采集时间、平均成本/百万 token。
- [ ] 历史用量 tab 提供今天、昨天、近 7 天、近 30 天快捷筛选。
- [ ] 每个连接器分组标题展示成本、tokens、分组数、最新采集时间。
- [ ] 明细行对成本/token 不一致或采集过旧数据展示异常提示。
- [ ] 相关视图测试和 i18n 测试通过。

## Definition of Done

- 更新相关视图测试与 i18n 测试。
- 运行针对性测试，必要时运行 typecheck。
- 不执行 `git commit` / `git push`。

## Out of Scope

- 不修改后端接口或新增聚合 API。
- 不引入图表库或趋势图。
- 不重构整个监控页组件结构。

## Technical Notes

- 相关组件：`frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
- 相关测试：`frontend/src/views/admin/__tests__/UpstreamRelayGroupMonitoringView.spec.ts`
- 相关 i18n 测试：`frontend/src/i18n/__tests__/upstreamRelayMonitoringLocales.spec.ts`
- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
