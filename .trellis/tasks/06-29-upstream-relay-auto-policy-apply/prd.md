# brainstorm: 上游倍率监控自动策略应用

## Goal

在现有上游中继监控工作台基础上，把“后台自动采集/探测 + 推荐策略预览 + 手动生成/应用建议”的半自动流程，升级为可配置的自动策略闭环：系统按监控策略定时刷新数据，按推荐策略生成建议 run，并在满足安全门条件时自动应用 priority 建议，减少日常人工运营。

## What I already know

- 用户希望实现“上游倍率监控自动生成策略，然后自动应用，实现完全自动化运营”。
- 用户指出前一次理解过于抽象，必须结合当前项目已有能力作为基础。
- 当前分支已有 `UpstreamRelayMonitoringRunner` 未提交实现，负责按 `auto_sync_enabled` / `auto_probe_enabled` 后台执行 `SyncAllConnectors()` 和 `ProbeAllCandidates()`。
- 现有 runner 任务明确把“自动生成或应用 priority 建议”列为 Out of Scope。
- 当前已有推荐策略配置与预览：
  - `GetRecommendationPolicy` / `UpdateRecommendationPolicy`
  - `PreviewRecommendations`
  - `GenerateRecommendations`
  - `ApplyRecommendationRun`
- 当前 `ApplyRecommendationRun` 已经事务化更新 `accounts.priority`，包含 run 状态检查、重复应用检查、旧 priority stale check、suggestion applied 标记、run applied 标记和 scheduler outbox。
- 当前前端工作台已有：
  - 监控策略区：自动同步、自动探测、间隔、失败重试等。
  - 推荐策略区：阈值、排序、priority 起点/步长、预览和排除原因。
  - 推荐历史区：生成 run、查看、人工确认应用、删除未应用 run。
- 这次需求不是从零设计策略系统，而是补齐现有工作台被刻意排除的自动生成与自动应用闭环。

## Assumptions (temporary)

- 第一版应复用现有推荐 run 和 apply 路径，不新增一套绕过审计的自动修改逻辑。
- 自动化应该有运行模式，避免上线后立即无条件改 priority：
  - `manual`：保留当前手动生成/应用。
  - `generate_only`：后台自动生成推荐 run，但不自动应用。
  - `auto_apply`：后台生成并自动应用通过安全门的 run。
- 自动应用应默认关闭，由管理员显式开启。
- 自动应用的 operator 可以使用系统操作者标识，或在审计字段中保留触发来源；具体落地需再看项目是否已有系统用户约定。
- 自动应用的第一阶段只调整现有推荐 suggestion 覆盖的 `accounts.priority`，不自动创建候选、不自动绑定 API Key、不自动启停候选。

## Open Questions

- 自动化第一版是否允许直接默认进入 `auto_apply`，还是默认只开启 `generate_only`，由管理员再手动打开自动应用？

## Requirements (evolving)

- 后台 runner 在现有 sync/probe 之外新增 recommendation job。
- recommendation job 应复用现有 `GenerateRecommendations()` 生成正式推荐 run，而不是只做 preview。
- 自动应用应复用现有 `ApplyRecommendationRun()` 事务路径，确保 stale check、applied 标记和 scheduler outbox 一致。
- 新增或扩展策略配置，表达：
  - 是否启用自动生成推荐。
  - 自动生成间隔和失败重试间隔。
  - 是否启用自动应用。
  - 自动应用安全门，例如最大 suggestion 数、最大 priority 变更幅度、最低置信度、是否允许 low confidence、是否允许失败健康状态、是否要求最近 sync/probe 都新鲜。
- 自动生成 run 后，如果 suggestion 数为 0，只记录 run，不触发应用。
- 自动应用失败时不能吞掉错误，需要让历史 run 或日志可追踪失败原因。
- UI 应在现有监控/推荐策略/推荐历史基础上补充自动策略状态，而不是新建一套割裂页面。
- 文案必须明确区分：
  - 自动同步/探测负责采集数据。
  - 推荐策略负责判断和排序。
  - 自动生成负责创建正式建议记录。
  - 自动应用负责修改账号 priority。

## Acceptance Criteria (evolving)

- [ ] 管理员可以在工作台开启/关闭自动生成推荐。
- [ ] 管理员可以在工作台开启/关闭自动应用，并看到安全门说明。
- [ ] runner 能按策略周期生成推荐 run。
- [ ] 自动应用只会应用 successful、未应用、存在 suggestion 且通过安全门的 run。
- [ ] 自动应用复用现有 `ApplyRecommendationRun` 的事务和 stale check。
- [ ] 自动应用后的 run 在推荐历史中可见，能区分自动触发和人工触发。
- [ ] 自动应用失败时有可诊断的错误记录或状态反馈。
- [ ] 后端测试覆盖生成调度、自动应用开关、安全门拒绝、stale 冲突、leader lock 防重入。
- [ ] 前端测试覆盖自动策略配置展示、保存、状态文案和历史状态。

## Definition of Done

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope

- 不自动创建或删除候选 API Key。
- 不自动绑定新的上游账号与目标组关系。
- 不自动修改倍率采集或探测算法本身。
- 不接入生产部署或真实线上开关。
- 不重做现有上游中继监控工作台信息架构。

## Technical Notes

- 任务目录：`.trellis/tasks/06-29-upstream-relay-auto-policy-apply`
- 当前分支：`feature/upstream-relay-policy-preview`
- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
- 相关既有任务：
  - `.trellis/tasks/06-28-upstream-relay-monitoring-global-policy-preview`
  - `.trellis/tasks/06-29-upstream-relay-monitoring-runner`
- 相关代码入口：
  - `backend/internal/service/upstream_relay_monitoring_runner.go`
  - `backend/internal/service/upstream_relay_group_monitoring.go`
  - `backend/internal/repository/upstream_relay_group_monitoring_repo.go`
  - `backend/internal/handler/admin/upstream_relay_group_monitoring_handler.go`
  - `backend/internal/server/routes/admin.go`
  - `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
  - `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`
