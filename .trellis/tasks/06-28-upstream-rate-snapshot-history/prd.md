# brainstorm: 上游倍率快照变更流水

## Goal

在管理端上游中转站分组倍率监控中新增“倍率快照”tab，用于展示系统自动同步上游分组倍率时产生的历史变更流水。核心目标是让管理员在上游中转站调整分组费率后，可以追溯“哪个上游、哪个分组、什么时候从多少变成多少”，并能看到新增分组与移除分组事件。

## What I Already Know

- 用户确认快照由系统同步上游分组倍率时自动生成，不需要管理员手动保存。
- 用户确认只有倍率发生变化时才留一条变更记录，同步结果完全一致时不写历史流水。
- 用户确认 tab 主要给管理员查看历史变更流水，暂不要求恢复、回滚、导出或账单重算。
- 用户确认上游分组新增、移除也需要纳入历史流水。
- 用户确认首次同步某个上游连接器时只记录当前态基线，不把所有分组批量写成“新增分组”历史流水；后续同步再记录变化。
- 用户希望同一个账号，也就是同一个上游中转连接器下的分组变更能够聚合在一起展示，避免全局流水过于分散导致难查找。
- 现有 `upstream_relay_group_rate_snapshots` 是当前态表，按 `(connector_id, upstream_group_id)` 唯一约束 upsert，适合展示当前可用/过期分组，不适合保留历史变更流水。
- 现有同步入口 `SyncConnector` 会拉取 `/api/v1/groups/available` 和 `/api/v1/groups/rates`，生成当前态 `UpstreamRelayGroupRateSnapshot` 后调用仓储 upsert。
- 现有缺失分组处理是把当前态快照标记为 `stale`，没有记录“移除分组”历史事件。
- 管理端页面 `UpstreamRelayGroupMonitoringView.vue` 已有顶部 section tab，当前包括候选、连接器、推荐建议等；连接器列表里已有“当前快照”弹窗。

## Assumptions

- 新增历史流水应独立于当前态快照表，例如新增 `upstream_relay_group_rate_snapshot_changes` 或语义更清晰的历史表。
- 历史流水的比较基准应是同步前已保存的当前态快照，而不是前端临时状态。
- 倍率变化以 `final_rate_multiplier` 为主，因为它代表当前用于决策的最终倍率；`default_rate_multiplier`、`override_rate_multiplier`、`source` 可作为辅助字段记录。
- “移除分组”可以由本次同步结果缺少但当前态表仍存在的分组推导，事件类型为 `removed`。
- “新增分组”可以由本次同步结果新增当前态表中不存在的分组推导，事件类型为 `added`。
- 首次同步的基线判定可以通过同步前是否存在该连接器的当前态快照来判断；同步前为空时仅 upsert 当前态，不写历史流水。

## Requirements

- 新增管理端“倍率快照”tab，展示上游分组倍率历史变更流水。
- 同步上游分组倍率时自动比对同步前后的分组集合和倍率。
- 首次同步某个上游连接器时只建立当前态基线，不写新增分组流水。
- 仅在以下情况写入历史流水：
  - 新增分组；
  - 移除分组；
  - 已存在分组的最终倍率发生变化。
- 同步结果未变化时不写入流水，避免噪音。
- 历史 tab 默认按上游中转连接器聚合展示，同一个连接器下的分组变更应放在同一块区域内。
- 历史流水至少展示：
  - 上游中转站名称或 ID；
  - 上游分组 ID；
  - 分组名称；
  - 平台；
  - 事件类型：新增、移除、倍率变化；
  - 变更前倍率；
  - 变更后倍率；
  - 变化方向或差值；
  - 来源；
  - 变更时间。
- 管理端 tab 支持按上游、分组关键词、事件类型筛选，并按变更时间倒序展示。
- 管理端 tab 应优先方便从某个上游连接器进入查看它下属分组的变更，避免只提供全局扁平列表。
- 上涨、下降、新增、移除应有清晰视觉标识。

## Acceptance Criteria

- [ ] 当同步发现某个分组 `final_rate_multiplier` 从旧值变为新值时，后端写入一条历史流水。
- [ ] 当某个上游连接器首次同步时，后端只建立当前态基线，不写新增分组流水。
- [ ] 当非首次同步发现新上游分组时，后端写入一条新增分组流水。
- [ ] 当同步发现已有上游分组在本次结果中消失时，后端写入一条移除分组流水。
- [ ] 当同步结果与当前态完全一致时，不新增历史流水。
- [ ] 管理端“倍率快照”tab 可查看历史流水，并能区分新增、移除、上涨、下降。
- [ ] 管理端“倍率快照”tab 能把同一个上游连接器下的分组变更聚合展示，管理员可以先定位连接器，再查看其分组变更。
- [ ] 当前态快照弹窗仍用于查看某个连接器的最新分组倍率，不被历史流水 tab 替代。
- [ ] 变更流水接口具备分页，避免历史数据增长后一次性拉取过多记录。
- [ ] 后端同步逻辑和列表接口有针对新增、移除、倍率变化、不变四类场景的测试覆盖。

## Definition of Done

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope

- 不做历史快照恢复或回滚。
- 不做历史流水导出。
- 不做账单重算或历史成本校准。
- 不做自动调价策略。
- 不改动当前态快照弹窗的核心用途。

## Technical Notes

- 下游二开分支边界遵循 `.trellis/spec/guides/downstream-fork-workflow.md`，任务 base branch 已设置为 `custom/main`，工作分支已设置为 `feature/upstream-rate-snapshot-history`。
- 当前态表来自 `backend/migrations/164_upstream_relay_group_monitoring.sql`，表名为 `upstream_relay_group_rate_snapshots`，存在唯一约束 `uq_upstream_relay_snapshot_connector_group`。
- 当前态 upsert 位于 `backend/internal/repository/upstream_relay_group_monitoring_repo.go` 的 `UpsertSnapshots`。
- 同步拉取与快照构造位于 `backend/internal/service/upstream_relay_group_monitoring.go` 的 `fetchGroupSnapshots` 和 `SyncConnector`。
- 管理端 API 位于 `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`，当前已有 `syncConnector` 和 `listSnapshots`。
- 管理端页面位于 `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`，当前已有 section tab 与当前快照弹窗。

## Open Questions

- 无阻塞问题；后续实现阶段可根据现有页面结构选择“连接器分组折叠列表”或“左侧连接器筛选 + 右侧变更流水”的具体 UI。
