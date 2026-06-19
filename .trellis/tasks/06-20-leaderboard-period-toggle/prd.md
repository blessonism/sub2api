# 排行榜日榜 / 周榜切换

## Goal

将用户侧排行榜从固定今日榜升级为可切换日榜和周榜，同时在管理员 Dashboard 用户消费排行榜区域提供同样的日榜/周榜快捷切换。周榜按自然日近 7 天计算，即今天和往前 6 天，提升普通用户用量感知和管理员快速排查效率。

## Requirements

* 用户侧 `/leaderboard` 页面支持日榜/周榜切换，默认日榜。
* 用户侧接口 `GET /api/v1/usage/dashboard/leaderboard` 支持 `period=day|week`，默认 `day`。
* 日榜使用今天自然日；周榜使用今天及前 6 天自然日，继续使用请求时区计算边界。
* 用户侧响应继续返回 `ranking`、`my_rank`、`start_date`、`end_date`、`limit`，并新增 `period`。
* 非法 `period` 返回 400，不静默降级。
* 普通用户排行榜仍固定 Top10，仍只展示脱敏邮箱，不暴露完整邮箱或用户 ID。
* 管理员 Dashboard 用户消费排行榜区域增加日榜/周榜快捷切换，只更新日期范围并触发已有排行榜、图表与统计刷新。
* 管理员手动选择自定义日期后，不强制归入日榜/周榜；现有 DateRangePicker 能力保留。
* 管理员现有订阅套餐额度回补、最近使用时间、邮箱展示和不展示用户 ID 的约束保持不变。

## Acceptance Criteria

* [ ] 用户侧默认请求日榜，表现与旧版今日榜一致。
* [ ] 用户侧切换周榜后请求 `period=week`，展示自然日近 7 天范围、Top10 和当前账号排名。
* [ ] 当前用户不在周榜 Top10 时，仍展示自己的排名。
* [ ] 非法 `period` 返回 400。
* [ ] 普通用户响应不包含完整邮箱或用户 ID。
* [ ] 用户侧今日无数据与近 7 天无数据有对应空状态文案。
* [ ] 管理员 Dashboard 可快速切换日榜/周榜，并重新加载用户消费排行榜和相关统计。
* [ ] 管理员榜单仍不展示用户 ID，缺失邮箱时也不回退显示用户 ID。

## Definition of Done

* 后端定向测试覆盖 period 参数、周榜窗口、非法 period、隐私边界和当前用户不在 Top10。
* 前端定向测试覆盖用户榜单切换、API 参数、空状态文案和管理员快捷切换。
* 运行与改动范围匹配的 Go/Vitest/typecheck 或 lint 检查。
* 不提交、不推送、不修改生产配置或生产数据库。

## Technical Approach

* 后端在 handler 层解析并校验 `period`，将其转换为明确的开始/结束时间窗口，服务层继续只接收显式时间窗口和 period 字符串。
* 仓储层 `GetUserTokenLeaderboard` 保持按时间窗口聚合，不感知 day/week，避免重复查询逻辑。
* 服务层在普通用户 DTO 中增加 `period` 字段，并继续负责邮箱脱敏。
* 前端用户侧在 `usage.ts` 类型和请求函数中补充 `period`，`LeaderboardView.vue` 使用分段按钮控制请求参数和文案。
* 管理员侧复用 Dashboard 当前 `startDate/endDate/onDateRangeChange/load*` 流程，新增日榜/周榜快捷方法，不新增管理端接口。

## Decision (ADR-lite)

**Context**: 用户侧和管理员侧都需要快速查看日榜/周榜，但普通用户有严格隐私边界，管理员端已有日期范围统计能力。

**Decision**: 普通用户接口新增 `period=day|week`；管理员端只做快捷日期切换，复用现有 `/admin/dashboard/users-ranking` 与相关统计接口。

**Consequences**: 用户侧契约清晰且便于未来扩展；管理员端改动小，不破坏现有自定义日期和筛选能力。

## Out of Scope

* 月榜、活动奖励、徽章、导出、独立周榜页面。
* 改变用户侧 Top10 限制或管理员侧 TopN/limit 逻辑。
* 公开完整用户信息给普通用户。
* 生产数据库迁移、生产配置或部署操作。

## Technical Notes

* 二开约束：`.trellis/spec/guides/downstream-fork-workflow.md`。
* 后端规范：`.trellis/spec/backend/quality-guidelines.md`。
* 前端规范：`.trellis/spec/frontend/type-safety.md`。
* 跨层与复用规范：`.trellis/spec/guides/cross-layer-thinking-guide.md`、`.trellis/spec/guides/code-reuse-thinking-guide.md`。
* 当前实现入口：`backend/internal/handler/usage_handler.go`、`backend/internal/service/usage_service.go`、`backend/internal/repository/usage_log_repo.go`、`frontend/src/views/user/LeaderboardView.vue`、`frontend/src/views/admin/DashboardView.vue`。
