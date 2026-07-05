# 管理员 token 排行榜用户邮箱打开记录弹窗

## Goal

让管理员侧 token 排行榜中“用户”列的邮箱可点击，并复用用户管理页面点击余额列时展示的“用户充值和并发变动记录”弹窗，便于运营从 token 用量排行直接追溯该用户的充值、并发、订阅等变动记录。

## What I already know

- 用户希望 token 排行榜管理员侧点击用户邮箱时，行为与用户管理余额列一致。
- 用户管理页通过 `UserBalanceHistoryModal` 展示用户充值和并发变动记录。
- token 排行榜管理员页 `TokenLeaderboardView.vue` 的 ranking 行已有 `user_id`、`email`、`username` 等字段，但没有完整 `AdminUser` 明细。
- 管理员用户 API 已有 `adminAPI.users.getById(id, includeDeleted)`，可在点击时按 `user_id` 拉取完整用户信息。

## Requirements

- 管理员 token 排行榜的用户列邮箱应渲染为可点击按钮。
- 点击后应按排行榜行的 `user_id` 拉取完整用户信息，并打开 `UserBalanceHistoryModal`。
- 弹窗默认复用现有记录列表、类型筛选、分页和展示文案。
- 从排行榜打开弹窗时不展示充值/扣款动作按钮，避免在排行榜上下文引入额外写操作。
- 拉取用户失败时应展示管理员页面已有错误提示，不打开空弹窗。

## Acceptance Criteria

- [ ] 点击排行榜用户标识会调用 `adminAPI.users.getById(row.user_id, true)`。
- [ ] 用户详情加载成功后展示 `UserBalanceHistoryModal`，并传入对应用户。
- [ ] 加载失败时通过 `appStore.showError` 展示失败提示。
- [ ] 现有用户管理余额列弹窗逻辑不被修改或破坏。
- [ ] 前端测试覆盖排行榜邮箱点击打开记录弹窗路径。

## Definition of Done

- 测试添加或更新，覆盖新增点击路径。
- 相关前端类型检查保持通过。
- 不新增后端接口，不复制弹窗内部逻辑。

## Out of Scope

- 不调整用户管理页面余额列现有行为。
- 不新增充值/扣款操作入口。
- 不改变排行榜接口返回结构。

## Technical Notes

- 遵守下游 fork 工作流：`.trellis/spec/guides/downstream-fork-workflow.md`。
- 复用组件：`frontend/src/components/admin/user/UserBalanceHistoryModal.vue`。
- 修改目标：`frontend/src/views/admin/TokenLeaderboardView.vue` 与对应测试。
