# brainstorm: 使用记录同 IP 多账号识别

## Goal

为管理员提供一个一键筛查能力，从使用记录中找出同一个请求 IP 对应多个不同账号的情况，用于快速识别疑似多账号共用、风控排查或运营审计场景。

## What I already know

- 用户希望“一键找出使用记录中 IP 相同的不同账号”。
- `usage_logs` 已存在 `ip_address` 字段，并已有 `idx_usage_logs_ip_address` 索引。
- 普通用户使用记录 DTO 明确不暴露 `ip_address`；管理员使用记录 DTO 会暴露 `ip_address`。
- 请求 IP 来自网关处理链路中的 `ip.GetClientIP(c)`，并在记录用量时写入 `usage_logs.ip_address`。
- 前端管理员使用记录页面已有 `ip_address` 列，但当前只是逐条展示 IP，并没有“一键找出同 IP 多账号”的聚合入口。
- 当前管理员使用记录列表支持按用户、API Key、账号、分组、模型、请求类型、计费类型、计费模式和时间范围过滤，但 `UsageLogFilters` 暂未包含 IP 过滤或同 IP 聚合筛查。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开仓库，后续实现需以 `custom/main` 为目标基线，并遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。

## Confirmed Decisions

- “不同账号”的统计口径是平台用户账号，即 `usage_logs.user_id`；不是上游渠道账号 `account_id`。
- 管理员入口采用现有使用记录列表中的快捷筛选，不新建独立筛查页或独立弹窗。
- “同 IP 多用户”判断范围复用当前管理员使用记录列表筛选条件，例如时间范围、用户、API Key、上游账号、分组、模型、请求类型和计费模式。
- 启用快捷筛选后，需要在列表顶部展示小摘要，用于快速说明命中规模。

## Assumptions (temporary)

- “使用记录”指 `usage_logs` 表及管理员使用记录页面，而不是登录日志或合规审计日志。
- 该能力仅管理员可用，不向普通用户暴露。
- MVP 更偏向“筛查/聚合结果”，而不是自动封禁、自动告警或风控处置。

## Open Questions

- 无。

## Requirements (evolving)

- 管理员可以一键查看存在同一 `ip_address` 对应多个不同平台用户账号的聚合结果。
- 管理员使用记录列表提供“同 IP 多用户”快捷筛选，点击后列表只展示命中该条件的使用记录明细。
- 快捷筛选必须与当前筛选条件组合生效，而不是绕过页面已有筛选去扫描全量历史。
- 聚合结果应排除空 IP。
- 筛选启用后，页面顶部显示小摘要，至少包含命中 IP 数、涉及用户数、命中记录数。
- 结果需要展示足够排查的信息，例如 IP、关联用户数、请求次数、最近使用时间和关联用户摘要。
- 功能应复用管理员使用记录的权限边界，避免普通用户看到 IP 或其他账号信息。

## Acceptance Criteria

- [x] 管理员能从管理后台一键进入或触发同 IP 多账号筛查。
- [x] 仅返回 `ip_address` 非空且关联不同 `user_id` 数量大于 1 的结果。
- [x] 启用快捷筛选后，管理员使用记录列表只展示属于“同 IP 多用户”命中集合的记录。
- [x] 已选择的时间范围、用户、API Key、上游账号、分组、模型、请求类型和计费模式等筛选条件仍然生效。
- [x] 启用快捷筛选后，列表顶部展示小摘要，至少包含命中 IP 数、涉及用户数、命中记录数。
- [x] 结果可帮助管理员继续定位到相关使用记录或账号。
- [x] 普通用户接口不新增 IP 或跨账号信息暴露。

## Technical Approach

- 复用管理员使用记录接口 `GET /api/v1/admin/usage`，新增查询参数 `shared_ip_users=true`。
- 后端在 `UsageLogFilters` 中增加 `SharedIPUsers`，repository 基于当前筛选条件追加同 IP 多用户子查询条件。
- 同 IP 多用户判断规则为：`ip_address IS NOT NULL`、`ip_address <> ''`，并且同一 IP 在当前筛选范围内 `COUNT(DISTINCT user_id) > 1`。
- 管理员列表响应增加 `shared_ip_users_summary`，包含 `ip_count`、`user_count`、`record_count`。
- 前端在管理员使用记录页的现有筛选工具条中增加“同 IP 多用户”快捷按钮，并在使用记录表格上方显示摘要。

## Decision (ADR-lite)

**Context**: 用户希望在现有管理员使用记录中一键找出同 IP 多平台用户账号，不需要独立页面或自动风控处置。

**Decision**: 在现有 `/admin/usage` 列表接口和页面中增加布尔筛选参数与摘要返回，不新增表、不新增迁移、不新增独立路由。

**Consequences**: 实现改动面较小，能天然复用当前筛选条件、分页、排序和管理员权限边界；代价是启用该筛选时会执行同 IP 聚合子查询，并强制使用精确分页总数，适合管理员排查场景。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 自动封禁、自动限流或自动风控处罚。
- 登录 IP 风险识别，除非后续确认“使用记录”之外也要纳入登录日志。
- 对历史空 IP 记录进行补填。

## Technical Notes

- 相关迁移：`backend/migrations/031_add_ip_address.sql`
- 管理员 DTO：`backend/internal/handler/dto/mappers.go`
- 管理员 usage 入口：`backend/internal/handler/admin/usage_handler.go`
- 过滤结构：`backend/internal/pkg/usagestats/usage_log_types.go`
- 前端管理员 usage 页面：`frontend/src/views/admin/UsageView.vue`
- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
