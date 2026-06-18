# brainstorm: token 消耗排行榜页面

## Goal

新增一个用户可访问的「排行榜」页面，让登录用户看到当日 token 消耗最高的用户列表，并明确看到自己的当日排名、token 消耗和请求次数，提升用量感知与运营展示能力。

## What I already know

* 用户希望新增页面名为「排行榜」。
* 页面核心信息是当日 token 消耗最多的用户，以及当前登录用户自己的排名。
* 用户已确认 MVP 口径：查看今日消费 Token Top10、当前账号自己的 Token 排名，榜单中用户邮箱需要打码展示。
* 当前仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，业务改动以 `custom/main` 为基线，任务分支为 `feature/token-usage-leaderboard`。
* 已有管理员侧用户消费榜接口：`/admin/dashboard/users-ranking`，实现位于 `DashboardHandler.GetUserSpendingRanking`、`DashboardService.GetUserSpendingRanking` 和 `usageLogRepository.GetUserSpendingRanking`。
* 现有管理员榜单按 `actual_cost DESC, tokens DESC` 排序，统计范围可配置；本任务需要的是“当日 token 消耗”排行榜，排序口径不同。
* 普通用户已有 `/usage/dashboard/*` 系列登录态接口，能通过 `middleware.GetAuthSubjectFromContext` 获取当前用户。
* 前端是 Vue 路由结构，用户侧已有 `/dashboard`、`/usage` 等受保护页面；侧栏个人导航通过 `buildSelfNavItems` 统一生成，管理员的“我的账户”区也复用该导航。

## Assumptions (temporary)

* “当日”默认按现有 `timezone` 用户/系统时区工具计算自然日，而不是滚动 24 小时。
* 排行榜面向所有已登录用户开放，但需要做隐私保护，避免暴露完整邮箱或敏感账号信息。
* MVP 只做当日榜单，不做历史日期切换、周榜/月榜、分组榜或导出。

## Open Questions

* 无。

## Requirements

* 新增一个受登录保护的用户侧「排行榜」页面。
* 页面展示当日 token 消耗 Top10 用户列表。
* 页面展示当前登录用户自己的当日排名；即使用户未进入 Top10，也应展示“我的排名”卡片或固定区域。
* 排序按当日总 token 数降序，请求数或用户 ID 作为稳定兜底排序。
* 榜单中用户邮箱必须打码展示；不得向普通用户返回完整邮箱。
* 后端接口只允许登录用户访问，不复用管理员接口直接暴露。
* 前端侧栏增加「排行榜」入口，并兼容普通用户主菜单与管理员个人菜单。

## Technical Approach

* 后端新增 `GET /api/v1/usage/dashboard/leaderboard`，复用用户侧 `/usage/dashboard/*` 鉴权体系。
* 仓储层按今日时间窗口聚合 `usage_logs`，排序口径为 `tokens DESC, requests DESC, user_id ASC`，返回 Top10 并在当前用户不在 Top10 时额外带回当前用户排名。
* 服务层将内部 raw row 转换为普通用户可见 DTO，并统一通过邮箱打码字段 `masked_email` 输出，避免完整邮箱进入响应。
* 前端新增 `/leaderboard` 用户页面、API 类型/方法、路由、侧栏入口、路由预加载和 zh/en 文案。

## Decision (ADR-lite)

**Context**: 用户侧排行榜需要展示跨用户统计，但普通用户不能看到其他人的完整邮箱。

**Decision**: 新增独立用户侧排行榜接口，不直接复用管理员消费榜；内部行保留原始字段但使用 `json:"-"`，服务层只输出打码邮箱。

**Consequences**: 普通用户接口的隐私边界更清晰；后续如果增加周榜/月榜，可复用同样的内部 row 与公共 DTO 分层。

## Expansion Sweep

### Future evolution

* 后续可能扩展为周榜/月榜、分组榜、模型榜或运营活动榜。
* 统计查询应保留时间窗口和 limit 的扩展空间，但 MVP 先固定为当日。

### Future admin leaderboard

后续单独实现管理员版排行榜，不并入普通用户页面，以保持普通用户隐私边界清晰。

* 管理员可查看完整邮箱、用户 ID、用户名、账号状态、注册时间等完整用户信息。
* 支持日期选择：今日、昨日、近 7 天、近 30 天、自定义日期范围。
* 指标包含请求数、非图片 Token 总量、实际扣除额度 `actual_cost`；可按需要展示标准计费 `cost` 和账号成本 `account_cost`。
* 支持 TopN 选择，例如 Top10、Top20、Top50、Top100。
* 支持按邮箱搜索，并按分组、模型、用户状态、时间范围筛选。
* 支持展开单个用户明细，查看该用户 API Key 用量、分组用量、模型用量、请求数、Token 和消耗额度。
* 明确不做 CSV 导出。

### Related scenarios

* 用户侧排行榜应与现有 `/usage/dashboard/*` 用户用量接口保持一致的鉴权和响应风格。
* 管理端现有用户消费榜继续服务管理员，不在本任务中重构。

### Failure & edge cases

* 当日无使用记录时展示空状态，并展示当前用户排名为空或未上榜。
* 当前用户未进入 Top10 时，接口仍需返回 `my_rank`，避免前端用 Top10 误判。
* 普通用户可见范围需要隐私保护，不能简单复用管理员接口里的完整邮箱。

## Acceptance Criteria

* [x] 登录用户能从侧栏进入「排行榜」页面。
* [x] 页面能展示当日 token Top10 排名、token 数、请求数。
* [x] 当前用户不在 Top10 时，页面仍能展示自己的排名和当日 token。
* [x] 未登录访问页面时走现有登录保护。
* [x] 后端接口对普通用户只返回打码后的邮箱，不暴露完整邮箱。
* [x] 排行榜为空时有明确空状态。

## Definition of Done (team quality bar)

* Tests added/updated (unit/integration where appropriate)
* Lint / typecheck / CI green
* Docs/notes updated if behavior changes
* Rollout/rollback considered if risky

## Out of Scope (explicit)

* 周榜、月榜、历史日期筛选和自定义时间范围。
* 管理端现有消费榜重构；管理员增强版排行榜作为后续独立需求记录在 `Future admin leaderboard`。
* 运营奖励、徽章、积分发放或排行榜活动规则。
* 生产数据库迁移或生产配置变更。

## Technical Notes

* 二开工作流约束：`.trellis/spec/guides/downstream-fork-workflow.md`。
* 候选后端位置：`backend/internal/handler/usage_handler.go`、`backend/internal/service/dashboard_service.go` 或用户用量相关 service、`backend/internal/repository/usage_log_repo.go`、`backend/internal/pkg/usagestats/usage_log_types.go`、`backend/internal/server/routes/user.go`。
* 候选前端位置：`frontend/src/router/index.ts`、`frontend/src/components/layout/AppSidebar.vue`、`frontend/src/api/usage.ts`、`frontend/src/views/user/`、`frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`。
* 现有管理员榜单测试可作为参考：`backend/internal/repository/usage_log_repo_request_type_test.go`、`backend/internal/handler/admin/dashboard_handler_request_type_test.go`、`frontend/src/components/charts/__tests__/ModelDistributionChart.spec.ts`。
* 需要在实现前补充 backend/frontend spec context 到 `implement.jsonl` 和 `check.jsonl`。
* 规范同步：已更新 `.trellis/spec/backend/quality-guidelines.md` 与 `.trellis/spec/frontend/type-safety.md`，记录用户可见用量统计接口的隐私与跨层契约。
