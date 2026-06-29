# brainstorm: 上游中继监控体验问题优化

## Goal

修复管理员上游中继监控页在推荐历史状态、轻量刷新、历史用量可读性和自动监控状态展示上的问题，避免失败状态被掩盖，并降低管理员读取历史与局部刷新连接器指标的成本。

## What I already know

- 用户指出 `UpstreamRelayGroupMonitoringView.vue` 中 6 个具体问题：
  - 推荐历史状态展示只按 `applied` 和 `suggestion_count` 判断，会把 `status=failed` 且有建议的 run 显示为“待确认”。
  - 全局轻量刷新成功后，如果停留在“历史用量”tab，`usageHistory` 没有同步刷新。
  - 连接器行缺少单连接器“轻量刷新用量/余额”按钮。
  - 历史用量筛选区依赖 placeholder，缺少可见 label。
  - 自动监控状态只显示单个最高优先级状态，无法同时表达同步、探测、推荐、自动应用。
  - 历史用量 tab 缺少当前筛选范围的日期级汇总。
- 前端类型规范要求连接器轻量刷新使用 `refreshConnectorMetrics(id)`，目标接口为 `/connectors/:id/metrics/refresh`，刷新后需要同时更新连接器和候选状态。
- 本仓库是下游二开 fork，修改必须基于 `custom/main` 派生的功能分支，并把下游 fork 规则加入实现/检查上下文。

## Assumptions

- 本轮优先在现有组件内收敛改动，不引入新的 UI 依赖。
- 历史汇总条基于当前已加载的历史列表与筛选条件计算，用于降低当前页读表成本，不改后端分页接口契约。
- 自动监控状态 chips 从最后加载/保存的策略快照读取，避免把未保存表单状态展示为正在运行。

## Requirements

- 推荐历史状态必须优先表达 `failed`、`running`、`success` 等 run 状态，再补充“已应用/待确认/无建议”的业务结果。
- 全局轻量刷新成功后，若当前 tab 为历史用量，应重新加载历史列表。
- 连接器行应提供单连接器轻量刷新入口，并复用既有刷新 API 与状态更新流程。
- 历史用量筛选项需要具备紧凑的可见 label。
- 自动监控状态区应以 chips 分别展示同步、探测、推荐、自动应用的开关与间隔。
- 历史用量 tab 顶部应展示当前已加载范围的汇总信息。
- 连接器和候选映射加载不能静默截断第一页数据；若继续使用分页接口，前端必须拉取所有分页或明确展示截断范围。
- 顶部今日用量口径必须与历史用量保持一致，或通过文案明确“快照今日用量”和“历史日汇总”的差异；优先复用历史用量接口获取当天汇总。
- 历史用量筛选条件处于待应用状态时，自动刷新/轻量刷新不能悄悄用旧筛选结果覆盖列表；页面需要展示当前结果实际基于的筛选条件。
- 应用 Priority 建议前必须增加强确认，降低批量修改 priority 的误操作风险。

## Acceptance Criteria

- [ ] `status=failed` 的推荐 run 即使有 suggestions，也显示失败状态。
- [ ] 在历史用量 tab 执行全局轻量刷新后，会重新请求历史用量。
- [ ] 连接器行存在单连接器轻量刷新按钮，点击后调用单连接器 metrics refresh。
- [ ] 历史筛选日期、连接器、分组、搜索输入都有可见 label。
- [ ] 自动监控状态能同时展示同步、探测、推荐、自动应用。
- [ ] 历史用量页面展示当前加载数据的总成本、总 tokens、连接器数等汇总。
- [ ] 初始加载能完整获取连接器和候选映射分页数据，不再只取前 100 条。
- [ ] 概览今日用量使用历史用量今日汇总，或页面明确展示不同口径。
- [ ] 历史用量筛选待应用时，刷新不会让用户误以为当前输入条件已经生效。
- [ ] Priority 建议应用按钮在管理员完成强确认前不可点击。

## Definition of Done

- 更新相关单元测试或视图测试覆盖核心回归。
- 运行针对性测试，必要时运行类型检查。
- 不执行 `git commit` / `git push`。

## Out of Scope

- 不修改后端历史用量接口分页/聚合契约。
- 不调整自动监控策略的保存逻辑。
- 不做整体页面重构。
- 不实现单条/多选应用 Priority 建议；后端 apply 语义仍保持按 run 批量应用。

## Technical Notes

- 相关组件：`frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
- 相关测试：`frontend/src/views/admin/__tests__/UpstreamRelayGroupMonitoringView.spec.ts`
- 相关 i18n 测试：`frontend/src/i18n/__tests__/upstreamRelayMonitoringLocales.spec.ts`
- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
- 前端类型约束：`.trellis/spec/frontend/type-safety.md`
