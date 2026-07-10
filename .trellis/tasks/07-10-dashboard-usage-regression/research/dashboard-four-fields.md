# Dashboard 四字段回归研究

## 结论

四个字段并未从公开契约或前端卡片中消失，而是在 2026-07-10 同步上游时丢失了仓储层的计算与赋值。当前接口仍会序列化这些字段，前端仍会渲染它们，但 Go 零值会一路传到 Dashboard，因此页面稳定显示 `0` / `$0.00`。

- 原始引入提交：`6a53f203a670df1dd6fdb95146dff93d4733f838`（`feat(admin):新增仪表盘运营指标`，2026-06-20）。
- 回归引入提交：`2c503a333df1f7903a8ecd3471787391c1601c4c`（`chore(sync):合入 upstream main 最新主线`，2026-07-10）。
- 直接根因：上游提交 `db3bd9971` 将 `usage_log_repo.go` 拆成多个文件；同步合并采用了上游的新 `usage_log_repo_dashboard.go`，但没有把下游原文件中的 `fillDashboardOperationalStats`、余额聚合、兼容赋值和 fallback 活跃口径迁入新文件。
- 影响集中在 `backend/internal/repository/usage_log_repo_dashboard.go`。现有 DTO、两个 handler 出口、前端类型、卡片、i18n 和聚合表的 `actual_cost > 0` 去重逻辑均仍存在，不需要重新设计公开契约。

## 四字段及原始口径

### `today_active_users`

- Go 字段：`usagestats.DashboardStats.TodayActiveUsers`。
- 前端字段：`DashboardStats.today_active_users`。
- 口径：与兼容字段 `active_users` 同值，表示当前本地 Dashboard 日内有成功扣费用量的去重用户数。
- 有效用量判定：`usage_logs.actual_cost > 0`。零扣费失败占位日志仍可计入请求数，但不能计入 token 活跃用户。
- 原实现：聚合路径从 `usage_dashboard_daily.active_users` 读取到 `ActiveUsers` 后执行 `stats.TodayActiveUsers = stats.ActiveUsers`；未启用预聚合的 range fallback 路径也在实时去重后执行相同赋值。
- 当前回归：两个路径均只写 `ActiveUsers`，从未写 `TodayActiveUsers`，所以新字段固定为零。

### `yesterday_active_users`

- Go 字段：`usagestats.DashboardStats.YesterdayActiveUsers`。
- 前端字段：`DashboardStats.yesterday_active_users`。
- 口径：读取当前本地 Dashboard 日的前一天 `usage_dashboard_daily.active_users`。
- 缺失昨日聚合行时返回零，而不是报错。
- 当前回归：原 `fillDashboardOperationalStats` 整体丢失，当前仓储没有任何代码给该字段赋值。

### `total_user_balance`

- Go 字段：`usagestats.DashboardStats.TotalUserBalance`。
- 前端字段：`DashboardStats.total_user_balance`。
- 口径：所有未软删除用户 `users.balance` 的总和，不额外按用户状态过滤。
- 原 SQL 位于 `fillDashboardEntityStats` 的用户统计查询，使用 `COALESCE(SUM(balance), 0)`，并扫描到 `stats.TotalUserBalance`。
- 当前回归：用户统计查询仅保留 `COUNT(*)` 和今日新用户数，余额表达式及对应 scan 参数均丢失。

### `subscription_remaining_value`

- Go 字段：`usagestats.DashboardStats.SubscriptionRemainingValue`。
- 前端字段：`DashboardStats.subscription_remaining_value`。
- 范围：未删除用户的当前有效、未删除、尚未过期订阅。
- 计算：`购买金额 * 剩余有效秒数 / 有效期秒数`，汇总后以 `COALESCE(..., 0)` 返回。
- 定价来源优先级：
  1. 当前用户和分组最近的已完成/已支付/充值中订阅订单；金额优先 `amount`，否则 `pay_amount`，有效期使用 `subscription_days`。
  2. 找不到订单金额时，回退到同分组在售套餐；优先选择有效期最接近当前订阅周期的套餐，再按 `sort_order`、价格和 ID 稳定选择。
- 当前回归：包含两个 lateral lookup 的整段原查询随 `fillDashboardOperationalStats` 丢失，字段固定为零。

## 当前跨层数据流

管理员 Dashboard 实际读取路径已经从原来的单独 stats 请求演进为 snapshot-v2：

```text
DashboardView.vue
  -> frontend adminAPI.dashboard.getSnapshotV2()
  -> GET /api/v1/admin/dashboard/snapshot-v2?include_stats=true
  -> DashboardHandler.buildSnapshotV2Response()
  -> DashboardService.GetDashboardStats()
  -> usageLogRepository.GetDashboardStats() / GetDashboardStatsWithRange()
  -> usagestats.DashboardStats
  -> DashboardSnapshotV2Stats
  -> 四张运营指标卡片
```

各触点现状如下：

- **来源/聚合保留**：`backend/internal/repository/dashboard_aggregation_repo.go` 仍在写入 hourly/daily active-user 去重表，并保留 `actual_cost > 0`。这部分不是本次恢复对象。
- **仓储计算丢失**：`backend/internal/repository/usage_log_repo_dashboard.go` 未迁入下游运营指标逻辑，是四字段失真的唯一直接断点。
- **服务层保留**：`backend/internal/service/dashboard_service.go` 仍根据是否启用预聚合选择普通或 range fallback 路径，并对结果做 15 秒 fresh / 30 秒默认 TTL 的 stats 缓存。
- **DTO 保留**：`backend/internal/pkg/usagestats/usage_log_types.go` 仍声明四个 JSON 字段。
- **legacy handler 保留**：`backend/internal/handler/admin/dashboard_handler.go` 的 `/stats` 仍显式输出四个字段。
- **实际 handler 保留**：`backend/internal/handler/admin/dashboard_snapshot_v2_handler.go` 将整个 `DashboardStats` 嵌入 snapshot-v2 响应；无需增加字段映射。
- **路由/API 类型保留**：`backend/internal/server/routes/admin.go`、`frontend/src/api/admin/dashboard.ts` 均已有 snapshot-v2 契约，`DashboardSnapshotV2Stats` 继承 `DashboardStats`。
- **前端类型和卡片保留**：`frontend/src/types/index.ts` 与 `frontend/src/views/admin/DashboardView.vue` 已包含四字段并正确格式化。
- **i18n 保留**：模块化迁移后，中英文文案位于 `frontend/src/i18n/locales/{zh,en}/custom.ts`，不应重新写回已删除的单文件 `zh.ts` / `en.ts`。

## 回归的精确差异

对比 `2c503a333^1` 与 `2c503a333`，原下游实现到新拆分文件的遗漏包括：

1. `GetDashboardStats` 与 `GetDashboardStatsWithRange` 不再调用 `fillDashboardOperationalStats`。
2. `fillDashboardEntityStats` 的用户 SQL 不再选择 `COALESCE(SUM(balance), 0)`，scan 列表也不再接收 `TotalUserBalance`。
3. `fillDashboardOperationalStats` 整个函数丢失，因此昨日活跃与订阅剩余价值没有任何生产者。
4. `fillDashboardUsageStatsAggregated` 不再执行 `TodayActiveUsers = ActiveUsers`。
5. `fillDashboardUsageStatsFromUsageLogs` 的活跃用户 scoped CTE 丢失 `AND actual_cost > 0`，且不再执行 `TodayActiveUsers = ActiveUsers`。

第 5 点不仅使新字段为零，还让兼容字段 `active_users` 在关闭预聚合的 fallback 模式下重新把零扣费失败日志算作活跃用户，属于同一处必须恢复的口径回归。

## 现有测试

### 已保留但会暴露仓储回归

- `backend/internal/repository/usage_log_repo_integration_test.go`
  - `TestDashboardStats_OperationalMetrics` 直接验证昨日活跃、余额池、订单优先与套餐 fallback 的订阅剩余价值。
  - Dashboard 基础统计测试验证 `TodayActiveUsers == ActiveUsers`。
  - `TestDashboardAggregationConsistency` 验证零扣费日志计入请求聚合但不计入 hourly/daily active users。
- 这些集成测试当前仍在工作树中，但需要 PostgreSQL 集成测试环境；本次只读研究未执行它们。仅跑普通 unit/frontend 检查可能无法发现 repository 实现与保留测试之间的断裂。

### 已保留但只验证契约/展示

- `backend/internal/handler/admin/dashboard_handler_cache_test.go`
  - `TestDashboardHandler_GetStats_IncludesOperationalMetrics` 使用 stub stats，验证 legacy `/stats` 输出字段。
  - 它不会验证 repository 是否真实计算字段，也不覆盖 Dashboard 当前使用的 `/snapshot-v2` 出口。
- `frontend/src/views/admin/__tests__/DashboardView.spec.ts`
  - `renders operational metric cards` 使用 mock snapshot，验证四个标签和值能渲染。
  - 它证明前端没有丢卡片，但无法发现后端返回零值。

### 建议补强

- 扩展 `TestDashboardStatsWithRange_Fallback`：加入当日成功扣费用户和零扣费用户，断言 `ActiveUsers == TodayActiveUsers` 且零扣费用户不计入。
- 增加 snapshot-v2 handler 测试：`include_stats=true` 时断言四字段位于 `data.stats`，避免以后实际页面出口与 legacy `/stats` 测试脱节。
- 保留现有 `TestDashboardStats_OperationalMetrics` 作为真实 SQL 回归测试；不要用只检查字段存在的 mock 测试替代。

## 建议恢复范围

最小且完整的恢复范围是：

1. 只在 `backend/internal/repository/usage_log_repo_dashboard.go` 恢复原有四字段生产逻辑，并适配当前拆分后的文件结构。
2. 两个入口都调用同一个 `fillDashboardOperationalStats`，避免聚合开启/关闭时口径漂移。
3. 在 aggregated 与 range fallback 两条活跃路径都显式同步 `TodayActiveUsers = ActiveUsers`。
4. range fallback 的活跃去重恢复 `actual_cost > 0`；请求数和费用聚合保持现有上游行为，不把过滤错误扩散到整个 scoped 查询。
5. 不修改 `DashboardStats` JSON 名、legacy handler、snapshot-v2 DTO、前端 API 类型、Dashboard 卡片或 i18n；这些触点均已正确保留。
6. 以现有 repository 集成测试为主，并补上 fallback 与 snapshot-v2 的缺口。

## 恢复时的边界与 caveat

- 不能直接把旧 `usage_log_repo.go` 整段覆盖回来；上游已完成仓储拆分且包含大量新功能，恢复应是对新 `usage_log_repo_dashboard.go` 的小范围移植。
- `todayUTC` 在相关函数名中是历史命名，实际入口使用项目 `timezone.Today()` 的本地 Dashboard 日界；恢复昨日查询时应沿用它，不要另造 UTC 日界。
- 订阅金额查询依赖当前 `payment_orders`、`subscription_plans`、`user_subscriptions` 字段和状态常量。原 SQL 在同步前已通过集成测试，优先原样迁移语义，再按新文件 import/常量位置调整。
- 修复部署后，旧零值可能在 DashboardService stats 缓存和 snapshot-v2 30 秒缓存中短暂可见；这是缓存生存期，不代表计算仍错误。
- 本文只研究 Dashboard 四字段。用户使用记录的延迟列回归由同任务的独立研究/实现边界处理。
