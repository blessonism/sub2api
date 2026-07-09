# Lottery Threshold Million Unit

## Goal

让抽奖活动的达标门槛与排行榜的 Token 展示单位保持一致：界面使用 million / M 表达，后端和 API 仍保存原始 token 数，避免历史数据迁移和契约漂移。

## Requirements

- 抽奖活动用户侧的今日 Token 和达标门槛展示为 `x.xxM`。
- 抽奖活动管理侧的达标门槛和阶梯步长输入使用 million 作为单位，提交时转换回原始 token 数。
- 编辑已有抽奖活动时，把后端返回的原始 token 数转换为 million 值回填表单。
- 活动详情指标展示与排行榜一致使用 `M` 后缀。

## Acceptance Criteria

- [x] 创建/编辑抽奖活动时，输入 `1.25` 会提交 `threshold_tokens: 1250000`。
- [x] 用户侧抽奖活动中 `1250000` tokens 展示为 `1.25M`。
- [x] 既有 API 字段名不变，后端无需迁移。

## Definition of Done

- 相关前端测试覆盖 million 单位显示与提交转换。
- 窄范围测试通过。

## Verification

- `pnpm --dir frontend test:run src/components/admin/activities/__tests__/LotteryCampaignAdminPanel.spec.ts src/views/user/__tests__/CampaignRewardsView.spec.ts src/views/user/__tests__/LeaderboardView.spec.ts`
- `pnpm --dir frontend typecheck`

## Technical Approach

复用 `frontend/src/utils/usagePricing.ts` 中的 `TOKENS_PER_MILLION` 常量；只在前端显示和表单层转换单位，保持 API payload 与后端存储仍为原始 token 数。

## Out of Scope

- 不改后端数据库字段和 API JSON 字段名。
- 不调整抽奖资格计算逻辑。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，当前分支为 `feature/user-activity-center`，未切换分支以避免影响已有未提交变更。
- 已读取 backend quality、frontend type-safety、code-reuse、cross-layer 指南。
