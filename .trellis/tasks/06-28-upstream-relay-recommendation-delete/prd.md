# brainstorm: 删除上游倍率优先级建议

## Goal

管理员需要在上游倍率监控页面删除不再需要的优先级建议批次，避免无效或过期建议长期停留在待处理列表里。

## What I already know

- 现有建议以 `upstream_relay_recommendation_runs` 和 `upstream_relay_recommendation_suggestions` 持久化。
- 前端列表展示 Run，并通过详情弹窗应用整次 Run。
- 已应用 Run 带有审计意义，不应随意删除。

## Assumptions

- 本任务删除的是整次建议 Run，而不是单条 suggestion。
- 仅允许删除未应用 Run；已应用 Run 保留审计记录。

## Requirements

- 在建议列表行增加删除按钮。
- 删除前使用确认弹窗。
- 后端提供删除 Run 接口。
- 删除 Run 时同步删除该 Run 下的 suggestions。
- 已应用 Run 删除应返回冲突错误。

## Acceptance Criteria

- [ ] 未应用 Run 可以从列表删除。
- [ ] 删除后列表和待应用建议计数更新。
- [ ] 已应用 Run 不能删除。
- [ ] 后端仓储测试覆盖未应用删除和已应用拒绝。
- [ ] 前端 typecheck 通过。

## Definition of Done

- Tests added/updated where meaningful.
- Typecheck / targeted Go tests pass.
- Downstream fork boundary respected.

## Out of Scope

- 删除单条 suggestion。
- 恢复已删除建议 Run。
- 删除已应用审计历史。

## Technical Notes

- 相关前端文件：`frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`、`frontend/src/api/admin/upstreamRelayGroupMonitors.ts`。
- 相关后端文件：`backend/internal/handler/admin/upstream_relay_group_monitoring_handler.go`、`backend/internal/service/upstream_relay_group_monitoring.go`、`backend/internal/repository/upstream_relay_group_monitoring_repo.go`、`backend/internal/server/routes/admin.go`。
- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
