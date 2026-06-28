# brainstorm: 上游赔率监控全局策略配置与建议预览

## Goal

为管理员版上游赔率监控增加“全局策略配置 + 建议预览”。现有系统已经能基于固定规则生成上游中继推荐建议，本任务希望把这些固定规则变成管理员可调控的全局策略，并在真正生成/应用建议前先预览策略影响，让系统提供建议但判断标准由管理员掌控。

进一步对齐后的产品方向：本能力不是孤立的“建议生成器”，而是上游中继监控与调度决策控制台。自动化负责采集和计算，推荐策略负责判断和排序，管理员负责最终决策和应用。

## What I already know

- 用户明确选择先做“全局策略配置 + 建议预览”。
- 本阶段不做分组级策略覆盖，也不把建议自动应用到目标分组 priority。
- 当前后端已有上游中继监控能力，包括连接器、倍率快照、候选关系、探测结果、推荐 run、推荐 suggestion。
- 当前 `GenerateRecommendations` 是无请求参数接口，调用固定规则生成并保存推荐 run。
- 当前建议生成逻辑位于 `backend/internal/service/upstream_relay_group_monitoring.go`：
  - 只纳入启用且连接器 active 的候选。
  - 需要存在新鲜有效倍率来源。
  - 需要最近探测成功，且探测结果未超过新鲜度窗口。
  - 若有健康统计，连续失败会被排除；样本数不少于 3 且成功率低于 50% 会被排除。
  - 排序优先级为：上游有效倍率更低优先、成功率更高优先、p95 延迟更低优先。
  - 生成的新 priority 以固定起点和步长递增。
- 当前前端 API 已有推荐 run / suggestion 类型与列表、详情、应用、删除接口。

## Assumptions (temporary)

- “全局策略配置”指影响所有上游中继候选的统一规则，不包含某个连接器、上游组、目标组或账号的单独覆盖。
- “建议预览”默认不改变目标组 priority，不调用现有 apply 接口。
- 策略配置应持久化，刷新页面后仍可继续使用。
- 预览应展示每条候选为什么入选或被排除，避免管理员只能看到最终排序。
- “倍率快照新鲜度 / 探测新鲜度”不应作为主要业务概念暴露给管理员；管理员真正关心的是自动同步和自动探测的间隔，新鲜度应由系统根据间隔派生或作为高级说明。

## Open Questions

- 已确认：建议预览结果不保存为历史 run，只做即时试算；真正需要留痕时再生成正式推荐记录。

## Requirements (evolving)

- 管理员可以查看和编辑一份全局推荐策略。
- 全局策略至少覆盖当前硬编码规则中的核心调控点：
  - 探测新鲜度窗口。
  - 倍率快照/用量推导倍率新鲜度窗口。
  - 最小成功率。
  - 最小样本数。
  - 是否允许连续失败候选进入建议。
  - priority 起点与步长。
  - 排序因子顺序或权重（倍率、成功率、延迟）。
- 管理员可以在保存策略前或保存后发起预览，看到策略对当前候选集生成的建议结果。
- 预览结果不写入 `upstream_relay_recommendation_runs` / `upstream_relay_recommendation_suggestions`，也不改变目标组 priority。
- 预览结果需要明确区分：
  - 会进入建议的候选。
  - 被排除的候选及排除原因。
  - 候选的新旧 priority、倍率来源、健康摘要、置信度。
- 现有“应用建议”能力不在本阶段扩展为自动应用；本阶段保持人工确认边界。
- 后续设计应拆出“自动监控配置”和“推荐策略配置”：
  - 自动监控配置回答“多久刷新数据”，例如自动同步上游倍率间隔、自动探测候选间隔、失败重试间隔、并发限制。
  - 推荐策略配置回答“怎么判断和排序”，例如最小成功率、最小样本数、连续失败排除、排序顺序、priority 起点/步长。
- 手动操作是最高优先级兜底：立即同步、立即探测、单个探测、批量探测、启停候选、手动调整 priority 都应保留。
- 任何会修改 priority 的动作必须由管理员确认并留下审计记录；自动监控和即时预览都不能静默修改 priority。

## Acceptance Criteria (evolving)

- [ ] 管理员可以在上游赔率监控页面看到全局策略配置入口。
- [ ] 修改策略参数后可以预览推荐结果，且预览不会直接修改目标组 priority。
- [ ] 预览结果能解释候选入选/排除原因。
- [ ] 现有固定推荐逻辑被策略参数驱动，不再只能通过改代码调整阈值。
- [ ] 默认策略与当前硬编码行为保持等价或接近，避免升级后建议突然大幅变化。
- [ ] 后端测试覆盖默认策略、阈值变化、排除原因和预览不应用变更。
- [ ] 前端类型、API 调用和关键交互测试按现有项目习惯补齐。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 分组、连接器、账号、目标组级别的单独策略覆盖。
- 一键自动应用建议或定时自动应用建议。
- 生产数据库或真实上游配置变更。
- 重做现有连接器、候选、探测、推荐 run 的整体信息架构。
- 后台自动调度、批量手动同步/探测、新鲜度由自动间隔派生，建议拆到后续实现阶段。

## Technical Notes

- 任务目录：`.trellis/tasks/06-28-upstream-relay-monitoring-global-policy-preview`
- Base branch：`custom/main`
- Work branch：`feature/upstream-relay-policy-preview`
- 已加入实现/检查上下文：`.trellis/spec/guides/downstream-fork-workflow.md`
- 现有路由入口：`backend/internal/server/routes/admin.go`
- 现有 handler：`backend/internal/handler/admin/upstream_relay_group_monitoring_handler.go`
- 现有 service：`backend/internal/service/upstream_relay_group_monitoring.go`
- 现有 repository：`backend/internal/repository/upstream_relay_group_monitoring_repo.go`
- 现有迁移：`backend/migrations/164_upstream_relay_group_monitoring.sql`
- 现有前端 API：`frontend/src/api/admin/upstreamRelayGroupMonitors.ts`
- 现有前端视图：`frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
- 设计落地文档：`.trellis/tasks/06-28-upstream-relay-monitoring-global-policy-preview/design.md`
