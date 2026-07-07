# 邀请活动奖池充值注入范围配置

## Goal

管理员创建或调整邀请活动规则时，可以选择奖池动态注入的充值来源范围：仅邀请的新用户充值注入奖池，或所有用户充值都注入奖池。目标是在不改变邀请榜单核心统计口径的前提下，让活动奖池规模可以按运营策略灵活配置。

## What I already know

- 当前活动配置版本已包含 `pool_injection_rate`、`recharge_threshold_cents`、`allow_accumulated_recharge` 等奖池与充值资格相关字段。
- 当前用户侧文案表达为“邀请新用户的达标充值按比例注入奖池”，说明现有默认口径是 invitee-only。
- 当前奖池入账依赖邀请记录与 `campaign_pool_entries`，`RecordRecharge` 会找到活动内的邀请记录并把该受邀用户充值累加到邀请记录、参与人统计和奖池。
- 管理员侧创建活动与配置版本调整入口已经存在，需要在这些入口增加充值注入范围选择并向后端传递。

## Assumptions

- 历史活动与未显式配置的活动默认保持现有行为：仅邀请的新用户充值注入奖池。
- “所有用户充值都注入奖池”只扩大动态奖池资金来源，不自动让非受邀用户计入邀请榜单、有效邀请数或贡献奖励权重。
- 非受邀用户充值入池需要保持幂等，避免同一支付单或兑换单重复注入奖池。

## Open Questions

- 已确认：当选择“所有用户充值都注入奖池”时，非受邀用户充值只增加奖池，不参与邀请榜单和贡献奖励。

## Requirements

- 增加活动配置字段，用于保存奖池充值注入范围，至少支持：
  - `invitees_only`：仅邀请的新用户充值注入奖池。
  - `all_users`：活动期内所有用户充值都注入奖池。
- 管理员创建活动时可以选择充值注入范围。
- 管理员创建配置版本时可以调整充值注入范围，并保存为新的配置版本。
- 后端入池逻辑按配置范围处理充值：
  - `invitees_only` 沿用现有逻辑。
  - `all_users` 对没有邀请记录的充值也写入奖池入账记录，但不改变邀请记录、邀请人数和榜单贡献。
- 前端活动规则摘要与用户侧规则文案应反映当前注入范围，避免用户误解奖池资金来源。
- 旧数据、旧 API 调用和旧配置版本在字段缺省时应安全回落到 `invitees_only`。

## Acceptance Criteria

- [ ] 管理员创建活动时能选择“仅邀请新用户充值”或“所有用户充值”作为奖池注入范围。
- [ ] 管理员调整活动配置版本时能同步调整注入范围，并能在规则摘要中看到当前口径。
- [ ] `invitees_only` 模式下现有邀请记录、榜单、奖池行为保持不变。
- [ ] `all_users` 模式下，活动期内非受邀用户的充值会按注入比例增加奖池。
- [ ] `all_users` 模式下，非受邀用户充值不会增加任何邀请人的有效邀请数、邀请充值额或贡献奖励权重。
- [ ] 同一充值来源重复回调或重复处理不会重复注入奖池。
- [ ] 历史活动与缺省配置不会因为新字段产生空值错误或行为变化。

## Definition of Done

- 后端模型、迁移、仓储、服务与接口均支持注入范围字段。
- 前端管理员创建/配置版本表单与规则摘要支持该字段。
- 用户侧规则文案根据配置展示对应注入范围。
- 覆盖关键测试：默认兼容、仅邀请新用户、所有用户、重复充值幂等、非受邀用户不影响榜单。
- 与下游 fork 工作流一致：任务基线为 `custom/main`，实现与检查上下文包含下游 fork 指南。

## Technical Approach

推荐采用“配置版本枚举 + 入池逻辑分支”的实现：

- 在 `campaign_config_versions` 增加 `pool_injection_scope` 字段，默认 `invitees_only`，约束可选值。
- Go 服务层增加常量与校验，把字段纳入 `CampaignConfigVersion`、创建活动输入、配置版本输入。
- 仓储层读写配置版本时同步读写该字段。
- 对现有邀请充值路径保持原事务逻辑；为 `all_users` 增加无邀请记录的入池路径，写入可幂等的奖池记录。
- 前端 API 类型、管理员表单、配置版本表单和规则摘要增加范围选择与展示。
- i18n 增加中英文文案，明确“仅受邀新用户”与“所有用户”的差异。

## Decision (ADR-lite)

**Context**: 奖池来源与邀请榜单统计是两个相关但不同的概念。若把“所有用户充值”同时纳入榜单，活动会从邀请奖励变成全站充值排行，影响现有结算逻辑和用户预期。

**Decision**: 将本任务范围限定为“奖池资金来源配置”。用户已确认采用方案 1：非受邀充值只增加奖池，不参与邀请榜单。

**Consequences**: 实现可以复用现有奖池汇总与结算拆分逻辑，但需要为非邀请充值建立可审计、可幂等的入池记录；未来若要做全站充值榜，应作为独立活动类型或独立榜单任务。

## Out of Scope

- 不改变邀请榜单排序口径。
- 不新增全站充值排行榜。
- 不改变奖励发放方式、分池比例和排名权重算法。
- 不处理生产数据迁移或生产回填。

## Technical Notes

- 下游 fork 指南：`.trellis/spec/guides/downstream-fork-workflow.md`
- 相关后端文件：
  - `backend/migrations/180_campaign_rewards_settlement.sql`
  - `backend/internal/service/campaign_service.go`
  - `backend/internal/repository/campaign_repo.go`
  - `backend/internal/handler/admin/campaign_handler.go`
  - `backend/internal/service/payment_fulfillment.go`
  - `backend/internal/service/redeem_service.go`
- 相关前端文件：
  - `frontend/src/api/admin/campaigns.ts`
  - `frontend/src/api/campaigns.ts`
  - `frontend/src/views/admin/CampaignRewardsView.vue`
  - `frontend/src/views/user/CampaignRewardsView.vue`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`
