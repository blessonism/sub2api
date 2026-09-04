# 实现记录：分组级渠道监控

分支：`feature/group-gateway-probe`（base: `custom/main`）。按 PRD/design 完成，未提交（高风险操作待确认）。

## 后端

1. **迁移** `234_channel_monitor_gateway_group.sql`：
   - `channel_monitor_histories` 加 `error_category VARCHAR(40) NOT NULL DEFAULT ''`
   - `channel_monitors` 加 `target_kind VARCHAR(20) NOT NULL DEFAULT 'endpoint'` + CHECK 约束
2. **ent schema**（`channel_monitor.go` / `channel_monitor_history.go`）加字段并 `go generate ./ent`。
3. **checker**（`channel_monitor_checker.go`）：新增 `classifyMonitorErrorCategory`——复用 v2 分类器
   `ClassifyChannelMonitorV2Error`（statusCode 同时传本侧/上游侧，让 5xx 归 `upstream_5xx`），
   并补充网关流内限流形态（HTTP 200 + SSE `response.failed`，无顶层 429，v2 关键词表未覆盖
   "pending requests" 文案）。`runCheckForModel` 的两个 error 分支注入归类。
4. **service**：`CheckResult`/`ChannelMonitor`/`ChannelMonitorHistoryRow`/`ChannelMonitorHistoryEntry`/
   `ChannelMonitorLatest`/`MonitorStatusSummary`/`UserMonitorView`/`UserMonitorTimelinePoint`/
   `ModelDetail` 加字段；Create/Duplicate/Update 传 `TargetKind`（`validateTargetKind` +
   `defaultTargetKind`）；`persistCheckResults` 写 category；aggregator 填
   `PrimaryErrorCategory`/timeline category/`ModelDetail.LatestErrorCategory`。
5. **repo**：ent CRUD 带两列；三个原生 SQL（latest per model / latest batch / recent history）
   select+scan `error_category`。
6. **handler**：admin（create/update 请求绑定 `target_kind`、响应带 `target_kind` +
   `primary_error_category`、check result/history 带 `error_category`）；user（list item 带
   `target_kind`/`primary_error_category`/timeline `error_category`，detail 带模型级 category）。

## 前端

- 类型（`api/admin/channelMonitor.ts`、`api/channelMonitor.ts`）：`TargetKind`、
  `MonitorErrorCategory`、各接口字段。
- 常量（`constants/channelMonitor.ts`）：`TARGET_KIND_*`。
- `useChannelMonitorFormat`：`statusLabel`/`statusBadgeClass` 加可选 `errorCategory` 参数，
  error + rate_or_capacity 显示黄色"限流"（`monitorCommon.status.rate_limited`）；导出 `isRateLimited`。
- `MonitorCard.vue`：状态徽标限流感知；`target_kind=gateway_group` 显示"分组"徽标。
- `MonitorTimeline.vue`：限流点用 `bg-amber-400`（区别于故障红）。
- `MonitorDetailDialog.vue`：模型表 latest 状态限流感知。
- 管理端 `MonitorFormDialog.vue`：探测目标选择（外部上游/本站分组）+ 分组探测引导文案；
  `buildPayload`/`resetForm`/`loadFromMonitor` 携带 `target_kind`。
- `ChannelMonitorView.vue`：列表平台列加"分组"徽标。
- i18n：`locales/{zh,en}/dashboard.ts`（`monitorCommon.status.rate_limited`、
  `monitorCommon.targetKind.gateway_group`）、`locales/{zh,en}/admin/channels.ts`
  （`admin.channelMonitor.form.targetKind*`）。

## 验证

- `go build ./...`、`go vet`（service/repository/handler/ent）通过。
- 新增 `channel_monitor_error_category_test.go`（//go:build unit，与既有 checker 测试同惯例）：
  分类器表驱动 + 429/流内限流端到端 + 成功/challenge mismatch 无归类 + target_kind 校验，全过。
- 监控全链路回归（service `-run Monitor`、repository、handler/admin、dto）通过。
- 前端 `vue-tsc --noEmit` 与 `npm run build` 通过。

## 预存在问题（与本次无关）

- `internal/server/api_contract_test.go`、`internal/handler/gateway_handler_cancellation_test.go`
  等因 `NewAdminService`/`NewGatewayService` 签名不匹配编译失败；repository 的
  `TestPrepareUsageLogInsert_RequestedReasoningEffortArgWiring` 失败——已用干净 HEAD worktree
  验证均为预存在失败（源于 usage/reasoning-effort 与上游同步提交），本次未触碰。
- `go.sum` 新增 ent CLI 传递依赖（go-runewidth/tablewriter），为 `go generate ./ent` 的正常副作用。
