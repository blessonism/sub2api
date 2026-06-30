# brainstorm: GPT 智力检验管理员新增题可删除

## Goal

在【GPT 智力检验】全局模板已经服务端共享的基础上，允许管理员新增自定义测试题，并删除管理员新增的测试题；普通用户能看到保存后的全局题目列表。

## What I already know

- 上一轮已把模板从浏览器本地草稿改为 DB-backed 全局设置。
- 用户追加要求：管理员能够删除新增测试题。
- 为避免误删基础能力，内置测试题应保留，只允许编辑/重置，不允许删除。

## Requirements

- 管理员可新增测试题。
- 管理员可删除新增测试题。
- 内置测试题不可删除，但仍可编辑和重置。
- 用户端读取同一份服务端全局题目列表。
- 保存接口应规范化模板并拒绝无效题目。

## Acceptance Criteria

- [x] 管理员新增题保存后出现在普通用户模板列表中。
- [x] 管理员新增题可删除并保存。
- [x] 内置题不会因保存 payload 缺失而被删除。
- [x] 聚焦前后端测试通过。

## Out of Scope

- 不做复杂题库分类、拖拽排序或独立 CRUD 页面。
- 不执行 git commit / push。

## Technical Notes

- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
- 2026-06-30：后端 `NormalizeGptIntelligencePromptTemplates` 现在会固定补齐内置题，并保留 payload 中的自定义题；前端管理员面板新增“新增测试题 / 删除测试题”，删除按钮仅对自定义题展示。
