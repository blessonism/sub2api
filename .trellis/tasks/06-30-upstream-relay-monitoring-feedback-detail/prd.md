# brainstorm: 上游倍率监控反馈优化

## Goal

优化上游倍率监控页面中手动探测和批量探测后的反馈信息，让管理员能清楚知道本次操作探测了哪些候选映射、哪些成功、哪些失败、失败原因是什么，以及是否还有被隐藏的明细。目标是把一次性操作结果从“数量摘要”升级为“可审计的操作反馈”。

## What I already know

- 用户反馈：当前上游倍率监控页面的反馈信息过于简单，典型问题是“一键探查一个成功一个错误时只显示错误，成功的是哪个不知道，也没有具体原因”。
- 候选映射页顶部已有 `candidateProbeFeedback` 和 `candidateBulkProbeFeedback` 两个反馈面板。
- 批量探测接口返回 `items`，每个 item 已有 `success`、`candidate_id`、`account_id`、`account_name`、`connector_name`、`error_reason` 等基础字段。
- 前端当前只展示失败项：`candidateBulkProbeFailedItems` 会过滤 `!item.success` 并截断前 5 条。
- 监控配置页的手动动作结果也只列出失败项，成功项只剩数量。
- 后端成功 item 目前只设置 `success=true` 和 `count=1`，没有透出 `probe_result_id`、`latency_ms`、`http_status`、`error_class`、`probed_at` 等探测上下文。
- 当前已有正式设计文档 `docs/UPSTREAM_RELAY_MONITORING_UI_REDESIGN_CN.md`，适合追加本轮反馈可审计性设计。
- 本仓库是 `Wei-Shaw/sub2api` 的下游二开仓库；本任务 base branch 为 `custom/main`，工作分支记录为 `fix/upstream-relay-monitoring-feedback-detail`。

## Problem List

1. 批量探测混合结果不可审计：只展示失败项，不展示成功项身份。
2. 批量失败列表最多显示 5 条，但没有“另有 N 条未显示”的提示。
3. 部分成功时图标和视觉语义偏失败，容易让操作者忽略成功结果。
4. 成功项缺少耗时、HTTP 状态、探测时间、probe result id 等上下文。
5. 失败原因原样展示，缺少错误分类、本地化标签和下一步建议。
6. 单个探测失败时详情存在，但错误分类不够友好，失败上下文层级可以更清楚。
7. 行内健康列只给“失败/认证问题/未知”等摘要，批量探测后需要更快定位最近错误。
8. 候选页顶部反馈和监控页手动动作反馈展示深度不一致。
9. 测试只覆盖批量失败摘要，没有覆盖混合成功/失败、隐藏数量、成功明细和错误分类。

## Assumptions

- 本轮不改变探测执行逻辑、并发策略或调度器，只增强手动/批量操作结果的返回和展示。
- 本轮优先服务管理员的即时判断，不引入新的历史审计表；持久化历史仍依赖现有 probe result / health snapshot。
- 成功项明细可通过扩展 `UpstreamRelayBulkOperationItem` 返回，不需要前端再从刷新后的 candidates 中反推。
- 前端可在同一页面内复用批量操作明细渲染逻辑，避免候选页和监控页继续分叉。

## Requirements

- 批量探测完成后必须展示总数、成功数、失败数和完成时间。
- 批量探测有成功项时，反馈面板必须能展示成功候选映射身份，至少包括账号、候选 id 或映射标签。
- 批量探测有失败项时，反馈面板必须展示失败候选映射身份、错误原因，并尽量展示错误分类。
- 当成功或失败明细被截断时，必须显示剩余数量提示。
- 部分成功状态必须使用“部分成功/部分失败”的中性视觉语义，而不是纯失败语义。
- 后端批量 item 应补充本次探测的可展示上下文：`probe_result_id`、`latency_ms`、`http_status`、`error_class`、`probed_at`。
- 前端 API 类型必须与后端 JSON 字段保持 snake_case 对齐。
- 候选页和监控页都应使用同一套明细标签、错误标签和隐藏数量逻辑。
- zh/en 文案必须补齐新增展示文案。

## Acceptance Criteria

- [x] 一键探测 1 成功 1 失败时，候选页反馈面板同时展示成功项和失败项。
- [x] 一键探测 1 成功 1 失败时，监控页手动动作结果也同时展示成功项和失败项。
- [x] 成功项至少展示账号/候选标识、耗时、HTTP 状态或完成时间中的可用信息。
- [x] 失败项展示账号/候选标识、错误分类标签和错误原因；缺失原因时显示明确 fallback。
- [x] 超过展示上限时显示“另有 N 条成功/失败未显示”。
- [x] 部分成功状态使用 warning/partial 文案与视觉，不使用纯失败标题。
- [x] `UpstreamRelayBulkOperationItem` 后端、前端类型和测试数据字段一致。
- [x] 相关视图测试覆盖混合成功/失败、隐藏数量、成功明细和失败原因。

## Definition of Done

- 任务 PRD 与正式 UI 设计文档已记录问题和设计。
- 后端批量探测结果字段已增强，并保持原有调用兼容。
- 前端候选页和监控页批量反馈展示已增强。
- zh/en i18n 文案已补齐。
- 相关单元测试或视图测试已更新。
- 运行聚焦测试，若无法运行需记录原因。

## Out of Scope

- 不新增批量探测历史表或新的后端分页查询接口。
- 不改自动监控 runner 的策略和调度。
- 不重做整个上游倍率监控页面布局。
- 不处理生产部署、数据库迁移或远端同步。

## Technical Notes

- 相关文件：
  - `backend/internal/service/upstream_relay_group_monitoring.go`
  - `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`
  - `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
  - `frontend/src/views/admin/__tests__/UpstreamRelayGroupMonitoringView.spec.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`
  - `docs/UPSTREAM_RELAY_MONITORING_UI_REDESIGN_CN.md`
- 已读约束：
  - `.trellis/spec/guides/downstream-fork-workflow.md`
  - `.trellis/spec/frontend/type-safety.md`
  - `.trellis/spec/backend/quality-guidelines.md`
  - `.trellis/spec/guides/cross-layer-thinking-guide.md`
  - `.trellis/spec/guides/code-reuse-thinking-guide.md`
- 当前分支检查：
  - 当前分支：`fix/upstream-relay-candidate-group-select`
  - `upstream` push：`DISABLED`
  - `origin/main...upstream/main`：`0 167`
