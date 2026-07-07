# campaign rewards admin ui polish

## Goal

优化邀请活动管理员页的布局与操作体验，让活动列表不再挤占详情区，右侧操作按钮和表单在常见后台视口下保持可读、可点、可扫描。

## What I already know

- 用户已明确指出活动列表和右侧宽度导致按钮被挤压。
- 当前主视图是 `frontend/src/views/admin/CampaignRewardsView.vue`。
- 当前页面已具备生命周期、创建弹窗、奖池调整预览、风险确认和结果明细等基础能力，本轮不重做业务流程。
- 当前分支是 `feature/campaign-rewards-admin-ui-r1`，任务 base branch 为 `custom/main`。

## Requirements

- 收窄并稳定左侧活动列表宽度，释放右侧详情区空间。
- 重排右侧活动操作区，降低 7 个按钮平铺造成的挤压。
- 修正右侧窄卡片内表单网格，避免按视口断点过早横排。
- 修正生命周期步骤在中等宽度下 5 列拥挤的问题。
- 保持现有 API、业务状态流转和 i18n 结构，不引入新组件库。

## Acceptance Criteria

- [ ] 管理员页在 `xl` 视口下右侧详情区明显更宽，左侧活动列表有合理最大宽度。
- [ ] 主要动作与次级/危险动作分组展示，按钮不因过度横排而互相挤压。
- [ ] 奖池调整、配置版本表单在窄卡片中不会被压成不可用的小列。
- [ ] 生命周期步骤在中等视口下不会强制 5 列。
- [ ] 现有邀请活动管理员页核心测试通过，或明确记录未能运行的原因。

## Out of Scope

- 不新增后端接口。
- 不改变奖励计算、结算、发放、删除或复制的业务逻辑。
- 不重构整个页面为多组件目录。
- 不处理用户侧邀请活动页。

## Technical Notes

- 本任务遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 前端约束参考 `.trellis/spec/frontend/type-safety.md`。
- 本轮优先使用 Tailwind utility 调整布局，不引入依赖。
