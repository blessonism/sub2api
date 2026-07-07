# 邀请活动管理员侧 UI 优化落地

## Goal

基于 `docs/CAMPAIGN_REWARDS_ADMIN_UI_OPTIMIZATION_CN.md` 的 R1/R2/R3 方案，优先完成管理员侧可落地的高价值 UI 优化：活动生命周期表达、用户侧预览与时间线、创建活动审阅弹窗、高风险动作确认、奖池调整影响预览，以及结算结果明细展示。

## Requirements

- 管理员侧 `/admin/campaign-rewards` 页面需要从按钮平铺升级为更清晰的运营控制台。
- 活动详情顶部需要展示生命周期阶段、当前状态提示和用户侧预览信息。
- 创建活动不能再直接静默创建默认草稿，需要先进入可审阅的创建弹窗。
- 发布、冻结、最终结算、一键发放、人工调减等高风险动作需要确认弹窗。
- 一键发放前必须确认已经生成最终结算结果。
- 奖池调整需要展示调整前、本次调整、调整后预计金额，并要求资金调整原因。
- 结算预览/最终结算后需要展示奖励结果明细，方便复核扣留和最终发放金额。
- 新增文案必须同步中英文 i18n。
- 保持后端接口兼容，不改奖励计算和发放幂等逻辑。

## Acceptance Criteria

- [x] `docs/CAMPAIGN_REWARDS_ADMIN_UI_OPTIMIZATION_CN.md` 存在，并作为管理员侧 UI 优化主参考。
- [x] 管理员活动详情展示生命周期流程条、当前阶段提示、用户侧预览和生命周期时间线。
- [x] 创建活动通过弹窗审阅默认参数后提交，保留原默认规则值。
- [x] 发布、冻结、最终结算、一键发放、人工调减均通过确认弹窗承接。
- [x] 一键发放在缺少最终结算时会阻止并提示。
- [x] 奖池调整展示调整前、本次调整、调整后预计金额；原因为空时阻止提交。
- [x] 结算结果明细表展示排名奖励、贡献奖励、毛奖励、最终发放、扣留金额和扣留原因。
- [x] 管理员页使用到的 `admin.campaignRewards.*` 文案在 zh/en 中均存在。
- [x] `pnpm --dir frontend typecheck` 通过。
- [x] `pnpm --dir frontend lint:check` 通过。

## Definition of Done

- 前端实现集中在管理员侧活动页面和对应 i18n 文案。
- 不修改后端结算、发放或数据库迁移逻辑。
- 不覆盖已有未提交改动，尤其是现存的后端测试改动和并行出现的用户侧生命周期改动。
- Trellis 上下文包含下游 fork 工作流、前端规范入口、类型安全规范和方案文档。

## Technical Approach

- 在 `CampaignRewardsView.vue` 内先内聚实现，避免过早拆组件扩大改动面。
- 使用 `BaseDialog` 承载创建活动弹窗，使用 `ConfirmDialog` 承接高风险动作。
- 使用本地 `computed` 派生活动阶段、时间线、奖池调整预览、结算结果展示和确认弹窗摘要。
- 创建活动弹窗复用原 `createDraft` 默认值，只把“直接创建”改为“审阅后创建”。
- 发放确认依赖本地最终结算结果；最终结算后保留 `calculation`，避免刷新详情后丢失发放确认上下文。

## Decision (ADR-lite)

**Context**：管理员侧已经具备活动管理和结算接口，但 UI 对生命周期、资金类动作、结算复核的表达不足，容易造成误操作或复核成本高。

**Decision**：优先在现有页面内落地交互和信息架构优化，不扩后端接口；把资金动作改为确认式操作，把创建和结算结果从黑盒变成可审阅。

**Consequences**：本轮能快速降低误操作风险并提升运营复核效率；后续若要继续做完整活动模板、活动编辑、历史版本列表或后端审计日志，可再拆独立任务。

## Out of Scope

- 不改用户侧页面作为本任务的必做范围。
- 不新增后端活动更新接口。
- 不改奖励计算公式、发放幂等策略或数据库结构。
- 不执行 git commit / push。

## Technical Notes

- 管理员页面：`frontend/src/views/admin/CampaignRewardsView.vue`
- 管理员 API：`frontend/src/api/admin/campaigns.ts`
- 活动通用类型：`frontend/src/api/campaigns.ts`
- 中文文案：`frontend/src/i18n/locales/zh.ts`
- 英文文案：`frontend/src/i18n/locales/en.ts`
- 方案文档：`docs/CAMPAIGN_REWARDS_ADMIN_UI_OPTIMIZATION_CN.md`
- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
