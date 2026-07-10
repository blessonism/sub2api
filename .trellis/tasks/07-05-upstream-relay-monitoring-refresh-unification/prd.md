# Upstream Relay Monitoring Refresh Unification

## Goal

让上游倍率监控页的“看数据”体验收敛成一个可信入口：用户进入页面、开启自动刷新或手动刷新时，都能同时更新倍率快照、余额、今日用量和页面展示数据，不再需要连续点击“刷新”和“刷新余额”；同时避免把有成本或有副作用的探测、生成建议、应用建议混入默认刷新。

## Requirements

- 顶部主按钮改为“刷新监控数据”，执行轻量刷新：同步倍率快照、刷新余额、刷新今日用量，然后重新加载候选、概览、快照变更/用量历史等可见状态。
- 自动刷新与顶部主按钮复用同一套轻量刷新语义，而不是只刷新 metrics。
- 轻量刷新不执行候选探测、不生成建议、不应用建议。
- 后端提供聚合刷新接口，返回每个 connector 的快照刷新和 metrics 刷新结果，支持成功、部分失败、失败状态。
- 前端用聚合结果展示统一反馈，包含成功/失败 connector 明细和最后更新时间。
- 保留“探测全部候选”和“生成/应用建议”为独立动作。
- 生成建议前如果关键数据不新鲜，应提供明确提示或“刷新后生成”的后续入口；本任务先保证主刷新入口能产生生成建议所依赖的新鲜数据，不自动触发探测。
- 候选表单必须在前端就地校验连接器、账号、上游分组、上游 API Key 和探测模型，后端错误只作为技术详情兜底。
- 历史候选缺少上游 API Key 时必须显示“配置不完整”，并支持一键筛选。
- 今日用量刷新遇到单个异常候选时必须跳过该候选并继续处理其他有效候选；结果需要区分成功分组、根因分组和被连带跳过的分组。
- 从刷新结果进入候选修复并保存后，必须自动刷新原连接器并更新当前结果卡。
- 监控刷新、批量同步、批量探测和调度优先级应用必须复用统一操作结果面板，固定说明动作、成功/失败对象、影响、下一步和折叠技术详情。
- 批量同步反馈必须出现在触发操作的当前区域；全局刷新异常必须提供进入连接器详情的入口。
- 调度优先级应用失败只在应用弹窗内展示，不得重复写入页面顶部错误。
- 候选健康详情必须把探测错误和不可靠原因翻译为结构化中文原因与处理建议，原始消息仅放在技术详情。
- Runner 定格失败必须包含日期、失败连接器和具体失败原因。
- 页面反馈必须带来源、可关闭，并在切换区域后清理；可见术语统一为“今日用量 / 上游分组 / 调度优先级 / Token”。
- `CandidateHealthDialog` 和 `RateSourceTag` 的全部可见文案必须接入 zh/en i18n。

## Acceptance Criteria

- [x] 顶部主刷新按钮一次点击后会调用新的聚合刷新 API，而不是只调用本地 `loadAll()` 或单独 metrics refresh。
- [x] 自动刷新开启后会调用同一聚合刷新 API，并避免并发重入。
- [x] 聚合刷新成功后，connector 余额、candidate 今日用量、倍率快照、概览和相关列表会被重新加载或局部更新。
- [x] 聚合刷新部分失败时，页面显示 partial 反馈，成功的 connector 数据仍更新。
- [x] 默认刷新不会调用 probe 或 recommendation apply/generate 接口。
- [x] API 类型、路由注册、前端 API 测试、视图测试覆盖新增聚合刷新路径。
- [x] 候选五个必填字段均有字段级提示，缺少任一字段时不发出保存请求。
- [x] 缺少上游 API Key 的历史候选有“配置不完整”徽标和筛选入口。
- [x] 同一连接器内异常候选不阻断其他有效候选刷新，结果明确显示成功 N 个、跳过 N 个及根因/连带分组。
- [x] 从刷新问题进入 API Key 绑定并保存后自动刷新对应连接器，结果卡同步更新。
- [x] 四类操作复用统一结果面板并回答动作、结果、影响、下一步和技术详情。
- [x] 批量同步就地反馈；全局刷新可进入连接器详情；调度优先级应用失败不污染顶部错误。
- [x] 健康详情、Runner 定格失败、顶部局部反馈和术语/i18n 满足“发生了什么、影响谁、怎么修复”的可读性要求。

## Definition of Done

- 后端服务、handler、route、测试完成。
- 前端 API 类型、页面交互、i18n、测试完成。
- 关键 Go/Vitest 测试通过，或记录无法运行的原因。
- 遵守下游 fork 分支边界，不修改无关脏工作区文件。

## Technical Approach

新增后端聚合刷新方法 `RefreshMonitoringData`，内部按 connector 并发执行：

1. `SyncConnector`：拉取 `/groups/available` 和 `/groups/rates`，更新倍率快照，同时尽力写入已知今日用量和余额。
2. `RefreshConnectorMetrics`：刷新余额和今日用量，并回读 connector/snapshots。
3. 汇总每个 connector 的 snapshot/metrics 子结果，生成整体 `success` / `partial` / `failed` 状态。

前端新增 `refreshMonitoringData()` API。页面顶部主按钮和自动刷新都调用该接口；刷新完成后复用现有 `refreshCandidatesSilent`、`refreshTodayUsageOverviewSilent`、`loadOverviewSnapshots`、`loadSnapshotChanges`、`loadUsageHistory` 等加载逻辑同步展示状态。旧的“仅抓取快照”“仅刷新余额与用量”保留在分区/连接器级操作中，降低主路径按钮噪音。

## Decision (ADR-lite)

**Context**: 当前页面存在本地 reload、snapshot sync、metrics refresh、probe、recommendation 多套刷新语义，用户需要理解内部链路并手动连点，自动刷新只刷 metrics，无法保证倍率监控数据新鲜。

**Decision**: 使用后端聚合接口作为“看数据”主入口；默认刷新只做低副作用读操作，不包含 probe、生成建议或应用建议。

**Consequences**: 主路径更可靠，前端状态更容易解释；后端接口会串联两类上游读取，耗时和失败面增加，需要 partial 结果和并发保护。探测新鲜度仍需单独策略控制，避免隐藏成本。

## Out of Scope

- 不自动执行候选 probe。
- 不自动生成或应用 recommendation。
- 不调整后台 runner 的调度策略。
- 不引入新的数据库表或迁移。

## Technical Notes

- 任务分支：`feature/upstream-relay-refresh-unification`，目标分支：`custom/main`。
- 必读约束：`.trellis/spec/guides/downstream-fork-workflow.md`、`.trellis/spec/frontend/type-safety.md`、`.trellis/spec/backend/quality-guidelines.md`。
- 主要影响文件：
  - `backend/internal/service/upstream_relay_group_monitoring.go`
  - `backend/internal/handler/admin/upstream_relay_group_monitoring_handler.go`
  - `backend/internal/server/routes/admin.go`
  - `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`
  - `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`
  - 对应 backend/frontend tests
