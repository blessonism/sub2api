# upstream relay group rate candidate mapping select upstream group

## Goal

上游分组倍率监控的候选映射编辑中，目标上游分组应支持从已有上游分组列表下拉选择，而不是只能手动填写 group id。体验应与本地账号选择保持一致，降低输错 group id 的概率。

## What I already know

- 用户明确指出：候选映射编辑应该也能选择上游分组，不应只是填写 group id。
- 当前仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，修改必须遵守下游 fork 工作流。
- 当前工作分支为 `fix/upstream-relay-candidate-group-select`，已有多处未提交变更，需要避免回退或覆盖无关改动。

## Assumptions

- 该问题主要位于前端上游中继监控/分组倍率监控候选映射编辑 UI。
- 后端若已有上游分组列表或候选项数据，应优先复用现有 API；若缺少必要字段，再做最小范围补齐。
- 本地账号已有下拉选择实现，应优先复用其组件/交互模式。

## Requirements

- 候选映射编辑中的上游分组字段提供下拉选择。
- 下拉选项来自真实的上游分组数据，展示用户可识别的名称/ID，提交时仍保存正确的 group id。
- 保留必要的手动输入或空值处理能力，避免已有数据无法显示或编辑。
- 不改变候选映射保存语义，不引入与当前任务无关的重构。

## Acceptance Criteria

- [ ] 打开候选映射编辑时，上游分组字段可下拉选择已有上游分组。
- [ ] 已存在的 group id 能正确回显到对应选项；未知 group id 仍可被识别/保留。
- [ ] 保存候选映射时提交的上游 group id 与用户选择一致。
- [ ] 前端相关测试覆盖上游分组选择行为。
- [ ] 针对性检查通过，且未覆盖当前工作区其他未提交变更。

## Definition of Done

- Tests added/updated where appropriate.
- Lint/typecheck/test command scoped to touched area when feasible.
- Trellis context includes downstream fork workflow and frontend spec entry.
- No unrelated files reverted or reformatted.

## Out of Scope

- 不调整上游分组倍率计算规则。
- 不重做整个监控工作台 UI。
- 不处理生产部署、提交、推送或数据库迁移。

## Technical Notes

- 必读 context：`.trellis/spec/guides/downstream-fork-workflow.md`。
- 前端规范入口：`.trellis/spec/frontend/index.md`。
- 后续实现前需定位候选映射编辑组件和上游分组数据来源。
