# brainstorm: GPT 智力检验题管理员编辑后全员可见

## Goal

管理员在后台编辑【GPT 智力检验】检测题后，该题应保持或成为面向所有用户开放可见，避免编辑操作意外把题目变成仅管理员或局部用户可见。

## What I already know

- 用户明确要求：检测题管理员编辑后应该对所有用户开放可见。
- 当前仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，变更必须遵守下游 fork 边界。
- 当前分支为 `custom/main`，工作区已有多处未提交改动；本任务只修改本需求相关文件。

## Assumptions (temporary)

- 【GPT 智力检验】检测题已有管理员编辑入口与普通用户可见列表入口。
- 问题更可能在后端保存或查询过滤条件中，而不是纯展示文案。

## Open Questions

- 暂无阻塞问题；优先通过代码定位现有行为。

## Requirements (evolving)

- 管理员编辑检测题时，题目应对所有用户开放可见。
- 不能引入只对单个用户、管理员或局部范围可见的保存结果。
- 保持现有编辑能力与字段校验不回退。

## Acceptance Criteria

- [x] 管理员编辑后的检测题可被普通用户题目列表或检测流程获取。
- [x] 相关后端测试覆盖编辑后可见性。
- [x] 聚焦测试通过。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green for touched scope
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 不重做 GPT 智力检验整体产品流程。
- 不调整 unrelated 上游中转、排行榜等现有未提交改动。
- 不执行 git commit / push。

## Technical Notes

- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
- 2026-06-30：将 GPT 智力检验模板从浏览器本地草稿改为 DB-backed 全局设置，用户快照接口返回 `intelligence_check_templates`，管理员保存接口为 `PUT /api/v1/admin/settings/gpt-intelligence/templates`。
