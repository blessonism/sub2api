# Lottery Threshold Progress Bar

## Goal

在用户侧 Token 抽奖活动的“达标门槛”展示中补充进度条，让用户在看到门槛值时能直接判断当前完成比例。

## Requirements

- 用户侧抽奖活动的达标门槛卡片展示小型进度条。
- 进度条复用现有 `progressPct` 计算，封顶 100%，threshold 不可用时为 0%。
- 进度条提供 `role="progressbar"`、`aria-valuenow` 等可测试和可访问属性。
- 不修改后端 API、抽奖资格计算或 million 单位格式化规则。

## Acceptance Criteria

- [x] 今日 0.18M / 门槛 1.25M 时，门槛卡片进度为 14%。
- [x] 今日 tokens 超过门槛时，进度条封顶 100%。
- [x] 相关用户侧测试通过。

## Definition of Done

- 更新用户侧活动测试覆盖新增进度条。
- 相关 vitest 通过。
- `pnpm --dir frontend typecheck` 通过。

## Technical Notes

- 当前仓库已有未提交活动中心改动，本任务只追加用户侧抽奖活动进度条。
- 已读取下游 fork 工作流、frontend type-safety 与 thinking guides。

## Verification

- `pnpm --dir frontend test:run src/views/user/__tests__/CampaignRewardsView.spec.ts`
- `pnpm --dir frontend typecheck`
