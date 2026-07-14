# 修复 Dashboard 四字段与使用记录回归

## 背景

同步上游 `main` 后，仓库拆分了 Dashboard 用量仓储文件并改造了用户用量页。合并结果保留了部分前端类型和展示，却漏掉了后端计算以及用户用量延迟列的渲染实现，形成静默回归。

## 可观察问题

1. 管理员 Dashboard 仍展示以下四个字段，但接口数据不再反映真实数据：
   - `today_active_users`
   - `yesterday_active_users`
   - `total_user_balance`
   - `subscription_remaining_value`
2. 用户使用记录仍声明 `latency` 列，但对应单元格为空，合并首字耗时/总耗时并按健康度着色的优化失效。

## 根因边界

- `backend/internal/repository/usage_log_repo_dashboard.go` 在上游文件拆分时漏迁运营指标查询、兼容赋值及有效扣费活跃用户口径。
- `frontend/src/views/user/UsageView.vue` 在合并时只保留了 `latency` 列定义，未保留其模板、健康度工具导入和长耗时格式化。
- 现有 Dashboard DTO、handler、前端类型、卡片、i18n 及管理员用量表的延迟健康列均已存在，不应重复实现或改动公共契约。

## 实现要求

### Dashboard

- 恢复 `GetDashboardStats` 和 `GetDashboardStatsWithRange` 的运营指标填充。
- `total_user_balance` 统计所有未软删除用户余额。
- `yesterday_active_users` 读取本地 Dashboard 日的前一天预聚合值；缺失行返回零。
- `subscription_remaining_value` 延续下游既有订单优先、套餐回退的剩余有效期折算口径。
- `today_active_users` 与兼容字段 `active_users` 同值。
- 实时范围查询的 token 活跃用户只统计 `actual_cost > 0` 的记录。
- 不修改公开字段名，不引入兼容分支或重复 DTO。

### 使用记录

- 在用户使用记录恢复单个 `latency` 列，同时显示首字耗时与总耗时。
- 复用 `frontend/src/utils/latencyHealth.ts`，与管理员用量表保持相同阈值、颜色和无首字耗时处理。
- 超过一分钟的总耗时使用既有紧凑格式，避免长秒数难以阅读。
- 保留上游合并后新增的筛选、错误请求、管理员代看、视频/图片计费等能力。

## 验收标准

- 现有 Dashboard handler/view 测试继续通过，仓储测试能验证四字段真实计算。
- 新增用户用量页测试，证明 `latency` 列实际渲染首字耗时、总耗时和健康度样式。
- 前端类型检查与针对性 Vitest 通过；后端相关 Go 测试通过。
- 变更仅位于 `fix/dashboard-usage-regression`，目标分支为 `custom/main`。
