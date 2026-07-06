# upstream relay usage history finalization

## Goal

让上游分组倍率监控的历史用量同时满足两个口径：今天展示实时刷新值，历史日展示可重复复盘的日终定格值；历史区间 summary 必须由后端按完整筛选条件聚合，不能受前端当前分页影响。

## What I Already Know

- 方案文档已落盘在 `docs/UPSTREAM_RELAY_USAGE_HISTORY_FINALIZATION_CN.md`。
- 当前历史表唯一键是 `(usage_date, connector_id, upstream_group_id)`，刷新会覆盖 `checked_at`。
- 当前 `ListUsageHistory` 只返回分页 items，前端 summary 直接对当前页求和。
- 上游 `/api/v1/usage/stats` 支持任意 `start_date/end_date`，可以事后拉取历史日。
- 当前历史表对 connector 使用 `ON DELETE CASCADE`，删除 connector 会删除历史行。
- 现有路由前缀是 `/api/v1/admin/upstream-relay-group-monitors`。

## Requirements

- 新增 `finalized_at` 字段，历史日完成定格后写入该字段；今天保持 `finalized_at = NULL`。
- `FinalizeUsageForConnectorDate` 只能接受 Asia/Shanghai 下严格早于今天的日期。
- 定格时按本地 snapshot 维度写历史行；上游缺失的 group 写 0，而不是省略行。
- 自动兜底不能只依赖已有 pending 行，必须覆盖最近窗口内“缺失整天行”和“已有未定格行”。
- `ListUsageHistory` 返回完整筛选条件下的 summary，包含总成本、总 token、connector 数、group 数、latest checked 与 pending finalize 行数。
- 前端 summary 使用后端返回值；行级展示区分 live / finalized / pending finalize。
- 管理员可对历史日手动触发定格，endpoint 使用现有 upstream relay group monitors 前缀。
- 方案文档必须修正已发现的不准确点：migration 路径、connector 删除语义、catch-up 口径、手动 endpoint 路径和 today/future 校验。

## Acceptance Criteria

- [ ] 最近 7 天/30 天 summary 与数据库完整区间聚合一致，不受分页影响。
- [ ] 今天行 `finalized_at` 为空，刷新后 `checked_at` 推进。
- [ ] 昨天及更早定格行 `finalized_at` 非空，`checked_at` 为对应日期 Asia/Shanghai 的 23:59:59。
- [ ] 服务停机跨天后，下一次刷新会补齐最近窗口内缺失或未定格的历史日。
- [ ] 手动 finalize 拒绝 today/future，允许历史日。
- [ ] 前端历史日不再显示误导性的“最新采集时间”作为主文案。
- [ ] 新增/更新的后端和前端检查通过。

## Out of Scope

- 本任务不改变 connector 删除的外键级联行为；删除 connector 后历史一并删除，长期审计保留另开任务处理。
- 本任务不新增监控 policy 字段；日切定格按日期变化触发。
- 本任务不做生产数据库迁移执行或生产回补，只提供代码与接口能力。

## Technical Notes

- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`，本仓库业务二开基线为 `custom/main`。
- 后端主要影响 service/repository/handler/routes/runner/migration。
- 前端主要影响 `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`、`UpstreamRelayGroupMonitoringView.vue`、i18n 与相关测试。
- `response.Paginated` 结构固定，usage history 需要自定义带 `summary` 的响应结构或新增 response helper。
