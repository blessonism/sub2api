# 技术设计：分组级渠道监控

## 边界

复用 v1 probe 引擎（checker/runner/调度/挑战校验零改动）。改动集中在：错误分类注入、两个新字段的持久化与透传、前端展示与表单。

## 数据模型

迁移 `2XX_channel_monitor_gateway_group.sql`：

1. `channel_monitor_histories` 加 `error_category VARCHAR(40) NOT NULL DEFAULT ''`。
   取值复用 v2 taxonomy 的类别名（本期只消费 `rate_or_capacity`，其余类别照存不消费）。
2. `channel_monitors` 加 `target_kind VARCHAR(20) NOT NULL DEFAULT 'endpoint'`
   加 CHECK 约束 `IN ('endpoint','gateway_group')`。
   `gateway_group` = 纯展示标注（探测仍走 probe 引擎，endpoint 指向本站网关入口）。

## 后端改动

- `service/channel_monitor_types.go`：
  - `CheckResult.ErrorCategory string`
  - `ChannelMonitor.TargetKind string`（空串按 endpoint 处理）
  - `ChannelMonitorCreateParams/UpdateParams` 加 `TargetKind`
  - `UserMonitorView.TargetKind`、`MonitorStatusSummary` 相关透传
- `service/channel_monitor_checker.go`：
  - error 路径（含 HTTP 非 2xx 与流内终态失败）构造结果时调用
    `ClassifyChannelMonitorV2Error`（只传 message 文本与已知 status code，
    复用 v2 分类器，不复制关键词）。
- `service/channel_monitor_service.go`：`persistCheckResults` 写入 error_category；
  summary/latest 查询带出新列。
- `repository/channel_monitor_repo.go`：两列读写；Create/Update 参数传递。
- handler：`admin/channel_monitor_handler.go`（Create/Update/List/Get DTO）与
  `channel_monitor_user_handler.go`（UserMonitorView）透传。

## 前端改动

- `api/channelMonitor.ts`：`error_category`、`target_kind` 类型。
- `constants/channelMonitor.ts`：`ERROR_CATEGORY_RATE_OR_CAPACITY`、`TARGET_KIND_*`。
- `MonitorCardGrid` / `MonitorDetailDialog`：error 主状态且 category=rate_or_capacity
  时黄色"限流/拥挤"样式；target_kind=gateway_group 显示"分组"徽标。
- 管理端表单：目标类型选择（外部上游/本站分组）；选本站分组时联动
  分组下拉 → 该分组 API key 下拉；endpoint 手填 + 提示文案。
- i18n：遵循 locales 目录结构，二开文案放 custom overlay（downstream 手册）。

## 测试

- checker 分类单测：限流文案 → rate_or_capacity；普通 5xx/网络错误 → 不标注或对应类别。
- 迁移测试：参照 `channel_monitor_quota_mode_migration_test.go` 模式验证新列与约束。
- repo/handler 透传断言。

## 回滚

恢复代码即可；两列带 DEFAULT，回滚不破坏旧数据。
