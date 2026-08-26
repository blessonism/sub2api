# 账号管理显示当天缓存命中情况

## Goal

在管理员账号管理列表中直接看到每个账号当天的缓存命中情况，减少逐个打开用量详情的操作。

## Confirmed Facts

- 账号管理页面为 `frontend/src/views/admin/AccountsView.vue`。
- 页面已有“今日统计”列和 `AccountTodayStatsCell` 组件，并通过
  `POST /api/v1/admin/accounts/today-stats/batch` 批量加载。
- 后端今日统计查询位于
  `backend/internal/repository/usage_log_repo_stats.go:GetAccountTodayStats`，目前只返回请求数、总 Token 和费用，未暴露输入 Token、缓存创建 Token、缓存读取 Token。
- 用量日志已有 `input_tokens`、`cache_creation_tokens`、`cache_read_tokens` 字段；趋势图已有缓存命中率口径：
  `cache_read / (input + cache_creation + cache_read)`。

## Requirements

- 在账号管理的今日统计中显示当天缓存读取（命中）Token。
- 显示缓存命中率，沿用现有趋势图口径；无输入 Token 时显示 `-`。
- 继续复用现有批量今日统计接口和账号管理权限，不新增独立请求或数据库表。
- 中英文界面均提供对应文案，保持现有账号表紧凑布局。
- 无缓存命中时显示稳定的零值，不因缺少统计记录导致列布局抖动。

## Acceptance Criteria

- [x] 账号管理列表中每个账号的今日统计可看到缓存命中 Token。
- [x] 命中率按 `cache_read / (input + cache_creation + cache_read)` 计算并保留 1 位小数；分母为 0 时显示 `-`。
- [x] 批量接口一次返回所有展示账号的缓存字段，未命中账号返回零值。
- [x] 后端和前端现有今日统计、账号列表测试不回归，并补充缓存字段/命中率测试。
- [x] 变更仅位于 `feature/account-cache-hit-today`，目标分支为 `custom/main`。

## Decision

- 采用“缓存命中 Token + 命中率”双字段展示。
- 命中率分母为普通输入 Token、缓存创建 Token、缓存读取 Token 三者之和；该口径与现有趋势图保持一致。
