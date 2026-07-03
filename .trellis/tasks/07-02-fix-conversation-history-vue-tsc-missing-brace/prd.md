# fix conversation history vue tsc missing brace

## Goal

修复 `frontend/src/views/admin/ConversationHistoryView.vue` 中导致 `vue-tsc` 报 `'}' expected` 的语法错误，让前端类型检查能够继续执行到后续阶段。

## What I already know

- 用户提供的报错位置为 `ConversationHistoryView.vue:948:1`。
- 当前仓库存在未提交改动，修复必须最小化，不能回退用户已有改动。
- 本仓库是下游二开仓库，修改需遵守 downstream fork 工作流。

## Requirements

- 定位缺失或多余的语法结构。
- 只修复该语法错误，不做无关重构。
- 运行针对性 `vue-tsc --noEmit` 验证，若出现后续既有错误需明确说明。

## Acceptance Criteria

- [ ] `ConversationHistoryView.vue` 不再在 948 行附近报 `'}' expected`。
- [ ] 修复范围最小，未覆盖其他工作区改动。
- [ ] 前端类型检查至少越过该语法错误。

## Out of Scope

- 不修复与该语法错误无关的业务逻辑或类型问题。
- 不提交、不推送、不改数据库。
