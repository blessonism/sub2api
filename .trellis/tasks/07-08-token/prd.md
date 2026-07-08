# Token 抽奖开奖安全修复

## Goal

修复 Token 抽奖活动 code review 中发现的资金安全与开奖恢复问题，确保重复触发、并发触发、进程中断或误操作不会导致重复发放、永久卡住或未发布活动发放余额。

## What I already know

- 现有活动中心仅实现邀请奖励活动，后端 `campaigns` 域强绑定邀请、奖池、排行榜、结算与发放。
- 用户端 `CampaignRewardsView.vue` 已有多活动切换器结构，但目前只挂载邀请活动。
- Token 统计口径按成功计费请求计算：`actual_cost > 0` 且 `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`。
- 余额发放可复用 `GrantUserBalances`，它会更新用户余额并写入 `admin_balance` 记录。
- 当前实现存在三类 review 问题：
  - `pending/failed` 中奖记录在发放前没有原子 claim，并发开奖可能重复调用余额发放。
  - 已创建 `processing` 批次后若中断，后续开奖直接早退，无法恢复。
  - 手动同步/开奖未强制要求活动已发布，草稿或取消活动可能误发余额。

## Requirements

- 开奖前必须校验活动状态为 `published`，草稿、取消、归档活动不能同步资格或开奖。
- 自动调度只处理到期批次；手动开奖也必须经过同一服务层安全校验。
- 重复触发同一批次必须幂等：
  - 已成功批次直接返回结果。
  - 部分成功或失败批次只重试未成功 winner。
  - 不允许同一个 winner 被并发流程重复发放余额。
- `processing` 批次必须可恢复：
  - 未过恢复窗口的 processing 仍视为运行中。
  - 过期 processing 可被后续手动或调度触发接管并继续/重试。
- 余额发放前应通过数据库原子状态转换 claim winner，claim 失败则跳过该 winner。
- 保留现有开奖、中奖去重、余额备注、用户端展示合同。

## Acceptance Criteria

- [ ] 并发或重复触发同一批次时，同一个 winner 最多发放一次余额。
- [ ] 已卡住的 `processing` 批次超过恢复窗口后可以继续处理并完成 summary。
- [ ] `draft/cancelled/archived` 活动调用同步资格或开奖返回错误，不会创建批次或发放余额。
- [ ] `success` 批次重复触发直接返回，不产生额外余额记录。
- [ ] `partial_success/failed` 批次可重试失败 winner 并更新批次 summary。
- [ ] 现有活动中心前端和抽奖 API 类型保持通过 typecheck。

## Definition of Done

- 后端单元测试覆盖状态校验、processing 恢复、winner claim 幂等、重复开奖不重复发放。
- 必要的前端 typecheck/API 测试保持通过。
- `implement.jsonl` / `check.jsonl` 包含下游 fork 工作流和相关 backend/frontend spec。
- 变更遵守 `custom/main` 二开边界，当前任务分支为 `feature/token-lottery-campaign`。

## Out of Scope

- 新增奖品类型、通知系统、公开中奖名单。
- 重做活动中心 UI。
- 替换现有 `GrantUserBalances` 发放能力。

## Technical Notes

- 任务基线：`custom/main`。
- 任务分支：`feature/token-lottery-campaign`。
- 主要修复模块：
  - `backend/internal/service/lottery_campaign_service.go`
  - `backend/internal/repository/lottery_campaign_repo.go`
  - `backend/internal/service/lottery_campaign_service_test.go`
  - 管理端抽奖面板仅做必要禁用或保留现状，核心安全在后端。
