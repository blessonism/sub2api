# 修复邀请活动管理员 UI Review 问题

## Goal

修复上一轮 code review 发现的两个关键问题：管理员活动最终结算发放入口不应依赖刷新后会丢失的前端本地状态；创建活动弹窗的前端校验应与后端活动配置契约保持一致，减少运营提交后端必失败配置的情况。

## What I already know

- 当前分支为 `feature/campaign-rewards-admin-ui-r1`，基线任务属于邀请活动 UI 优化。
- Review 发现 P1：`CampaignRewardsView.vue` 用本地 `calculation.value?.calculation_status === 'final'` 控制一键发放，刷新或重新选择活动后 `calculation` 被清空，导致已持久化 final settlement 的活动无法从 UI 发放。
- Review 发现 P2：创建活动前端校验只检查比例和大于 0、权重数量匹配，弱于后端 `validateCampaignConfig`：比例和必须等于 1，参与排名的权重总和必须为 100，金额/比例/数量必须是合法有限值。
- 后端 `Payout` 从持久化 final reward results 读取，不依赖前端内存态。
- 当前任务只处理 review 问题，不扩展新的 UI 模块或活动生命周期能力。

## Assumptions

- 可以在管理员页进入/刷新活动详情时拉取或生成本地可审阅的 final 结果来源；若现有 API 无读取 final 结果接口，优先增加最小后端接口而不是让 UI 继续依赖本地内存。
- 创建弹窗前端校验应尽量与后端同构，但后端仍是最终安全边界。

## Requirements

- 管理员生成 final settlement 后，即使刷新页面或重新进入活动，也能看到可审阅的 final 结果，并在确认后执行 payout。
- 管理员页不得因为本地 `calculation` 为空而隐藏服务端已存在的 final settlement 发放入口。
- 创建活动弹窗前端校验必须拒绝：无效日期、非有限数字、负金额/比例、非正整数排名奖励数量、比例和不等于 1、参与排名权重和不等于 100、权重数量不足或非法权重。
- 补充/更新前端测试覆盖上述两个回归场景。
- 保持后端接口、前端 API 类型、视图状态和 i18n 文案一致。

## Acceptance Criteria

- [ ] 已有 final reward results 的活动在页面加载/选择后可以展示结算结果并启用 payout。
- [ ] 刷新页面后不需要重新执行 final settlement 也能发放已存在的 final settlement。
- [ ] 创建活动表单在前端阻止与后端配置契约不一致的比例/权重/非法数值。
- [ ] 相关前端单测、typecheck、lint 通过。
- [ ] 后端目标测试通过；若新增接口，增加对应 service/handler 或 API 覆盖。

## Definition of Done

- Tests added/updated for the regression paths.
- Lint/typecheck pass for frontend.
- Target backend tests pass for affected campaign service/handler behavior.
- No unrelated refactor or broad UI rewrite.

## Out of Scope

- 不重做邀请活动整体 UI 布局。
- 不改变后端结算/发放核心业务规则。
- 不处理非 review 指出的其他 Trellis active tasks。

## Technical Notes

- 前端重点文件：`frontend/src/views/admin/CampaignRewardsView.vue`、`frontend/src/api/admin/campaigns.ts`、管理员页测试。
- 后端候选文件：`backend/internal/handler/admin/campaign_handler.go`、`backend/internal/service/campaign_service.go`、`backend/internal/repository/campaign_repo.go`。
- 规范：`.trellis/spec/guides/downstream-fork-workflow.md`、`.trellis/spec/guides/cross-layer-thinking-guide.md`、`.trellis/spec/frontend/type-safety.md`。
