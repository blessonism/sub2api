# 邀请活动创建页排名权重表格化

## Goal

将后台“创建邀请活动”弹窗里的排名奖励配置从“排名奖励人数 + 逗号分隔权重”改为可增删行的表格，让管理员直接填写第 N 名对应权重，并用表格行数决定奖励前几名。

## What I Already Know

- 当前创建弹窗在 `CampaignRewardsView.vue` 使用 `rank_reward_count` 数字输入和 `rank_weights` 逗号字符串输入。
- 前端提交给后端的 API 仍为 `rank_reward_count` 与 `rank_weights` 数组。
- 后端配置、数据库和结算逻辑已经以 `rank_weights[rank-1]` 作为百分比权重，合计必须为 100。
- 用户已确认继续使用“百分比合计 100”的权重口径。

## Requirements

- 创建弹窗展示“名次 / 权重 / 操作”表格。
- 名次由行顺序自动生成，删除中间行后自动重排，不支持跳号。
- 默认仍为 10 行，权重为 `30,20,15,10,8,6,4,3,2,2`。
- 管理员可以增加或删除行，至少保留 1 行。
- 提交 API 不变：`rank_reward_count` 等于表格行数，`rank_weights` 等于每行权重数组。
- 不改活动编辑页、历史配置展示或已发布活动结算。

## Acceptance Criteria

- [ ] 默认创建弹窗渲染 10 个排名权重行。
- [ ] 删除或新增行后，校验与提交 payload 立即反映最新行数和权重。
- [ ] 权重必须为大于 0 的整数，且合计必须等于 100。
- [ ] 前端目标测试和类型检查通过。

## Definition of Done

- 更新相关前端实现、i18n 和视图测试。
- 保持后端接口、数据库和结算逻辑不变。
- 遵守下游 fork 工作流，任务 base branch 为 `custom/main`，工作分支为 `feature/invite-rank-weight-table`。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 frontend type-safety 指南和 code-reuse thinking guide。
