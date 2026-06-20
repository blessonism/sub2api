# brainstorm: 管理员排行榜计入订阅套餐额度

## Goal

管理员排行榜需要把订阅用户使用订阅套餐额度产生的用量也统计进去，避免只统计余额扣费或现金消耗时漏掉订阅套餐覆盖的真实资源使用。

## What I already know

- 管理员希望排行榜不漏算订阅用户消耗的订阅套餐额度。
- 当前反馈是“目前的统计没有计算到订阅用户使用的订阅套餐额度”。
- 仓库是 `Wei-Shaw/sub2api` 的下游二开版本，修改仓库内容必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 管理员 Dashboard 的用户消耗排行榜当前按 `usage_logs.actual_cost` 聚合。
- 订阅路径会把 `ActualCost` 写入 `user_subscriptions.daily/weekly/monthly_usage_usd`，所以统计口径应覆盖这部分消费。
- 现有实现里 `usage_logs.actual_cost` 可能因为历史数据/特殊账单路径为 `0`，需要明确是否回补为 `total_cost * rate_multiplier`。

## Assumptions (temporary)

- “排行榜”优先指管理员 Dashboard 的用户消耗排行榜。
- 订阅套餐额度可能在请求结算时不进入余额扣费字段，导致按实际扣费金额聚合的排行榜低估订阅用户消耗。
- 需要优先复用已有 usage log 字段和订阅扣减记录，不引入新的生产数据迁移，除非现有数据无法支撑。

## Open Questions

- 暂无阻塞问题；先通过代码确认现有统计口径和订阅额度落库位置。

## Requirements (evolving)

- 管理员排行榜统计口径应覆盖订阅套餐额度消耗。
- 对订阅相关记录，如果 `actual_cost` 为空或为 `0`，应使用可还原的额度消耗口径回补，避免低估排行。
- 保持现有排行榜交互能力，例如点击用户跳转到用量筛选。
- 不改变普通用户排行榜的邮箱脱敏和隐私口径。

## Acceptance Criteria (evolving)

- [x] 订阅套餐覆盖的用量会计入管理员排行榜的用户消耗统计。
- [x] 订阅记录在缺少 `actual_cost` 时能按回补规则计入排行。
- [x] 非订阅用户和余额扣费用户的现有统计结果不回退。
- [x] 有后端或前端单元测试覆盖订阅额度计入排行的场景。
- [x] 目标范围内的测试通过。

## Definition of Done

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green for touched scope where practical
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope

- 不在本任务中重构完整计费系统。
- 不改变订阅套餐扣减本身的业务规则。
- 不新增独立排行榜页面，除非现有管理员 Dashboard 统计链路无法承载。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，当前应保持二开分支边界。
- 已实现 `(usage_logs.subscription_id IS NOT NULL OR usage_logs.billing_type = 1) AND actual_cost <= 0` 时按 `total_cost * rate_multiplier` 回补管理员用户消耗榜的统计口径；删除订阅后 `subscription_id` 被置空的历史记录仍可通过 `billing_type` 识别为订阅额度消耗。
- 已新增仓储单元测试覆盖订阅额度回补表达式。
