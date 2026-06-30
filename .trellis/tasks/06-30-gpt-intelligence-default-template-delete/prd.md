# brainstorm: GPT 智力检验内置题也可删除

## Goal

管理员可以删除【GPT 智力检验】中的内置 3 道默认题；默认题只作为首次无配置时的种子，不应在管理员保存后被自动补回。

## Requirements

- 首次没有服务端全局配置时，仍展示默认 3 道题。
- 管理员保存后，题库完全以保存 payload 为准。
- 管理员可以删除内置题和自定义题。
- 允许保存为空题库，普通用户看到空题库状态。

## Acceptance Criteria

- [x] 后端保存 payload 缺少内置题时不会自动补回。
- [x] 后端允许保存空模板列表。
- [x] 前端内置题也展示删除按钮。
- [x] 前端空题库不崩溃，并可继续新增题目。
- [x] 聚焦测试通过。

## Out of Scope

- 不做删除确认弹窗或题目排序。
- 不执行 git commit / push。

## Technical Notes

- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
- 2026-06-30：`NormalizeGptIntelligencePromptTemplates` 改为只校验并保留 payload 内题目；无服务端配置时前端仍用默认 3 题，服务端返回空数组时显示空题库。
