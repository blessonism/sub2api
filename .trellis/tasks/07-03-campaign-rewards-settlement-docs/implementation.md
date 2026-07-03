# 活动邀请奖励与结算实现文档

## 1. 实现原则

- 以独立活动模块实现，避免改动现有普通邀请返利的核心语义。
- 复用现有用户、邀请关系、支付订单、兑换码/充值码、余额赠额能力。
- 数据写入采用可追溯、可重算、可冻结、可审计的模型。
- 奖励发放必须幂等；预览、冻结、最终结算、发放是不同阶段，不混用状态。
- 首期先实现完整运营闭环，自动风控模型延后，保留人工审核与风险标记。
- 活动账务金额统一使用整数最小货币单位，默认人民币分；禁止用浮点数作为活动结算事实源。

## 2. 建议工作包

### PR1：活动模型与配置版本

目标：建立活动配置、状态流转和版本化规则。

主要工作：

- 新增活动实体和配置版本实体。
- 新增活动状态机：草稿、预热中、进行中、审核中、公示中、待发放、已发放、已取消/提前终止。
- 实现基础配置校验：时间区间、奖励比例、Top10 权重、最低发放金额、动态注入比例。
- 活动发布时冻结奖励分配规则；活动开始后只允许有效充值门槛和动态注入比例生成新版本。
- 校验首期同一时间只能存在一个进行中的邀请奖励活动。
- 后台提供活动创建、编辑、发布、取消、提前终止接口。

验收：

- 活动可创建草稿并发布。
- 活动开始后修改门槛或注入比例会生成新版本。
- 排行榜/贡献池比例、奖励人数、名次权重、最低发放金额、发放方式和发放渠道发布后冻结。
- 配置版本不追溯影响已有邀请记录和充值入池记录。
- 配置版本 ID 会写入邀请记录、入池记录和奖励计算结果。

### PR2：邀请统计与达标判定

目标：把用户邀请关系映射到活动内邀请记录，并按活动规则判定有效邀请。

主要工作：

- 被邀请人注册时识别活动归因：邀请链接优先，其次邀请码，再次其他可验证归因。
- 写入活动邀请记录，并保存门槛快照。
- 支持累计充值达标。
- 支付订单或兑换充值成功后，更新被邀请用户累计有效充值。
- 达到门槛后更新邀请记录为待审核或已生效。
- 活动结束后拒绝新的注册、充值、邀请进入本期统计；结束时仍未支付成功、未兑换成功或未充值到账的订单不进入本期待审核。

验收：

- 注册层和首充/达标层数据分开统计。
- 活动期内注册但结束后达标的用户不计入本期有效邀请。
- 同一被邀请人不能给多个邀请人贡献有效邀请。
- 不实现“未注册”邀请记录；未注册点击追踪如有需要，后续单独设计。

### PR3：奖金池与入池记录

目标：实现初始奖金池、动态注入、待确认、已确认、扣减和调整。

主要工作：

- 新增充值入池记录。
- 仅纳入本次活动被邀请用户产生的有效充值。
- 排除赠送金额、管理员手动赠送、非真实有效订单。
- 按支付成功或兑换成功时间判断活动归属，并按有效充值金额 × 10% 写入动态入池金额。
- 支持初始奖金、追加奖金、手动补偿、异常扣减记录。
- 新增奖金池扣减明细，支持多次部分退款、冲正、撤销兑换和风控拒绝的幂等审计。
- 明确奖金池调整金额符号：初始奖金、追加、奖金池层补偿、低额回收、尾差回收为正；异常扣减、人工调减为负。
- 支持奖金池已确认、待确认、预计总额和趋势数据。

验收：

- 500 元有效充值产生 50 元动态入池金额。
- 退款、冲正、撤销兑换会扣减有效充值和入池金额。
- 奖金池调整有完整审计记录。
- 奖金池层手动调整只写入 `campaign_pool_adjustments`，不写入充值入池记录，避免双记账。
- 用户级补发或扣回写入 `campaign_reward_adjustments`，不混入奖金池层调整。

### PR4：排行榜与奖励计算

目标：实现实时预估、结束冻结、审核后最终结算。

主要工作：

- 实时榜单按有效邀请人数排序。
- 并列规则依次为累计有效充值金额、达到当前邀请数时间、参与活动时间。
- 活动结束时冻结预估榜单。
- 奖励计算支持预览、冻结、最终三种状态。
- 排行榜奖励池 = 最终奖金池 × 80%。
- 贡献奖励池 = 最终奖金池 × 20%。
- Top10 权重为 `30/20/15/10/8/6/4/3/2/2`。
- 最终结算使用活动发布时冻结的奖励配置版本。
- 贡献奖励按 `√有效邀请人数` 分配。
- 低于最低发放金额的奖励最终发放为 0，并回收到奖金池。
- 单用户最终发放金额向下取整到分，取整尾差回收到奖金池且不二次分配。

验收：

- Top10 奖励金额与权重一致。
- 贡献奖励总额不超过贡献奖励池。
- 榜单冻结后新增数据不影响冻结快照。
- 最终结算可重算但保留历史计算批次。
- 最终奖金池能与实际发放总额、低额回收、取整尾差和其他未发放回收金额对平。

### PR5：用户端页面

目标：让用户能看到活动、邀请入口、我的数据、邀请记录和排行榜。

主要工作：

- 活动首页。
- 我的数据区域。
- 邀请记录列表。
- 排行榜页面。
- 活动规则与数据延迟提示。
- 用户昵称、邮箱等信息脱敏。

验收：

- 用户能复制专属邀请码和邀请链接。
- 用户能看到有效邀请、待审核、无效邀请、当前排名和预计奖励。
- 排行榜默认展示前 50 名，并固定展示当前用户自己的排名。
- 页面清楚标注预估数据以最终审核结算为准。

### PR6：管理后台

目标：支持运营完整配置、审核、预览、结算和发放。

主要工作：

- 活动列表和活动详情。
- 活动基础配置。
- 邀请规则配置和门槛版本。
- 奖金池配置与调整记录。
- 奖励配置与计算预览。
- 数据看板。
- 审核队列。
- 冻结榜单、最终结算、结果公示。
- 发放批次与失败重试。

验收：

- 管理员可完成从创建活动到一键发放的完整链路。
- 关键操作有权限控制和审计记录。
- 奖励预览不产生余额变动。

### PR7：发放与审计

目标：安全地把最终奖励一键发放到账户余额。

主要工作：

- 新增发放批次和明细。
- 对每个用户生成幂等键。
- 发放前校验活动状态、最终结算状态、是否已发放。
- 调用现有余额赠额/余额调整能力。
- 记录发放前后余额快照。
- 支持失败明细重试。
- 发放后退款/冲正生成追偿记录，不修改历史排行榜和原发放批次。
- 追偿扣回使用幂等键，并记录余额扣回前后快照、余额流水 ID、错误信息和处理时间。

验收：

- 重复点击发放不会重复加余额。
- 部分失败可重试成功项不重复处理。
- 发放后活动状态更新为已发放。
- 发放后追偿可经后台审核后从余额扣回；余额不足时记录待追偿金额。
- 重复处理同一追偿事件不会重复扣余额。

## 3. 数据表建议

### 3.1 `campaigns`

- `id`
- `name`
- `description`
- `cover_url`
- `rules_text`
- `status`
- `warmup_start_at`
- `start_at`
- `end_at`
- `audit_start_at`
- `audit_end_at`
- `publicity_start_at`
- `publicity_end_at`
- `payout_due_at`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

### 3.2 `campaign_config_versions`

- `id`
- `campaign_id`
- `version`
- `version_scope`
- `effective_at`
- `recharge_threshold_cents`
- `allow_accumulated_recharge`
- `pool_injection_rate`
- `rank_pool_ratio`
- `contribution_pool_ratio`
- `rank_reward_count`
- `rank_weights_json`
- `min_payout_amount_cents`
- `change_reason`
- `created_by`
- `created_at`

### 3.3 `campaign_participants`

- `id`
- `campaign_id`
- `user_id`
- `invite_code_snapshot`
- `invite_link_snapshot`
- `participant_status`
- `joined_at`
- `valid_invite_count`
- `pending_invite_count`
- `invalid_invite_count`
- `invitee_recharge_amount_cents`
- `estimated_rank_reward_cents`
- `estimated_contribution_reward_cents`
- `estimated_total_reward_cents`
- `final_rank_reward_cents`
- `final_contribution_reward_cents`
- `final_total_reward_cents`
- 唯一约束：`campaign_id + user_id`

### 3.4 `campaign_invite_records`

- `id`
- `campaign_id`
- `config_version_id`
- `inviter_user_id`
- `invitee_user_id`
- `invite_source`
- `threshold_snapshot_cents`
- `registered_at`
- `qualified_at`
- `effective_recharge_amount_cents`
- `status`
- `risk_level`
- `invalid_reason`
- `audit_status`
- `audit_by`
- `audit_at`
- `audit_note`
- 唯一约束：`campaign_id + invitee_user_id`

### 3.5 `campaign_pool_entries`

- `id`
- `campaign_id`
- `config_version_id`
- `invite_record_id`
- `invitee_user_id`
- `source_type`
- `source_id`
- `source_success_at`
- `effective_recharge_amount_cents`
- `injection_rate_snapshot`
- `pool_amount_cents`
- `pool_status`
- `confirmed_at`
- `deducted_amount_cents`
- `last_deduct_reason`
- 唯一约束：`campaign_id + source_type + source_id`

### 3.6 `campaign_pool_deductions`

- `id`
- `campaign_id`
- `pool_entry_id`
- `source_type`
- `source_id`
- `deduct_amount_cents`
- `idempotency_key`
- `reason`
- `processed_at`
- `created_at`
- 唯一约束：`idempotency_key`

### 3.7 `campaign_pool_adjustments`

- `id`
- `campaign_id`
- `adjustment_type`
- `amount_cents`，可正可负
- `reason`
- `operator_id`
- `created_at`

### 3.8 `campaign_leaderboard_snapshots`

- `id`
- `campaign_id`
- `snapshot_type`
- `rank`
- `user_id`
- `valid_invite_count`
- `invitee_recharge_amount_cents`
- `reached_count_at`
- `joined_at`
- `estimated_reward_cents`
- `final_reward_cents`
- `created_at`
- 索引：`campaign_id + snapshot_type + rank`

### 3.9 `campaign_reward_results`

- `id`
- `campaign_id`
- `config_version_id`
- `user_id`
- `rank`
- `rank_reward_amount_cents`
- `contribution_weight`
- `contribution_reward_amount_cents`
- `gross_reward_amount_cents`
- `min_payout_amount_snapshot_cents`
- `final_payout_amount_cents`
- `withheld_amount_cents`
- `withheld_reason`
- `rounding_residual_cents`
- `calculation_status`
- `calculated_at`
- 索引：`campaign_id + calculation_status`

### 3.10 `campaign_reward_adjustments`

- `id`
- `campaign_id`
- `user_id`
- `reward_result_id`
- `adjustment_type`
- `amount_cents`，可正可负
- `reason`
- `operator_id`
- `status`
- `payout_batch_id`
- `created_at`
- `processed_at`

### 3.11 `campaign_payout_batches`

- `id`
- `campaign_id`
- `batch_no`
- `status`
- `operator_id`
- `total_users`
- `total_amount_cents`
- `success_count`
- `failed_count`
- `started_at`
- `finished_at`

### 3.12 `campaign_payout_items`

- `id`
- `batch_id`
- `campaign_id`
- `user_id`
- `reward_result_id`
- `amount_cents`
- `balance_before_snapshot`
- `balance_after_snapshot`
- `status`
- `idempotency_key`
- `error_message`
- `processed_at`
- 唯一约束：`idempotency_key`

### 3.13 `campaign_payout_recoveries`

- `id`
- `campaign_id`
- `payout_item_id`
- `user_id`
- `source_type`
- `source_id`
- `recover_amount_cents`
- `recovered_amount_cents`
- `status`
- `idempotency_key`
- `balance_before_snapshot`
- `balance_after_snapshot`
- `balance_ledger_id`
- `error_message`
- `reason`
- `review_by`
- `review_at`
- `processed_at`
- `created_at`
- 唯一约束：`idempotency_key`

## 4. API 设计建议

### 4.1 用户端接口

- `GET /api/v1/campaigns/active`：获取当前活动首页数据。
- `GET /api/v1/campaigns/:id/me`：获取我的活动数据。
- `GET /api/v1/campaigns/:id/invites`：获取我的邀请记录。
- `GET /api/v1/campaigns/:id/leaderboard`：获取活动排行榜。
- `GET /api/v1/campaigns/:id/rules`：获取活动规则。

### 4.2 管理端接口

- `GET /api/v1/admin/campaigns`：活动列表。
- `POST /api/v1/admin/campaigns`：创建活动。
- `GET /api/v1/admin/campaigns/:id`：活动详情。
- `PATCH /api/v1/admin/campaigns/:id`：更新活动基础信息。
- `POST /api/v1/admin/campaigns/:id/publish`：发布活动。
- `POST /api/v1/admin/campaigns/:id/terminate`：提前终止活动。
- `POST /api/v1/admin/campaigns/:id/config-versions`：新增配置版本。
- `GET /api/v1/admin/campaigns/:id/pool`：奖金池汇总。
- `POST /api/v1/admin/campaigns/:id/pool-adjustments`：奖金池调整。
- `GET /api/v1/admin/campaigns/:id/dashboard`：活动数据看板。
- `GET /api/v1/admin/campaigns/:id/audit-items`：审核队列。
- `POST /api/v1/admin/campaigns/:id/audit-items/:item_id/approve`：审核通过。
- `POST /api/v1/admin/campaigns/:id/audit-items/:item_id/reject`：审核拒绝。
- `POST /api/v1/admin/campaigns/:id/freeze`：冻结排行榜。
- `POST /api/v1/admin/campaigns/:id/recalculate`：重新计算奖励预览或最终结算。
- `POST /api/v1/admin/campaigns/:id/publicize`：进入公示。
- `POST /api/v1/admin/campaigns/:id/payout`：一键发放到账户余额。
- `POST /api/v1/admin/campaigns/:id/payout/retry`：重试失败发放明细。
- `POST /api/v1/admin/campaigns/:id/reward-adjustments`：新增用户级补发、扣回或人工修正。
- `POST /api/v1/admin/campaigns/:id/recoveries/:recoveryId/process`：处理发放后追偿扣回。

## 5. 服务拆分建议

- `CampaignService`：活动生命周期、状态机、配置版本、奖励规则冻结、进行中活动唯一性校验。
- `CampaignInviteService`：邀请归因、活动邀请记录、有效邀请判定。
- `CampaignPoolService`：奖金池入池、确认、扣减明细、调整、趋势。
- `CampaignLeaderboardService`：实时榜单、冻结快照、最终榜单。
- `CampaignRewardService`：奖励预览、最终结算、最低发放金额处理。
- `CampaignPayoutService`：一键发放、幂等、失败重试。
- `CampaignAuditService`：审核队列、风险状态、处罚记录。

## 6. 事件接入点

### 6.1 注册成功

触发活动邀请归因：

- 判断是否存在进行中活动。
- 判断注册来源是否包含邀请链接或邀请码。
- 记录活动邀请关系和门槛快照。
- 初始化邀请状态为已注册未充值。

### 6.2 支付或兑换成功

触发达标与入池：

- 判断订单支付成功时间或兑换成功时间是否发生在活动期内。
- 判断用户是否是活动被邀请用户。
- 排除赠送金额和管理员手动赠额。
- 累计有效充值。
- 达标后更新有效邀请状态。
- 按 10% 写入奖金池入池记录。

### 6.3 退款/冲正/撤销

触发扣减：

- 扣减有效充值金额。
- 扣减奖金池注入金额。
- 写入 `campaign_pool_deductions` 幂等扣减明细，并更新 `campaign_pool_entries.deducted_amount_cents` 聚合金额。
- 若低于门槛，则邀请失效。
- 更新排行榜与预计奖励。
- 若发生在发放后，生成追偿记录，不修改历史排行榜和原发放批次。
- 发放后追偿必须按来源事件生成幂等键，重复事件不得重复扣余额。

### 6.4 定时任务

- 活动开始：草稿/预热中转进行中。
- 活动结束：进行中转审核中并冻结榜单。
- 公示结束：可进入待发放。
- 数据刷新：每 1 至 5 分钟刷新排行榜和奖金池预估。

## 7. 测试计划

### 7.1 单元测试

- 有效邀请门槛判定。
- 门槛版本快照。
- 动态奖金池 10% 注入。
- 金额向下取整到分和尾差回收。
- Top10 权重计算。
- 平方根贡献奖励计算。
- 最低发放金额回收。
- 排行榜并列排序。
- 发放幂等键生成。
- 进行中活动唯一性校验。
- 奖励规则发布后冻结，门槛和注入比例按版本生效。
- 奖金池扣减明细幂等处理。
- 发放后追偿幂等扣回和余额快照。

### 7.2 集成测试

- 注册 → 充值达标 → 奖金池入池 → 排行榜更新。
- 活动结束 → 冻结榜单 → 审核通过 → 最终结算。
- 退款后有效邀请失效、奖金池扣减、排行榜重算。
- 多次部分退款生成多条扣减明细，聚合扣减金额正确。
- 一键发放到账户余额，重复发放不重复加余额。
- 发放部分失败后重试。
- 发放后退款生成追偿记录，原发放批次保持不变。
- 重复处理同一追偿记录不会重复扣余额。

### 7.3 前端测试

- 活动首页展示活动状态和奖金池。
- 我的数据展示预计奖励。
- 邀请记录状态流转。
- 排行榜固定展示当前用户排名。
- 后台奖励预览不触发真实发放。
- 发放确认弹窗和失败重试状态。

### 7.4 权限与审计测试

- 非管理员不能访问后台活动接口。
- 配置修改产生版本和审计记录。
- 奖金池调整必须记录原因。
- 用户级奖励调整必须记录原因、操作人和处理状态。
- 审核拒绝必须记录原因。
- 发放批次保存余额快照。
- 追偿审核记录操作人、原因、幂等键、余额流水 ID 和处理结果。

## 8. 风险与降级

- 若高级风控暂未实现，首期通过风险状态、人工审核和排除账号范围兜底。
- 若实时榜单刷新压力较高，先采用 1 至 5 分钟缓存刷新。
- 若一键发放失败，发放批次进入部分成功，并只允许重试失败明细。
- 若活动需要提前终止，保留已有数据并按已完成有效邀请合理结算。

## 9. 上线顺序

1. 数据表与服务骨架。
2. 活动配置和后台基础页面。
3. 注册、充值、退款事件接入。
4. 奖金池与实时排行榜。
5. 用户活动页。
6. 审核、冻结、最终结算。
7. 一键发放与审计。
8. 风控队列和异常处理增强。
