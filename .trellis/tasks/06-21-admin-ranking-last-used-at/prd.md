# 管理员排行榜最近使用时间

## Goal

管理员排行榜不再展示标准计费和账号成本，改为展示用户在当前筛选范围内的最近使用时间，让运营查看榜单时能直接判断用户活跃度。

## What I Already Know

- 用户希望管理员排行榜中的“标准计费”和“账号成本”替换为“最近使用时间”。
- Memory 中已有项目约束：管理员 Dashboard 用户消费榜应展示邮箱、请求数、非图片 Token、实际扣除和最近使用时间，不展示 `cost` 或 `account_cost`。
- 下游 fork 约束已读取：二开改动以 `custom/main` 为基线，开发分支使用 `feature/*`，不得误推 upstream。
- 当前实际分支是 `feature/channel-monitor-request-color-override`；本任务记录 base branch 为 `custom/main`。

## Requirements

- 管理员排行榜表格不展示标准计费字段。
- 管理员排行榜表格不展示账号成本字段。
- 管理员排行榜表格展示最近使用时间。
- 最近使用时间应来自当前榜单筛选范围内的用户最近一条用量记录。
- 邮箱缺失时不得回退展示用户 ID。

## Acceptance Criteria

- [x] 管理员排行榜页面中不再出现标准计费列。
- [x] 管理员排行榜页面中不再出现账号成本列。
- [x] 管理员排行榜页面中出现最近使用时间列，并能处理空值。
- [x] 后端响应如缺少最近使用时间，应补齐 `last_used_at` 口径。
- [x] 相关类型、测试或检查与改动保持一致。

## Definition of Done

- 代码改动保持在管理员排行榜相关前后端范围内。
- 运行与改动范围匹配的定向检查。
- 不提交、不推送，除非用户明确确认。

## Out of Scope

- 不调整普通用户排行榜隐私口径。
- 不改变排行榜排序、筛选周期或统计金额口径。
- 不改动生产数据库或部署配置。

## Technical Notes

- 必须遵循 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已用 Semble 定位管理员 Dashboard 消费榜和独立 Token 排行榜；本次实际命中 `frontend/src/views/admin/TokenLeaderboardView.vue` 与 `/api/v1/admin/dashboard/token-leaderboard`。
- 后端 `GetAdminTokenLeaderboard` 通过 `MAX(ul.created_at)` 返回当前筛选范围内的 `last_used_at`。
- 前端概览卡和表格都不再展示标准计费/账号成本，改为展示最近使用时间。
- 已验证：Go 定向测试、前端 TokenLeaderboardView Vitest、前端 typecheck、`git diff --check`。
