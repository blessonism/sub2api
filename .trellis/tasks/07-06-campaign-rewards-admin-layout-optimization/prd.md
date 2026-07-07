# campaign rewards admin layout optimization

## Goal

优化管理员邀请活动页面的信息布局，解决活动列表作为独立侧栏后导致详情区域被挤压、页面留白过多的问题，让运营在大屏和常规笔记本宽度下都能更高效地扫视活动与详情。

## What I Already Know

- 用户明确反馈：活动列表单独放在右侧/侧边列，导致页面空白过多。
- 当前页面为 `frontend/src/views/admin/CampaignRewardsView.vue`，顶层使用两栏网格承载活动列表和活动详情。
- 现有测试位于 `frontend/src/views/admin/__tests__/CampaignRewardsView.spec.ts`，已有活动渲染、生命周期、确认弹窗和创建校验覆盖。
- 本仓库是下游二开 fork，修改必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。

## Requirements

- 活动列表不再作为固定窄侧栏与详情区并排占据首屏主布局。
- 活动列表应融入主内容流，减少大屏宽度下的无效空白，并让详情区获得完整横向空间。
- 保留活动列表的加载、空状态、选中态、状态展示和点击选择行为。
- 不改变邀请活动的业务状态流转、结算、发放、奖池调整或创建逻辑。

## Acceptance Criteria

- [ ] 管理员邀请活动页面中，活动列表与详情区不再使用固定侧栏式双列主布局。
- [ ] 有活动时仍能看到活动名称、时间、生命周期提示和状态标签，并可点击切换选中活动。
- [ ] 未选中活动或空列表时的现有空状态仍正常显示。
- [ ] 相关前端测试通过，或若现有环境阻塞需记录原因。

## Definition of Done

- 代码变更范围集中在管理员邀请活动页面及必要测试。
- 不引入新的 UI 依赖。
- 不覆盖或回退当前工作树中已有的其他未提交改动。

## Technical Approach

采用最小前端布局调整：将活动列表从 `xl:grid-cols-[sidebar_main]` 的固定侧栏结构改为主内容流内的完整宽度列表区；列表项在较宽屏幕下使用响应式网格横向排列，详情区独占下方完整宽度。这样可以复用现有状态与事件处理，同时减少空白并避免改动业务逻辑。

## Decision (ADR-lite)

**Context**: 当前页面为了持续展示活动列表使用侧栏布局，但活动数量少或详情内容较宽时会造成主区域宽度浪费。
**Decision**: 不新增组件、不改 API，把列表改为顶部/主流式活动选择区，详情卡片保持现有内容层次。
**Consequences**: 交互路径保持稳定；页面滚动时列表不再 sticky，但获得更好的横向空间利用率。

## Out of Scope

- 不新增活动筛选、搜索或分页。
- 不重构活动创建向导、结算结果表或奖池业务逻辑。
- 不调整用户侧邀请活动页面。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 `.trellis/spec/frontend/index.md` 和 `.trellis/spec/frontend/type-safety.md`。
- 相关设计背景参考 `docs/CAMPAIGN_REWARDS_ADMIN_UI_OPTIMIZATION_CN.md`。
