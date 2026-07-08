# 修复 Token 抽奖开奖安全与批次恢复

## Goal

修复 Token 抽奖开奖批次在 `processing`、`partial_success`、`failed` 状态下不可恢复，以及并发/重复触发可能重复发放、未发布活动可误开奖的问题，避免服务重启、进程中断或误操作造成资金风险。

## What I Already Know

- 当前 `LotteryCampaignService.Draw` 在已有批次状态为 `success`、`processing`、`partial_success` 时直接返回。
- `processing` 可能出现在批次已创建但 winners 未创建或未完成发放时。
- `partial_success` 表示已有部分中奖者发放成功，失败中奖者需要可重试。
- `lottery_winners.idempotency_key` 和批次唯一键已经提供幂等基础。
- 当前余额发放由 service 先查 winner 再调用余额发放服务，winner 成功状态是事后写入，并发开奖存在重复发放窗口。
- 当前同步资格/开奖服务层没有强制要求活动为 `published`。

## Requirements

- `success` 批次保持幂等返回，不重复开奖或发放。
- `processing` 批次不能永久卡死；再次调用 `Draw` 时应恢复同一批次的 winner 创建和余额发放流程。
- `partial_success` / `failed` 批次应允许重试未成功中奖者，不重复给已成功中奖者发余额。
- 已成功 winner 必须跳过，避免重复发放。
- 单个 winner 的余额发放与 winner success 状态写入必须在同一个数据库事务内完成，并锁定 winner/user 行。
- `draft` / `cancelled` / `archived` 活动不能同步资格或开奖。
- 无候选、无中奖者的开奖仍应能完成为成功批次。

## Acceptance Criteria

- [ ] 已有 `processing` 批次再次 `Draw` 会继续创建缺失 winners 并完成发放。
- [ ] 已有 `partial_success` 批次再次 `Draw` 只重试非 success winner。
- [ ] 已有 `success` 批次再次 `Draw` 仍直接返回，不重复发放。
- [ ] 同一 winner 的并发处理只能成功发放一次。
- [ ] 未发布活动调用同步或开奖返回错误，不创建批次、不发放余额。
- [ ] 新增或更新单元测试覆盖恢复与幂等行为。

## Definition of Done

- 后端相关测试通过。
- 代码不引入候选级真实调度或额外数据库 schema 变更。
- 不修改无关的活动中心、上游倍率监控、抽奖 UI 变更。

## Technical Approach

- 在 service 层把 `Draw` 拆成“获取/创建批次、确保 winners 存在、发放 pending/failed winners、刷新 summary”的可恢复流程。
- repository 继续利用 `ON CONFLICT` 和 winner `status` 防重；service 层跳过 `success` winner。
- 对已有新鲜 `processing` 批次直接返回；超过恢复窗口后接管恢复。
- 对已有 `processing` 批次，如果 winners 为空，使用当前候选重新生成 winners；如果 winners 已存在，直接进入发放重试。
- repository 增加 winner 级事务发放：`FOR UPDATE` 锁定 winner/user，更新用户余额、写入 `admin_balance` 兑换记录、标记 winner success 在同一事务内提交。

## Decision (ADR-lite)

**Context**: 抽奖批次创建、winner 创建、余额发放跨多步流程；原先复用批量余额发放服务会在 winner 状态写入前产生重复发放窗口。  
**Decision**: 保持现有 schema，使用已有唯一约束和 winner 状态做可恢复、可重入的 service 流程；winner 发放改为 repository 内单事务完成。  
**Consequences**: 修复卡死、补发和并发重复发放问题；直接 SQL 发放需要与现有 `admin_balance` 记录语义保持一致。

## Out of Scope

- 不新增批次锁、租约、后台补偿表或管理侧补发 UI。
- 不改变中奖算法和奖项配置语义。
- 不重做前端抽奖页面，仅允许对危险操作按钮做辅助禁用。

## Technical Notes

- 相关文件：`backend/internal/service/lottery_campaign_service.go`
- 相关测试：`backend/internal/service/lottery_campaign_service_test.go`
- 任务上下文：下游 fork 工作流、后端质量约束、代码复用思考指南。
