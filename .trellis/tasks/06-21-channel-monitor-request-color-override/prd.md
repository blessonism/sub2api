# brainstorm: 管理员修改渠道监控请求颜色

## Goal

让管理员能够针对渠道监控里的某一次请求结果，人工调整其绿 / 黄 / 红展示状态，用于修正自动检测结果与实际运营判断不一致的场景，同时保留系统自动检测与人工干预之间的边界。

## What I already know

- 用户希望讨论的是“渠道监控”功能，而不是普通渠道配置或用量榜单。
- 目标对象是“某次请求”的绿 / 黄 / 红颜色，颗粒度看起来更接近监控历史记录，而不是整个监控配置。
- 当前前端将监控状态抽象为 `operational`、`degraded`、`failed`、`error`，并映射为绿色、黄色、红色、灰色。
- 管理员侧和用户侧共享渠道监控状态格式化逻辑，因此人工调整是否对用户可见需要明确。
- 后端 `channel_monitor_histories` 当前是一行代表“一次检测中的一个模型结果”，包含 `status`、`latency_ms`、`ping_latency_ms`、`message`、`checked_at`。
- 用户侧渠道状态页会读取最近历史点作为 timeline，并读取聚合状态 / 可用率；人工覆盖是否进入这些用户侧视图与统计需要明确。
- 用户已确认：人工修改后的颜色需要影响用户侧状态页、最近状态、可用率统计。

## Assumptions (temporary)

- “绿 / 黄 / 红”不是任意颜色选择器，而是三档健康状态：正常、降级、失败。
- 管理员修改的应是一次历史检测结果的展示状态，默认不直接修改请求原始延迟、错误信息或检测日志。
- 后续新检测结果仍按系统规则自动生成，不会被上一次人工修改继承。

## Open Questions

- 暂无。

## Requirements (evolving)

- 管理员可以在某个监控的历史请求记录中选择一条记录，并将其展示状态调整为绿、黄或红。
- 系统应能区分自动检测状态与人工覆盖状态，避免以后排查时看不出该状态是系统判定还是人工修正。
- 被修改的记录应保留原始检测信息，如模型、延迟、ping 延迟、消息、检测时间。
- 权限边界应限制为管理员接口，普通用户不能修改监控历史状态。
- 管理员修改对象应按历史记录 ID 定位，避免同一监控、同一模型、相近时间的记录被误改。
- 用户侧状态页、最近状态和可用率统计应使用人工覆盖后的有效状态；没有人工覆盖时使用系统自动检测状态。
- 管理员可以清除某条记录的人工覆盖，使其恢复为系统自动检测状态。

## Acceptance Criteria (evolving)

- [x] 管理员可以对单条渠道监控历史记录设置人工颜色状态。
- [x] 管理员可以看到某条记录是否存在人工覆盖。
- [x] 未设置人工覆盖的记录继续使用自动检测状态展示。
- [x] 后续自动检测不会覆盖已有历史记录的人工标记，除非管理员主动清除或再次修改。
- [x] 用户侧状态页 timeline 展示人工覆盖后的有效颜色。
- [x] 最近状态聚合使用人工覆盖后的有效状态。
- [x] 可用率统计使用人工覆盖后的有效状态。
- [x] 管理员可以清除人工覆盖，并恢复该条记录的系统自动检测状态。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 不做任意颜色选择器。
- 不改变监控调度和真实请求执行逻辑。
- 不改变延迟、ping 延迟、错误消息等原始检测结果。

## Technical Notes

- 已按下游 fork 工作流设置任务 base branch 为 `custom/main`，工作分支为 `feature/channel-monitor-request-color-override`。
- 后续实现和检查上下文均已加入 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 初步相关文件：
  - `frontend/src/composables/useChannelMonitorFormat.ts`
  - `frontend/src/api/admin/channelMonitor.ts`
  - `frontend/src/api/channelMonitor.ts`
  - `backend/ent/schema/channel_monitor_history.go`
  - `backend/internal/service/channel_monitor_types.go`
  - `backend/internal/service/channel_monitor_service.go`
  - `backend/internal/repository/channel_monitor_repo.go`
  - `backend/internal/handler/admin/channel_monitor_handler.go`
  - `frontend/src/views/admin/ChannelMonitorView.vue`
- 当前管理员页面只展示监控配置列表与手动执行结果弹窗，没有直接暴露历史明细列表；若要“修改某次请求”，可能需要先提供历史记录入口或在运行结果/历史弹窗中增加操作。
- 实现采用 `status` 保留系统自动检测状态，新增 `override_status` 记录人工覆盖；读取侧统一用 `COALESCE(override_status, status)` 作为 `effective_status`。
- 人工覆盖仅允许绿 / 黄 / 红三档：`operational`、`degraded`、`failed`；系统原始状态仍允许保留 `error`。
- 新增管理员历史弹窗入口，可按模型查看历史记录、设置三档人工状态、清除覆盖恢复自动状态。
- 验证记录：
  - `pnpm typecheck`
  - `go test -tags=unit ./internal/service -run 'TestBuild(StatusSummary|TimelinePoints)|TestSchedule' -count=1`
  - `go test ./internal/repository ./internal/handler/admin ./internal/server/routes -run '^$' -count=1`
  - 曾尝试 `go test -tags=unit ./internal/service ./internal/repository ./internal/handler/admin ./internal/server/routes`，超过 60 秒后按项目测试要求中断，改跑更窄范围验证。
