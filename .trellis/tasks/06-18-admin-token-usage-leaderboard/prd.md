# brainstorm: 管理员版 Token 排行榜

## Goal

新增独立的管理员版 Token 排行榜，帮助管理员按日期范围、用户、分组、模型和账号状态排查 Token 用量与扣费情况，同时保持普通用户排行榜的隐私边界不变。

## What I already know

* 当前仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，所有仓库改动必须遵循 `.trellis/spec/guides/downstream-fork-workflow.md`。
* 本任务 base branch 已记录为 `custom/main`，任务分支已记录为 `feature/admin-token-usage-leaderboard`。
* 原工作区 `/Users/suki/code/sub2api` 曾在 `feature/token-usage-leaderboard`，且存在用户侧排行榜相关改动；管理员实现改在干净 worktree `/Users/suki/code/sub2api-admin-token-usage-leaderboard` 完成。
* 需求来源来自 `.trellis/tasks/06-18-token-usage-leaderboard/prd.md` 的 `Future admin leaderboard`。
* 用户侧排行榜已经实现为普通用户只看今日 Token Top10、自己的排名、邮箱打码；统计口径为同一用户当日所有 Key、所有分组的非图片 Token 总和，即 `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`，不统计 `image_output_tokens`。
* 管理员版需要展示更完整的运营和排查信息，但不能导致普通用户接口返回完整邮箱。
* 现有 `/admin/dashboard/users-ranking` 必须保持可用，本任务不破坏该接口和页面。

## Assumptions

* 管理员版排行榜使用独立的管理员路由、接口和前端页面，不复用普通用户排行榜响应体。
* 时间范围按现有后端时间工具和数据库字段口径实现，日期选择项转换为明确的开始/结束时间窗口。
* 自定义日期范围只用于查询，不引入报表导出、定时任务或持久化筛选偏好。

## Requirements

* 新增独立管理员版 Token 排行榜，不混入普通用户排行榜页面。
* 管理员可查看完整邮箱、用户 ID、用户名、账号状态和注册时间。
* 支持日期选择：今日、昨日、近 7 天、近 30 天、自定义日期范围。
* 指标包含请求数、非图片 Token 总量、实际扣除额度 `actual_cost`、标准计费 `cost` 和账号成本 `account_cost`。
* 支持 TopN 选择：Top10、Top20、Top50、Top100。
* 支持按邮箱搜索，并按分组、模型、用户状态、时间范围筛选。
* 支持展开单个用户明细，查看该用户 API Key 用量、分组用量、模型用量、请求数、Token 和消耗额度。
* 支持管理员在当前排行榜前十名中勾选用户，并一键赠送余额额度。
* 批量赠送额度必须复用现有管理员余额调整和余额历史记录逻辑，避免新增不一致的账本口径。
* 批量赠送额度必须在后端校验被选用户属于相同筛选条件下的当前 Top10，不能只信任前端勾选状态。
* 管理员可控制普通用户侧排行榜可见性：`所有用户可看` 或 `仅管理员可看`。
* 当配置为 `所有用户可看` 时，普通用户侧排行榜继续只返回今日 Top10、当前用户排名和 `masked_email`。
* 当配置为 `仅管理员可看` 时，普通用户侧排行榜入口应隐藏，用户侧接口应拒绝普通用户访问；管理员版 `/admin/token-leaderboard` 始终可见。
* 管理员接口必须走管理员鉴权，不允许普通登录用户访问。
* 普通用户排行榜接口仍必须只返回 `masked_email`，不能因为管理员功能泄露完整邮箱。

## Acceptance Criteria

* [x] 管理员能打开独立的 Token 排行榜页面，不影响普通用户排行榜页面。
* [x] 管理员列表能展示完整邮箱、用户 ID、用户名、账号状态、注册时间、请求数、非图片 Token 总量和扣费指标。
* [x] 日期快捷项和自定义日期范围都能正确影响排行榜统计窗口。
* [x] TopN 选择能限制返回 Top10、Top20、Top50 或 Top100。
* [x] 邮箱搜索、分组筛选、模型筛选、用户状态筛选能组合生效。
* [x] 展开单个用户后能看到该用户 API Key、分组、模型三个维度的明细聚合。
* [x] 管理员能勾选当前排行榜 Top10 内的用户，输入赠送额度和备注后一键批量赠送。
* [x] 后端会拒绝空用户列表、非正数额度、超过 10 个用户、重复用户 ID，以及不属于当前 Top10 的用户。
* [x] 批量赠送成功后会复用单用户余额调整流水，并刷新排行榜数据。
* [x] 管理员可在系统设置中切换普通用户侧排行榜可见性。
* [x] `仅管理员可看` 时普通用户侧排行榜入口隐藏，普通用户接口拒绝访问；管理员版排行榜不受影响。
* [x] 现有 `/admin/dashboard/users-ranking` 行为不被破坏。
* [x] 普通用户排行榜响应仍不包含完整邮箱。
* [x] 明确不提供 CSV 导出入口。

## Definition of Done

* 后端相关单元或接口测试覆盖管理员排行榜聚合、筛选、TopN、明细、Top10 批量赠额校验和普通用户邮箱打码边界。
* 前端相关测试覆盖管理员页面渲染、筛选交互、TopN、Top10 勾选赠额、明细展开和普通用户页面隔离。
* 后端测试、前端测试、typecheck、lint 按变更范围运行并通过。
* 不提交、不推送、不清理数据库、不修改生产配置。

## Technical Approach

* 后端新增管理员专用接口，复用现有管理员鉴权和 usage/dashboard 代码风格，不直接扩展普通用户排行榜接口。
* 可见性配置复用现有 public settings / feature flag 链路，新增 `token_leaderboard_user_visible` 布尔开关，默认 `true`，管理员可在功能开关页修改。
* 仓储层按用户维度聚合 usage logs，非图片 Token 口径沿用用户侧排行榜：`input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`，排除 `image_output_tokens`。
* 管理员聚合查询额外关联用户基础信息，并接受时间范围、邮箱、分组、模型、用户状态和 TopN 参数。
* 明细接口或同接口的展开数据按单个用户继续聚合 API Key、分组和模型维度，避免前端自行拼装敏感明细。
* 批量赠额接口放在管理员排行榜 API 边界内，读取同一组时间/筛选参数并强制以 Top10 校验选中用户，再调用现有 `UpdateUserBalance(..., "add", notes)` 完成入账。
* 前端在管理员路由下新增页面和 API 类型，优先复用现有管理员 dashboard/usage 页面组件模式。

## Decision (ADR-lite)

**Context**: 普通用户排行榜需要严格保护邮箱隐私，而管理员版需要完整运营排查信息。

**Decision**: 管理员版排行榜采用独立管理员接口和页面；普通用户接口继续只暴露 `masked_email`，管理员接口可返回完整身份信息。

**Consequences**: 管理员功能可以扩展筛选和明细能力，不污染普通用户页面；代价是后端需要维护用户侧与管理员侧两套明确的响应契约，并用测试固定隐私边界。

## Out of Scope

* CSV 导出。
* 修改生产数据库、清理数据库或修改生产配置。
* 重构或替换现有 `/admin/dashboard/users-ranking`。
* 自动按榜单发奖、定时活动规则、通知或 CSV 导出。

## Technical Notes

* 需求来源：`.trellis/tasks/06-18-token-usage-leaderboard/prd.md` 的 `Future admin leaderboard`。
* 二开约束：`.trellis/spec/guides/downstream-fork-workflow.md`。
* 候选后端位置：`backend/internal/handler/usage_handler.go`、`backend/internal/service/usage_service.go`、`backend/internal/repository/usage_log_repo.go`、`backend/internal/pkg/usagestats/usage_log_types.go`、管理员路由文件。
* 候选前端位置：`frontend/src/api/admin/dashboard.ts`、`frontend/src/router/index.ts`、管理员 dashboard/usage 视图、i18n 文案和相关测试。
* 实现前需要确认 backend/frontend spec context 已加入 `implement.jsonl` 和 `check.jsonl`，并避免无关修改进入本任务。
* 实际后端实现：`GET /api/v1/admin/dashboard/token-leaderboard`、`POST /api/v1/admin/dashboard/token-leaderboard/grant-balance` 与 `GET /api/v1/admin/dashboard/token-leaderboard/users/:user_id/details`。
* 实际前端实现：`/admin/token-leaderboard` 独立管理员页面，侧栏入口为 `nav.tokenLeaderboard`。
