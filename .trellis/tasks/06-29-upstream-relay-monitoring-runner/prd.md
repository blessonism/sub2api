# fix: 上游中继自动监控后台 Runner

## Goal

实现真正的上游中继后台自动监控：管理员保存 `auto_sync_enabled` / `auto_probe_enabled` 后，服务端无需打开页面也会按策略执行上游倍率同步和候选探测。自动监控只负责采集与探测，不自动生成或应用 priority 建议。

## Requirements

- 新增 `UpstreamRelayMonitoringRunner`，每 30 秒读取一次 `GetMonitoringPolicy()`，动态感知开关和间隔变更。
- `auto_sync_enabled=true` 且到期时调用 `SyncAllConnectors()`；`auto_probe_enabled=true` 且到期时调用 `ProbeAllCandidates()`。
- 开关从 false 变 true 后，对应任务在 30 秒内触发首轮。
- 单轮同步/探测使用 20 分钟超时；成功后按对应 interval 安排下次执行，失败后按 `failure_retry_interval_minutes` 安排重试。
- 同步和探测各自防重入；多实例部署通过 `tryAcquireSingletonLeaderLock` 避免重复执行。
- Wire/provider/cleanup 接入 runner 生命周期。
- 更新前端自动监控文案，说明保存后后台会按间隔执行，priority 仍需手动确认。

## Acceptance Criteria

- [ ] 开启自动同步后，runner 首轮 tick 会调用一次 `SyncAllConnectors()`。
- [ ] 开启自动探测后，runner 首轮 tick 会调用一次 `ProbeAllCandidates()`。
- [ ] 两个开关关闭时不执行任务。
- [ ] 任务运行中再次 tick 不产生并发重入。
- [ ] 执行失败后，下次执行时间按失败重试间隔推进。
- [ ] leader lock 被其他实例持有时跳过执行。
- [ ] 前端监控页不再显示“当前阶段仅保存配置和支持手动触发”的过期说明。

## Definition of Done

- 后端相关 Go 测试通过。
- 前端 UpstreamRelayGroupMonitoringView 相关测试通过。
- 不新增数据库迁移，不修改 HTTP API。
- 遵守下游 fork 分支边界。

## Technical Approach

- 新增后台 runner 文件，复用现有 `LeaderLockCache` / `tryAcquireSingletonLeaderLock`。
- runner 依赖一个小接口，生产由 `UpstreamRelayGroupMonitoringService` 实现，单测使用 fake service。
- Provider 构造 runner 后注入 leader lock 并 `Start()`；应用 cleanup 调用 `Stop()`。
- 前端只改 i18n 文案，不改 API 类型。

## Decision (ADR-lite)

**Context**: 现有自动监控策略只保存配置，后台没有消费者，导致“启用自动同步/探测”不会自动运行。

**Decision**: 使用 30 秒 ticker + 动态读取策略 + leader lock 的后台 runner。首轮启用后 30 秒内触发，避免 handler 与 runner 之间增加事件耦合。

**Consequences**: 实现简单且符合现有后台服务模式；最坏情况下配置变更需要等待一个 tick 生效。

## Out of Scope

- 不自动生成、不自动应用 priority 建议。
- 不新增运行历史表或 UI 运行日志。
- 不调整现有同步/探测业务逻辑。

## Technical Notes

- 当前分支：`feature/upstream-relay-policy-preview`。
- 二开约束：`.trellis/spec/guides/downstream-fork-workflow.md`。
- 后端规范：`.trellis/spec/backend/quality-guidelines.md` 的 Admin upstream relay group monitoring 场景。
- 前端规范：`.trellis/spec/frontend/type-safety.md` 的 Upstream Relay Monitoring API Types 场景。
