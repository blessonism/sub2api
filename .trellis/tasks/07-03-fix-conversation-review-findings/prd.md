# Fix Conversation Review Findings

## Goal

修复上一轮 code review 发现的两个前端回归：对话历史页测试缺少必要依赖注入，以及对话详情页菜单可能被点击外部遮罩覆盖。

## Requirements

- `ConversationHistoryView` 单测应显式提供 Pinia 与 router mock，避免组件新增 store/router 依赖后测试环境崩溃。
- `ConversationDetailView` 打开菜单后，菜单项应可点击，点击菜单外部仍可关闭菜单。
- 修复范围限定在 review findings，不调整业务接口与页面功能。

## Acceptance Criteria

- [ ] `ConversationHistoryView.spec.ts` 相关测试通过。
- [ ] 前端类型检查通过。
- [ ] 对话详情页 header 菜单层级高于点击外部遮罩。

## Definition of Done

- 针对性前端测试通过。
- `pnpm --dir frontend typecheck` 通过。
- 遵守下游 fork 分支边界。

## Technical Approach

- 测试侧复用项目既有 `createPinia` / `setActivePinia` 模式，并 mock `vue-router` 的 `useRouter().push`。
- 视图侧调整 `ConversationDetailView` 的 header、菜单和遮罩 z-index，让透明遮罩只覆盖内容区域，不覆盖菜单。

## Out of Scope

- 不新增对话详情页功能。
- 不修复本轮 review 外的既有 Go 仓储测试失败。
- 不提交、不推送。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 `.trellis/spec/frontend/index.md` 与 `.trellis/spec/frontend/type-safety.md`。
