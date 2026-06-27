# brainstorm: 管理员余额与兑换入口整合

## Goal

将管理员侧栏中的“余额汇总”和“兑换记录”整合为一个统一入口，降低后台菜单密度，让管理员查找余额、额度、兑换流水相关信息时先进入同一处，再在页面内切换具体视图。

## What I already know

- 用户明确诉求是“兑换记录和余额汇总能整合一下”，核心原因是侧栏入口太多、不好找。
- 任务重点是后台导航与页面信息架构整合，不是把余额汇总和兑换记录的后端数据模型合并。
- 现有前端已有独立页面：
  - `frontend/src/views/admin/BalanceSummaryView.vue`
  - `frontend/src/views/admin/RedeemRecordsView.vue`
- 现有路由分别为：
  - `/admin/balance-summary`
  - `/admin/redeem-records`
- 现有侧栏在 `frontend/src/components/layout/AppSidebar.vue` 中分别展示 `nav.balanceSummary` 与 `nav.redeemRecords`。
- 现有侧栏已经支持带 children 的折叠菜单，但本任务更符合“减少侧栏入口”的页面内 tabs 方案。

## Decisions

- 统一侧栏入口命名确定为“余额与兑换”。

## Assumptions (temporary)

- 新统一路由推荐为 `/admin/balance-redemption` 或类似稳定路径。
- 统一页面默认展示“余额汇总”，因为它更像财务/额度总览；“兑换记录”作为流水明细放在第二个 tab。
- 原 `/admin/balance-summary` 与 `/admin/redeem-records` 可保留为兼容重定向或隐藏入口，避免旧链接失效。
- 不新增后端接口、不改数据库结构、不改变余额汇总和兑换记录的统计口径。

## Open Questions

- 暂无阻塞问题。

## Requirements (evolving)

- 侧栏管理员区只暴露一个与余额/兑换记录相关的统一入口。
- 统一页面内提供“余额汇总”和“兑换记录”两个 tab。
- 两个 tab 应保留原页面既有筛选、刷新、分页、排除用户配置等能力。
- 页面标题、路由 meta、中文/英文 i18n 文案需要与新入口一致。
- 旧独立入口从侧栏移除或隐藏，但不应破坏已有深链访问。
- 实现时应避免把两个已有 `AppLayout` view 直接嵌套导致布局重复；必要时将页面主体抽成可复用内容组件。

## Acceptance Criteria (evolving)

- [ ] 管理员侧栏中不再同时出现“余额汇总”和“兑换记录”两个独立入口。
- [ ] 管理员可以从统一入口进入页面，并在页面内切换“余额汇总 / 兑换记录”。
- [ ] “余额汇总”tab 的统计卡片、分组汇总、排除用户配置能力可正常使用。
- [ ] “兑换记录”tab 的类型筛选、刷新、分页、表格展示能力可正常使用。
- [ ] 旧路由 `/admin/balance-summary` 与 `/admin/redeem-records` 有明确处理策略，优先重定向到统一页面对应 tab。
- [ ] 简单模式、折叠侧栏、移动端侧栏行为不因本次整合回退。

## Definition of Done (team quality bar)

- Tests added/updated where appropriate for route/sidebar/tab behavior.
- Frontend lint/typecheck passes for touched files.
- i18n 文案同步更新中英文。
- Docs/notes updated if behavior changes need to be recorded.
- Rollout/rollback considered if existing admin deep links are changed.

## Out of Scope (explicit)

- 不重构余额汇总或兑换记录的后端业务逻辑。
- 不新增兑换码管理、充值订单、订阅套餐等更宽泛财务功能。
- 不改变用户余额、订阅额度、兑换流水的计算口径。
- 不处理支付订单管理与支付配置入口。

## Technical Notes

- 已读取下游 fork 工作流：`.trellis/spec/guides/downstream-fork-workflow.md`。
- 任务 base branch 已设置为 `custom/main`。
- 建议任务分支已设置为 `feature/admin-balance-redemption-navigation`。
- `implement.jsonl` 与 `check.jsonl` 已加入下游 fork 工作流上下文。
- 当前仓库存在大量未提交改动，后续实现必须避免误改或回滚无关文件。
- 现有页面都是完整 view 且包含 `AppLayout`；统一 tab 页面实现时更适合抽出主体内容组件，或让旧 view 通过轻量 wrapper 复用同一内容，避免布局嵌套。
- 实现采用旧 view 的 `embedded` 模式复用现有业务逻辑：独立访问时仍渲染 `AppLayout`，统一页内只渲染内容，避免复制余额汇总和兑换记录逻辑。

## Implementation Notes

- 新增 `BalanceRedemptionView.vue` 作为“余额与兑换”统一入口，使用 query `tab=redeem-records` 打开兑换记录标签，默认展示余额汇总。
- 侧栏管理员区用 `/admin/balance-redemption` 替换原 `/admin/balance-summary`，并移除独立 `/admin/redeem-records` 入口。
- 旧路由 `/admin/balance-summary` 与 `/admin/redeem-records` 保留为兼容重定向。
- 简易模式继续限制兑换记录能力：访问统一页的 `redeem-records` 标签会被路由守卫重定向。
- 已同步路由预加载、中英文 i18n、侧栏测试、路由守卫测试和统一页测试。

## Proposed MVP

1. 新增管理员统一页面，例如 `AdminBalanceRedemptionView.vue`。
2. 将余额汇总与兑换记录各自页面主体抽成可在 tab 内复用的内容组件。
3. 新页面顶部展示统一标题与两个 tab，默认 tab 为余额汇总。
4. 侧栏用一个入口替换原“余额汇总”和“兑换记录”两个入口。
5. 旧路由重定向到统一页面，并通过 query 或 path segment 打开对应 tab。
