# brainstorm: 上游倍率监控历史用量持久化

## Goal

管理员在“上游倍率监控”中不仅能看到当前今日用量快照，还能回看历史日期的上游分组用量，避免过了当天或下一次刷新后只能看到最新快照、无法追踪趋势和异常。

## What I already know

- 用户希望“今日用量”能够持久化，不只是当天能看到。
- 现有候选行与快照行暴露 `today_actual_cost`、`today_total_tokens`、`today_usage_checked_at`。
- 当前 `upstream_relay_group_rate_snapshots` 以 `connector_id + upstream_group_id` 为唯一快照，只保存当前今日用量字段，会被同步或轻量刷新覆盖。
- 现有顶部“今日用量”卡片任务明确不新增后端聚合接口，前端通过快照汇总展示。
- 轻量刷新链路当前会调用 `RefreshConnectorMetrics`，再写入 `UpdateSnapshotTodayUsage`。
- 当前分支已有上游中继监控 runner、策略预览等未提交改动，本任务应避免覆盖无关变更。

## Assumptions (temporary)

- 历史数据应以本地监控系统的刷新结果为事实源，而不是每次打开页面临时重新扫远端。
- 历史数据应保留未知状态：刷新失败或数据不可用时不能把未知强行记为 0。
- 第一版优先支持管理端查看历史日期/趋势，不追求复杂 BI 分析。

## Open Questions

- 已确认：历史粒度采用“每天每连接器每上游分组一条最终日汇总”，不保存每次刷新采样。

## Requirements (evolving)

- 持久化上游倍率监控用量历史，至少包含日期、connector、upstream_group_id、actual_cost、total_tokens、checked_at/采集时间。
- 历史表按 `usage_date + connector_id + upstream_group_id` 保持唯一，每次成功刷新覆盖当天该分组最终汇总。
- 当前今日用量快照仍保留，用于现有候选表和概览卡即时展示。
- 历史写入应接入现有刷新链路，避免 UI 打开时才生成历史。
- 历史查询应能区分无数据、刷新失败、真实 0 用量。
- 历史展示应支持查看之前日期的数据。

## Acceptance Criteria (evolving)

- [ ] 刷新今日用量后，历史表/历史记录中能查到对应日期的数据。
- [ ] 同一天多次刷新不会产生多条采样记录，而是更新当天最终汇总。
- [ ] 第二天仍能查看前一天的上游用量数据。
- [ ] 无用量的已成功刷新分组记录为 0；刷新失败或未知不记录为 0。
- [ ] 前端能从管理页入口查看历史日期的数据。
- [ ] 后端测试覆盖历史写入、查询、未知与 0 的区别。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green for touched areas
- Docs/spec updated if behavior changes
- Rollout/rollback considered if migration is added

## Out of Scope (explicit)

- 不在第一版做自动报表、告警或长期趋势预测。
- 不改变现有上游账号余额字段语义。
- 不把 connector 账号余额混入 candidate/group usage 历史。

## Technical Notes

- Branch: `feature/upstream-relay-policy-preview`
- Downstream fork guide: `.trellis/spec/guides/downstream-fork-workflow.md`
- Backend spec: `.trellis/spec/backend/quality-guidelines.md` 的 `Admin upstream relay group monitoring` 场景
- Frontend spec: `.trellis/spec/frontend/type-safety.md` 的 `Upstream Relay Monitoring API Types` 场景
- Current snapshot table: `upstream_relay_group_rate_snapshots`
- Current usage snapshot migration: `backend/migrations/171_upstream_relay_group_usage_snapshot.sql`
- Current write paths: `UpsertSnapshots`, `UpdateSnapshotTodayUsage`
- Current refresh paths: `SyncConnector`, `RefreshConnectorMetrics`
