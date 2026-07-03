# 手动拉取倍率快照

## Goal

在上游分组倍率监控页提供清晰的手动拉取倍率快照入口，方便管理员无需等待自动同步即可更新上游分组倍率快照。

## Requirements

- 复用现有连接器快照同步能力，不新增后端协议。
- 在页面主要数据操作区提供“拉取倍率快照”按钮，用于手动拉取全部连接器快照。
- 在单连接器快照弹窗内提供当前连接器的手动拉取按钮。
- 操作期间显示禁用/进行中文案，避免重复触发。
- 保持现有未提交改动，不回滚连接器外链与历史用量列调整。

## Acceptance Criteria

- [ ] 顶部数据操作区能手动触发全部连接器快照同步。
- [ ] 快照弹窗内能手动触发当前连接器快照同步。
- [ ] 同步完成后快照列表、连接器最近同步时间和快照变更列表按现有逻辑刷新。
- [ ] 中英文文案完整。
- [ ] 针对性前端测试通过。

## Definition of Done

- Tests added/updated where appropriate.
- Lint / typecheck / targeted test green where feasible.
- Downstream fork workflow respected.

## Technical Approach

现有后端已提供 `POST /admin/upstream-relay-group-monitors/connectors/:id/sync` 与 `POST /connectors/sync-all`，服务层会拉取上游可见分组倍率并写入 `upstream_relay_group_rate_snapshots`。本任务只补前端入口和文案，继续调用 `syncConnector()` / `syncAllConnectors()`。

## Decision (ADR-lite)

**Context**: 用户希望“倍率快照”有按钮可以手动拉取。页面已有行内“同步”和监控页“立即同步全部连接器”，但语义不够直观，快照弹窗也没有就地拉取入口。

**Decision**: 复用现有 sync API，把相关按钮命名为“拉取倍率快照”，并在快照弹窗顶部增加当前连接器拉取按钮。

**Consequences**: 不增加后端维护成本；按钮语义更清楚。后续如果需要只拉取变更或异步任务进度，可再扩展 sync 返回结构。

## Out of Scope

- 不新增后端接口。
- 不改变自动监控调度逻辑。
- 不调整快照差异计算规则。

## Technical Notes

- 遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 涉及文件：`frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`、`frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`、相关视图测试。
