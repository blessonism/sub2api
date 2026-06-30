# brainstorm: 上游分组倍率监控建议列表过滤

## Goal

修正上游分组倍率监控里的 priority 建议列表体验：管理员不能只能看到最近 20 条建议，应能继续查看更早记录；同时提供一键过滤能力，只查看存在实际建议的记录，隐藏无建议/无需调整项。

## What I Already Know

- 用户反馈当前 priority 建议只显示最近 20 条，历史建议不可见。
- 用户希望可以一键过滤无建议项。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开，变更必须遵守 `custom/main` 基线和 downstream fork 工作流。
- 相关前端规范要求 `frontend/src/api/admin/upstreamRelayGroupMonitors.ts` 与后端 JSON 字段保持一致，并补齐中英文 i18n key。

## Assumptions

- “无建议”指后端返回的建议记录中没有实际 priority 变更/应用价值的行，而不是删除历史记录。
- 初始仍可保留 20 条加载量以控制页面性能，但必须提供清晰的继续查看入口。

## Requirements

- 管理员能从上游分组倍率监控页面查看超过最近 20 条的 priority 建议历史。
- 管理员能一键切换为仅显示有实际建议的记录。
- 过滤不应破坏生成、预览、应用建议等既有操作。
- 若新增或调整 API 参数，前端类型、调用方和测试需同步。
- 推荐历史 UI 应保持后台工作台风格：过滤开关不要加外框，位置应和摘要信息自然组合。
- 应用建议弹窗不需要额外 checkbox 二次确认，管理员点击“确认应用”即可执行。
- 应用建议弹窗需要提供“关闭建议”入口，复用现有关闭/删除建议 run 的确认流程。
- 已关闭的 priority 建议不应因局部更新被移动到列表最前，关闭、恢复、应用都应保留当前列表行位置。
- 已关闭的 priority 建议可以恢复；恢复后若仍是成功运行、未应用且存在建议，应重新显示“查看并应用/关闭建议”。

## Acceptance Criteria

- [x] priority 建议列表支持查看第 20 条之后的历史记录。
- [x] 页面有明确的一键过滤控件，只显示有建议项。
- [x] 无建议过滤状态下，空结果有合理展示，不误报接口错误。
- [x] 推荐历史过滤开关去除外框并调整到摘要工具行。
- [x] 应用建议弹窗移除“我确认要应用...”二次确认。
- [x] 应用建议弹窗提供“关闭建议”入口。
- [x] 关闭、恢复、应用建议时前端局部更新不改变当前列表排序位置。
- [x] 已关闭建议支持恢复，恢复不删除 suggestions、不修改账号 priority。
- [x] 聚焦测试或类型检查覆盖受影响契约。

## Definition of Done

- 代码实现遵循现有前后端模式。
- 已更新必要 i18n 文案。
- 已运行与变更范围匹配的聚焦验证。
- 若发现可沉淀规范的新约束，更新 `.trellis/spec/`。

## Out of Scope

- 重做整套上游监控工作台布局。
- 改变建议生成算法本身。
- 自动应用建议或修改生产配置。

## Technical Notes

- 必读上下文：`.trellis/spec/guides/downstream-fork-workflow.md`。
- 计划优先定位 `UpstreamRelayGroupMonitoringView.vue`、`upstreamRelayGroupMonitors.ts`、后端 recommendation handler/service/repository。
- 实现采用后端 `has_suggestions` 查询参数过滤 `suggestion_count > 0`，避免仅前端隐藏导致分页第一页为空但后续仍有建议。
- 前端保留每页 20 条默认加载量，增加上一页/下一页控制以查看历史记录。
- UI polish：推荐历史顶部改为摘要 chip + 无外框 toggle；分页底栏展示当前范围；空状态改为标题 + 说明。
- 应用弹窗：移除确认 checkbox；“关闭建议”是软关闭，只标记 run 不再作为待处理建议展示，保留历史与建议明细可继续查看，不复用删除流程。
- 恢复弹窗/列表：`POST /recommendations/:id/restore` 只清除 run 的 `closed/closed_by/closed_at`，保留 suggestions，不触碰 `accounts.priority`。
- 前端局部更新：close/restore/apply 对已在当前列表中的 run 原位替换；只有新生成或当前页不存在的 run 才按后端排序 `created_at DESC, id DESC` 插入。
- 验证：前端 API/视图测试通过，`pnpm typecheck` 通过；后端上游推荐相关 repository 测试通过。完整 `go test ./internal/repository ./internal/handler/admin` 中 `internal/repository` 仍存在既有 `usage_log_repo_request_type_test.go` 参数数量/扫描数量失败，和本任务改动无关。
- 追加验证：`pnpm exec vitest run "src/views/admin/__tests__/UpstreamRelayGroupMonitoringView.spec.ts" "src/i18n/__tests__/upstreamRelayMonitoringLocales.spec.ts"` 通过；`pnpm typecheck` 通过；相关前端文件 `git diff --check` 通过。
