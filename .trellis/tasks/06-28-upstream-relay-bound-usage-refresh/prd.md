# 上游中继今日用量按本地绑定刷新

## Goal

修正上游中继监控的今日用量刷新逻辑：不要用上游 `/api/v1/usage` 明细分页聚合，也不要让轻量刷新硬编码提示不可用；改为基于本地候选/账号绑定记录中的上游 API Key，调用上游普通用户 `/api/v1/usage/stats` 聚合接口，按本地 `connector_id + upstream_group_id` 写入今日用量快照。

## What I already know

- 上游普通用户 `/api/v1/usage/stats` 支持 `start_date`、`end_date`、`api_key_id`、`timezone`，与 Web 端 API Key + 时间范围统计口径一致。
- 当前 `RefreshConnectorMetrics` 只刷新余额，并硬编码 `账号维度绑定无法从本站日志精确刷新上游分组用量，请执行完整同步`，没有更新今日用量。
- 现有 `UpdateSnapshotTodayUsage` 可以只更新已有快照的 `today_actual_cost`、`today_total_tokens`、`today_usage_checked_at`。
- 本地候选/绑定记录已经包含账号和上游分组关系，应优先以本地绑定决定统计范围和分组归属。

## Requirements

- 轻量刷新连接器用量/余额时，同时刷新本地已绑定上游 API Key 的今日用量。
- 今日用量来源为上游普通用户 `/api/v1/usage/stats`，按本地绑定的上游 API Key ID 查询。
- 用量按本地绑定记录的 `upstream_group_id` 聚合，而不是依赖上游 API Key 列表枚举。
- 成功刷新时更新已有快照字段；没有匹配用量的已有快照应写入 0。
- 失败时保留已有今日用量快照，并返回清晰的 `usage_error`，避免显示 stale 数据被误认为新数据。
- 完整同步的今日用量也应尽量复用新口径，减少明细分页和 Web 端统计口径不一致。

## Acceptance Criteria

- [ ] 轻量刷新成功调用上游 `/api/v1/usage/stats` 并写入今日用量快照。
- [ ] 轻量刷新不调用上游 `/api/v1/usage` 明细分页接口。
- [ ] 多个本地绑定属于同一上游分组时，`total_actual_cost` 和 `total_tokens` 正确累加。
- [ ] 上游 stats 查询失败时，轻量刷新保留原有快照用量并返回 `usage_available=false` 与错误原因。
- [ ] 没有本地绑定或没有快照时，返回可解释的 usage 不可用原因。

## Definition of Done

- 后端服务测试覆盖成功聚合、失败保留、无绑定场景。
- 相关 repository/service 接口保持最小变更。
- 聚焦 Go 测试通过。

## Out of Scope

- 不新增管理员接口。
- 不修改生产数据或执行迁移。
- 不改变前端页面布局。

## Technical Notes

- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 相关文件：
  - `backend/internal/service/upstream_relay_group_monitoring.go`
  - `backend/internal/repository/upstream_relay_group_monitoring_repo.go`
  - `backend/internal/service/upstream_relay_group_monitoring_test.go`
  - `backend/internal/repository/upstream_relay_group_monitoring_repo_test.go`
