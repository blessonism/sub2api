# campaign reward settlement quality fixes

## Goal

修复活动邀请奖励与结算分支在质量闸门 review 中发现的资金安全、幂等、状态流转与前后端契约问题，使当前以兑换码增加额度的生产路径可靠入池，并为未来开启支付充值时保留安全边界。

## What I already know

- 当前项目没有开启支付充值，主要通过兑换码兑换增加额度。
- 活动模块已经接入注册邀请、兑换码充值入池、支付充值入池、排行榜、结算预览、最终结算和后台一键发放。
- Review 发现兑换码/支付充值入池、奖励发放、状态流转、尾差记录、后台手动扣减枚举存在质量风险。
- 当前工作分支是 `feature/campaign-rewards-settlement-docs`，基线是 `custom/main`。

## Requirements

- 兑换码兑换成功后，活动充值记录必须按 `campaign_id + source_type + source_id` 幂等，重复调用不能重复累加邀请充值金额或奖金池。
- 活动充值入池的邀请记录更新与奖金池 entry 写入必须在同一个事务中完成，避免额度已入账但活动统计丢失或半更新。
- 支付余额充值路径在未来启用时不能和兑换码路径双重记录活动充值；当前应避免 campaign 记录失败阻断真实充值完成。
- 后台一键发放必须使用稳定幂等维度，避免同一活动同一奖励结果重复发放，并允许明确处理失败重试语义。
- 奖励计算需要记录向下取整产生的尾差，并汇总到 `total_rounding_residual_cents`。
- 用户排行榜/我的数据需要在当前用户不在 Top50 时仍能得到自己的排名。
- 后台奖金池手动扣减枚举必须与数据库约束一致。
- 活动发布和状态流转至少不能把未来活动直接作为当前 active 活动统计；应按当前时间进入预热或进行中，并让进行中查询只命中真实活动期。

## Acceptance Criteria

- [ ] 同一个兑换码兑换重复触发 `RecordRecharge` 时，邀请充值金额和 pool entry 都只记录一次。
- [ ] campaign 记录失败不会让已完成的主充值/兑换链路变成失败状态。
- [ ] 同一 reward result 不能被一键发放重复入账。
- [ ] reward calculation 的尾差汇总不再永久为 0。
- [ ] 当前用户 Top50 外仍返回 `current_rank`。
- [ ] 前端提交的手动扣减类型可通过 DB 约束。
- [ ] 针对关键幂等、尾差、排行补位补充回归测试。

## Definition of Done

- 关键后端单元测试通过。
- 与 campaign 相关的 repository/service/handler 路径验证通过。
- 前端类型检查在依赖可用时应通过；若本地缺依赖，需要记录。
- Trellis task context 记录下游 fork 工作流和相关 backend/frontend 规范。

## Out of Scope

- 不开启真实支付充值。
- 不执行生产数据库迁移。
- 不推送远端分支。
- 不重做完整活动运营后台 UX。

## Technical Notes

- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 相关文件：`backend/internal/service/campaign_service.go`、`backend/internal/repository/campaign_repo.go`、`backend/internal/service/redeem_service.go`、`backend/internal/service/payment_fulfillment.go`、`backend/migrations/180_campaign_rewards_settlement.sql`、`frontend/src/views/admin/CampaignRewardsView.vue`。
