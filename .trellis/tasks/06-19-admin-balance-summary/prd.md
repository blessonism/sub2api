# 管理员用户余额汇总与内部用户排除

## Goal

新增管理员侧“一键统计用户余额”能力，让运营者可以快速看到当前非删除用户的余额总额，并通过独立排除名单剔除内部账号，得到更接近真实外部用户余额负债的口径。

## Requirements

- 新增管理员接口 `GET /api/v1/admin/dashboard/balance-summary`，返回非删除用户余额汇总、纳入/排除人数、按角色与状态拆分的小计，以及排除名单明细。
- 新增管理员接口 `PUT /api/v1/admin/dashboard/balance-summary/exclusions`，保存内部用户排除名单，输入为用户 ID 列表。
- 排除名单持久化到现有 `settings` 表，设置项使用 JSON 数组，不新增用户字段、不新增数据库迁移。
- 统计范围默认包含 `active` 与 `disabled` 的非删除用户，排除软删除用户；不自动排除管理员角色。
- 前端新增 `/admin/balance-summary` 页面与管理员侧栏入口，展示刷新统计、汇总卡片、角色/状态拆分、排除名单维护与统计口径说明。
- 排除名单编辑区复用管理员用户搜索能力，支持添加、移除、保存后刷新统计。

## Acceptance Criteria

- [x] 管理员可以打开余额汇总页面并点击刷新看到最新统计。
- [x] 排除名单为空时，统计所有非删除用户余额。
- [x] 排除名单包含用户时，这些用户不计入余额总额，并显示在排除名单明细中。
- [x] 排除名单中已删除或不存在的用户不影响汇总，并在响应中标识为无效项供前端提示清理。
- [x] 保存非法、重复或不存在用户 ID 时返回明确错误。
- [x] 前端展示总余额、纳入人数、排除人数、active/disabled 拆分与 admin/user 拆分。
- [x] 后端、前端定向测试与类型/格式检查通过。

## Definition of Done

- Tests added/updated for backend service/handler behavior and frontend page/API behavior.
- Targeted lint/typecheck/test commands pass.
- Trellis task context validates.
- No production data, deployment config, secret, or unrelated working-tree changes are modified.

## Technical Approach

- 在后端新增余额汇总服务方法，读取 `settings` 表中的 `admin_balance_summary_excluded_user_ids`，校验并规范化 user IDs。
- 在仓储层新增聚合查询，基于 `users` 表一次性读取 `deleted_at IS NULL` 用户并在 Go 侧汇总，避免分页累加遗漏。
- 响应 DTO 明确区分 included/excluded/invalid exclusion，金额统一按现有余额口径返回 number，前端显示两位小数。
- 前端新增 admin API 封装、路由、页面和 i18n 文案，页面使用现有 `AppLayout`、卡片、按钮、搜索输入和管理员用户搜索接口模式。

## Decision (ADR-lite)

**Context**: 内部用户不是固定等同于管理员角色，且当前用户模型已有备注但没有可靠内部用户字段。

**Decision**: 使用独立排除名单并保存到 `settings` 表。

**Consequences**: 不需要迁移，风险低；名单需要管理员维护，未来如内部用户成为通用用户属性，可迁移为显式字段或用户属性。

## Out of Scope

- 不做 CSV/Excel 导出。
- 不做历史快照、定时统计或趋势图。
- 不做分组筛选或充值流水重算。
- 不自动按角色、备注关键词或邮箱域名排除内部用户。
- 不修改生产数据库或部署配置。

## Technical Notes

- 当前仓库是 `Wei-Shaw/sub2api` 下游二开，任务 base branch 为 `custom/main`，工作分支为 `feature/admin-balance-summary`。
- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 相关现有代码：管理员 dashboard 路由、用户列表/搜索接口、`settings` 表与 `SettingRepository`、前端管理员用户管理页。
