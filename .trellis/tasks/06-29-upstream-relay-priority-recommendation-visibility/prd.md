# 上游倍率监控 priority 建议保留查看与失败详情

## Goal

管理员在上游倍率监控中生成 priority 建议后，即使已应用也仍能查看该次建议明细；当建议无法应用时，界面和接口应提供更具体、可操作的原因，避免只表现为建议消失或泛化失败。

## What I already know

- 用户明确希望已应用的 priority 建议仍可查看。
- 用户明确希望无法应用时有更详细提示。
- 当前分支已存在上游倍率监控 policy preview / recommendation 相关未提交改动，本任务需要在现有实现上小步修正。
- 本仓库是 Wei-Shaw/sub2api 下游二开，变更必须遵守 downstream fork 工作流。

## Requirements

- 已应用的 priority 建议必须继续可从管理端查看，包括 run 级别信息和建议明细。
- 应用动作完成后，前端不得因为状态变化隐藏该次建议详情入口。
- 对不可应用场景返回或展示具体原因，例如已应用、无待应用建议、旧值已变化、目标记录缺失或运行状态不允许。
- 保持生成/预览不写目标配置、应用才写配置的延迟应用语义。

## Acceptance Criteria

- [ ] 已应用 run 的 priority 建议详情接口仍返回建议明细。
- [ ] 前端在建议已应用后仍可打开并查看该次建议。
- [ ] 不可应用时返回/展示比通用失败更具体的提示。
- [ ] 相关后端与前端测试覆盖关键状态。

## Definition of Done

- Tests added/updated where behavior changes.
- Targeted backend/frontend checks pass or blockers are recorded.
- No unrelated user changes are reverted.
- Downstream fork workflow context is attached to implement/check context.

## Out of Scope

- 不重新设计整套上游监控策略生成算法。
- 不改动生产数据或执行生产迁移。
- 不提交、不推送。

## Technical Notes

- Relevant specs read: `.trellis/spec/guides/downstream-fork-workflow.md`, `.trellis/spec/backend/quality-guidelines.md`, `.trellis/spec/frontend/type-safety.md`.
- Backend guideline scenario applies: Admin delayed-apply suggestions.
