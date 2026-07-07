# campaign settlement consistency guard

## 背景

邀请活动已经具备最终结算和一键发放能力。当前邀请记录调整、排行榜手工调整会失效 final 结算结果，但奖池调整尚未纳入同一一致性规则；同时，进入发放批次后仍需要更明确地禁止继续修改会影响结算输入的数据。

这会带来两个核心风险：

- final 结算后调整奖池，旧 final results 仍可能被拿去发放。
- 已创建 payout batch 后继续改奖池、邀请记录或排行榜，会导致部分用户按旧口径发放、后续数据按新口径展示。

## 目标

- 奖池调整写入后必须失效已有 final 结算结果和 final 排行榜快照。
- 一旦活动已有 payout batch，禁止继续修改结算输入：奖池调整、邀请记录调整、排行榜手工调整。
- final 结算只能在活动结束后执行，避免活动期内提前 final 后继续产生新数据。
- 保持 R1 范围小，不处理配置版本 `CreateConfigVersion` 的结算失效语义。

## 非目标

- 不重构邀请活动状态机。
- 不新增数据库表或迁移。
- 不改用户侧活动展示与 warmup 可见性。
- 不处理配置版本对历史 final 的复杂影响。
- 不调整奖励计算公式、排行榜排序口径或发放金额计算方式。

## 需求

- 后端必须在 service 层提供统一校验，防止已存在 payout batch 后继续修改结算输入。
- `AddPoolAdjustment` 必须和 final 失效处于同一事务，避免“奖池调整成功但 final 未删除”的中间态。
- `AdjustInviteRecord` 和 `AddLeaderboardAdjustment` 继续沿用现有 final 失效逻辑，但在写入前增加 payout batch 锁定校验。
- `RecalculateRewards(..., final)` 必须校验当前时间已达到或超过活动 `end_at`。
- `Payout` 继续要求存在 final results；如果 final 已被失效删除，应返回缺少最终结算的错误。

## 验收标准

- [ ] final 后新增奖池调整会删除 final reward results 和 final leaderboard snapshot。
- [ ] final 后新增奖池调整后直接 payout 会返回 `CAMPAIGN_NO_FINAL_SETTLEMENT`。
- [ ] payout batch 存在后，奖池调整被拒绝。
- [ ] payout batch 存在后，邀请记录调整被拒绝。
- [ ] payout batch 存在后，排行榜手工调整被拒绝。
- [ ] 活动未结束时直接请求 final 结算被拒绝。
- [ ] 已有邀请活动用户侧规则移除和 pending invite 改动不被回退。

## 技术方案

### 后端 service

- 新增轻量守卫函数，例如 `ensureCampaignSettlementInputsMutable`：
  - 读取活动，保持现有 `paid` 状态拒绝。
  - 调用现有 payout batch 查询能力；只要存在 `processing`、`partial_success`、`success`、`failed` 等发放批次，拒绝修改结算输入。
  - R1 可复用 `ErrCampaignAlreadyPaid`，不新增错误码，降低前端适配范围。
- `AddPoolAdjustment`、`AdjustInviteRecord`、`AddLeaderboardAdjustment` 写入前调用该守卫。
- 新增或内联 final 时间校验：
  - `RecalculateRewards` 在 `status == final` 时读取 campaign。
  - 若 `time.Now().Before(campaign.EndAt)`，返回 `ErrCampaignInvalidConfig`。

### 后端 repository

- `AddPoolAdjustment` 改为事务：
  - 插入 `campaign_pool_adjustments`。
  - 调用已有 `invalidateFinalSettlementTx`。
  - 提交事务。
- 保持 `AdjustInviteRecord` 和 `AddLeaderboardAdjustment` 当前事务失效 final 的逻辑。

### 前端

- 管理员页 `canFinalize` 增加本地时间判断：当前时间达到 `selectedCampaign.end_at` 后才允许 final。
- 其余发放禁用逻辑保持现状，后端错误继续通过既有错误提示展示。

## 测试计划

- Go service tests：
  - payout batch 存在后拒绝奖池、邀请记录、排行榜调整。
  - 活动未结束时拒绝 final。
  - final 被奖池调整失效后 payout 返回无 final。
- Go repository tests：
  - `AddPoolAdjustment` 同一事务内插入调整并删除 final results / snapshots。
  - final 删除失败时事务回滚。
- Frontend unit test：
  - 未到活动结束时间时 final 按钮 disabled。

## Definition of Done

- 定向 Go 测试通过。
- 定向前端测试通过。
- 变更范围保持在邀请活动后端 service/repo/tests 与管理员页最小 UI 状态判断。
- 不引入生产数据迁移和高风险操作。

## Technical Notes

- 任务分支：`feature/campaign-rewards-admin-ui-r1`
- PR 目标分支：`custom/main`
- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`
- 现有 final 失效 helper：`invalidateFinalSettlementTx`
- 现有 final 结果入口：`GetFinalRewardResults` / `Payout`
