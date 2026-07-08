# User Activity Center

## Goal

将用户侧原“邀请活动”入口升级为“活动中心”：当前只有邀请类活动时，用户进入后直接看到活动详情；未来增加抽奖、限时福利等营销活动时，在同一页面内用轻量切换承载，避免新增一层中转页。

## Requirements

- 用户侧侧边栏入口显示为“活动中心”，英文为 “Activity Center”。
- 新用户侧主路由使用活动中心语义，推荐路径为 `/activities`；旧 `/campaign-rewards` 访问必须保持兼容。
- 活动中心常态只有一个活动时不显示活动列表、活动广场或多余 tab，直接展示邀请活动详情。
- 活动中心需要保留轻量活动注册结构，至少包含活动 `id`、标题、状态/标签、组件或渲染入口；当未来有 2-3 个活动时，页面顶部可显示紧凑切换条。
- 现有邀请活动业务能力保持不变：活动生命周期、复制邀请码/链接、排行榜、邀请记录、奖励预估、空状态和错误处理继续沿用现有 API。
- 后台邀请活动管理页不在本任务中更名，仍保留“邀请活动结算/管理”语义。

## Acceptance Criteria

- [ ] 普通用户和管理员“我的账户”区域的用户侧入口显示“活动中心”，并跳转到 `/activities`。
- [ ] `/activities` 渲染现有邀请活动详情；当只注册一个活动时，不出现活动切换条。
- [ ] `/campaign-rewards` 仍能访问同一活动中心页面，不破坏历史入口。
- [ ] 路由 meta 使用活动中心标题和描述的 i18n key。
- [ ] 前端 i18n 同时补齐 zh/en 活动中心文案。
- [ ] 现有邀请活动视图测试继续覆盖邀请详情核心行为；新增或更新路由测试覆盖新路径和旧路径兼容。

## Definition of Done

- 前端相关测试已更新并通过针对性运行。
- 类型检查通过，或如项目既有问题阻塞则明确记录。
- Trellis `implement.jsonl` 与 `check.jsonl` 包含下游 fork 工作流和前端规范上下文。

## Technical Approach

- 保持邀请活动详情组件的现有数据加载与业务逻辑，优先做轻量结构调整，避免引入后端统一活动接口。
- 新增或调整活动中心容器，内部注册当前邀请活动；当注册活动数量为 1 时直接渲染详情，当数量大于 1 时展示紧凑 tab。
- 将用户侧导航指向 `/activities`，用路由 alias 或 redirect 兼容 `/campaign-rewards`。
- 更新用户侧 nav 与 route 标题描述文案；管理员活动管理文案不跟随更名。

## Decision (ADR-lite)

**Context**: 常态只有 1-3 个营销活动，如果做活动广场会增加不必要点击和空界面。

**Decision**: 采用“活动中心作为统一归属，单活动直达详情，多活动轻量切换”的前端结构；第一版不新增后端活动聚合接口。

**Consequences**: 当前体验保持直接，未来接入抽奖时只需注册新活动组件；如果活动数量显著增加，再升级为后端配置化活动列表。

## Out of Scope

- 不实现抽奖活动业务、奖池、中奖记录或后台配置。
- 不重构管理员邀请活动管理后台。
- 不修改邀请注册、返利或活动结算后端接口。
- 不新增复杂运营配置平台。

## Technical Notes

- 当前用户侧邀请活动视图：`frontend/src/views/user/CampaignRewardsView.vue`。
- 当前用户侧路由：`/campaign-rewards`，route name `CampaignRewards`。
- 当前侧栏入口来自 `frontend/src/components/layout/AppSidebar.vue` 的 `buildSelfNavItems`。
- 邀请活动与邀请返利是不同入口：`/affiliate` 与限时活动入口需继续并存。
- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，任务 base branch 为 `custom/main`，工作分支为 `feature/user-activity-center`。
