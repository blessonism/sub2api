# 邀请活动生产文案调整

## Goal

把邀请活动管理页中偏开发调试、本地演示或实现细节的文案，调整为可直接面向生产运营使用的表达。

## Requirements

- 移除邀请活动相关文案中的“默认活动”“首期”“本地最终结算”“复用用户页规则”“未启用暂停状态”等内部实现或调试语境。
- 保留现有活动创建、发布、冻结、试算、最终结算、发放、复制和归档流程，不改变接口或交互逻辑。
- 中英文文案保持语义一致，后台运营场景优先使用“活动”“结算试算”“用户端展示”等生产表达。
- 用户首次进入邀请活动页时，即使还没有活动参与记录，也应能看到自己的邀请码和可复制邀请链接。
- 已存在活动参与快照时，应继续优先使用快照中的邀请码和邀请链接，避免历史活动入口漂移。

## Acceptance Criteria

- [x] 邀请活动管理页不再展示本地演示、开发调试或内部实现语境文案。
- [x] 管理员仍可通过原按钮和提示完成活动生命周期操作。
- [x] 相关前端测试通过。
- [x] 无活动参与记录的用户也能通过 `/campaigns/:id/me` 获取邀请码和邀请链接。
- [x] 已有活动参与快照时继续优先返回快照邀请码和邀请链接。

## Definition of Done

- 相关 i18n 文案已更新。
- 聚焦测试通过。
- 任务上下文包含下游 fork 工作流约束。

## Technical Approach

调整 `frontend/src/i18n/locales/zh.ts` 与 `frontend/src/i18n/locales/en.ts` 中 `admin.campaignRewards` 和用户侧少量邀请活动展示文案，不改 Vue 组件结构和 API 调用。

同时在 `CampaignService.GetMyData` 中补齐邀请身份兜底：优先使用 `campaign_participants` 快照；当快照缺失时，通过仓储确保当前用户的 affiliate profile，并返回 `/register?aff=<code>` 格式的邀请链接。

## Out of Scope

- 不调整邀请活动业务规则。
- 不新增暂停、下线或结算状态。
- 不改后端接口和数据库结构。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，本仓库二开变更需遵守下游 fork 分支边界。
- 已读取 `.trellis/spec/frontend/index.md` 与 `.trellis/spec/frontend/type-safety.md`，本次为 i18n 文案修正，不涉及类型结构变更。
