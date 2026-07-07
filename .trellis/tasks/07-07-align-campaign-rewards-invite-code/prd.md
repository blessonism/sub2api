# Align Campaign Rewards Invite Code

## Goal

让用户侧“邀请活动”页面中的“我的邀请码 / 邀请链接”展示与“邀请返利”页面保持一致，降低两个入口并存时的认知差异。

## Requirements

- “邀请活动”邀请入口中，邀请码展示样式、按钮文案和复制反馈对齐“邀请返利”的“我的邀请码”模块。
- “邀请活动”邀请入口中，邀请链接展示样式、按钮文案和复制反馈对齐“邀请返利”的“邀请链接”模块。
- 保留活动页面已有的活动门槛、预计奖励、进度提示、排行榜和邀请记录逻辑。
- 不改变后端活动邀请码、邀请链接生成逻辑，仅调整用户可见展示与复制口径。

## Acceptance Criteria

- [x] 活动页显示“我的邀请码”作为与邀请返利一致的主展示块，复制按钮文案为“复制邀请码”。
- [x] 活动页显示“邀请链接”作为并列展示块，复制按钮文案为“复制链接”。
- [x] 活动页的邀请码和邀请链接保持上下两行展示，不在桌面端并排。
- [x] 活动页邀请入口与活动规则左右卡片在桌面端保持等高对齐。
- [x] 复制邀请码成功时使用“邀请码已复制”，复制邀请链接成功时使用“邀请链接已复制”。
- [x] 活动页相关回归测试覆盖新文案与复制行为。

## Definition of Done

- Tests added or updated for the changed user-facing behavior.
- Targeted frontend test passes.
- Downstream fork branch/context requirements are recorded for implementation and check.

## Technical Approach

复用“邀请返利”页面现有视觉结构和 i18n 口径，在 `CampaignRewardsView.vue` 的邀请入口卡片内对齐邀请码与邀请链接的布局、按钮和复制反馈；同步补齐 zh/en 文案，并更新用户活动页测试断言。

## Decision (ADR-lite)

**Context**: 两个入口分别服务常驻返利和限时活动，但用户看到的“我的邀请码/邀请链接”应保持一致，避免同一个动作在两个页面呈现不同优先级和文案。

**Decision**: 仅调整活动页用户侧展示与复制口径，不合并两个页面的数据源或改变活动规则。

**Consequences**: UI 认知更一致；活动页仍保留活动专属统计和规则，不影响邀请返利常驻能力。

## Out of Scope

- 不调整邀请返利页面本身。
- 不改活动邀请 API 或邀请链接生成逻辑。
- 不改管理端活动配置和结算逻辑。

## Technical Notes

- 已参考 `frontend/src/views/user/AffiliateView.vue` 的邀请码/邀请链接模块。
- 已参考 `frontend/src/views/user/CampaignRewardsView.vue` 当前活动页邀请入口实现。
- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md` 的下游 fork 分支边界。
