# fix: 上游候选 group id 支持选择

## Goal

上游 Relay 候选映射的新建/编辑弹窗中，管理员应能从已同步的上游分组快照里选择 `upstream_group_id`，不用只能手动复制填写；同时保留手动输入能力，覆盖快照未同步或临时分组的场景。

## What I already know

- 用户反馈“新建候选的上游 group id 选不了，只能自己填”。
- 当前候选表单位于 `UpstreamRelayGroupMonitoringView.vue`，`upstream_group_id` 是纯文本输入框。
- 页面已经加载 `overviewSnapshots` 和连接器快照列表，包含 `upstream_group_id`、`name`、`platform`、`final_rate_multiplier` 等可用于展示的字段。
- 规范要求候选绑定保持 account-scoped，不引入 `target_group_id`。

## Assumptions

- 本轮只修复前端交互，不改变后端 DTO 或存储契约。
- 分组选项来自已同步快照；若没有快照，管理员仍可手动输入。

## Requirements

- 候选表单按当前连接器展示已同步上游分组选项。
- 选择某个上游分组后自动填入 `candidateForm.upstream_group_id`。
- 保留手动输入框，支持未同步分组。
- 切换连接器时清空旧的上游分组选择，避免误提交旧连接器的 group id。

## Acceptance Criteria

- [x] 新建候选时可以从当前连接器的快照分组中选择 group id。
- [x] 无快照或需要特殊值时仍可手动填写 group id。
- [x] 切换连接器不会保留上一个连接器的 group id。
- [x] 前端类型检查通过。

## Out of Scope

- 新增后端接口或修改候选 DTO。
- 改变候选去重、保存、探测或推荐策略。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 `.trellis/spec/frontend/type-safety.md` 中的上游 Relay 监控 API 类型约束。
- 已读取 `.trellis/spec/backend/quality-guidelines.md` 中的 Admin upstream relay group monitoring 场景。
