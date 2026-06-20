# brainstorm: 仪表盘运营指标扩展

## Goal

在管理员仪表盘补充更适合日常运营判断的数据，优先覆盖用户活跃、用户余额池和订阅价值，让管理员能快速判断今天是否有真实使用、昨天对比是否异常、当前钱包余额与订阅资产规模。

## What I already know

- 用户希望新增“今日和昨日的活跃用户（使用了 token）”。
- 用户希望新增“当前所有用户的余额剩余”。
- 用户希望用“订阅价值总数”替换“低余额用户数”作为第一版核心指标。
- 订阅价值总数按“剩余有效期折算后的剩余价值”统计。
- 当前后端已有管理员仪表盘接口 `GET /api/v1/admin/dashboard/stats`。
- 当前 `DashboardStats.active_users` 已表示“今日有请求的用户数”，数据来自 `usage_dashboard_daily.active_users`。
- 当前已有 `hourly_active_users`、`today_requests`、`today_tokens`、`today_actual_cost`、`today_account_cost`、`total_actual_cost`、`total_account_cost` 等统计字段。
- 用户余额字段存储在 `users.balance`，后台已有单用户余额调整能力。
- 本仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，后续实现必须遵循 `.trellis/spec/guides/downstream-fork-workflow.md`。

## Assumptions (temporary)

- “使用了 token”按成功计费/有效 usage log 口径统计，而不是仅有请求记录就算活跃。
- “今日/昨日”应沿用当前仪表盘聚合口径的时区规则，优先避免同一页面内出现 UTC 与本地日界混用。
- “当前所有用户的余额剩余”默认指未删除用户的 `balance` 总和；是否仅统计 `active` 用户待确认。
- “订阅价值总数”指当前有效订阅按剩余有效期折算后的剩余价值。
- 第一版先补总览卡片与接口字段，不做独立明细页。

## Decisions

- 余额池第一版按所有未删除用户统计，不额外过滤 `status = active`。
- token 活跃用户按 `usage_logs.actual_cost > 0` 的有效扣费用量统计；零扣费/失败占位日志不计入活跃用户。

## Requirements (evolving)

- 新增今日活跃用户展示，命名上明确是今日 token 活跃用户，避免和“当前小时活跃用户”混淆。
- 新增昨日活跃用户展示，并支持与今日活跃用户形成对比。
- 新增当前用户余额池总额展示。
- 新增订阅价值总数展示，用于表示当前有效订阅的剩余资产规模。
  - 折算公式：订阅剩余价值 = 套餐价格 × 剩余有效时长 / 套餐有效时长。
  - 汇总范围：仅统计当前有效且未删除的用户订阅。
- 建议新增今日经营差额：
  - 今日实际扣除 `today_actual_cost` 减今日账号成本 `today_account_cost`。
  - 今日经营差额率，便于判断用量增长是否真的带来正收益。
- 建议新增昨日对比数据：
  - 昨日请求数、昨日 token、昨日实际扣除。
  - 前端可展示今日相对昨日的增减幅，帮助发现突增/突降。

## Acceptance Criteria (evolving)

- [x] 管理员仪表盘能显示今日 token 活跃用户与昨日 token 活跃用户。
- [x] 管理员仪表盘能显示当前用户余额池总额。
- [x] 管理员仪表盘能显示按剩余有效期折算后的订阅价值总数。
- [x] 接口字段命名清晰，前端类型和中英文文案同步。
- [x] 统计口径在 PRD 或代码注释中说明清楚，尤其是时区、用户状态、失败请求是否计入。
- [x] 后端测试覆盖新增统计字段的聚合口径。
- [x] 前端测试覆盖新增卡片渲染或关键格式化逻辑。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 不在本任务中做新的明细钻取页面。
- 不在本任务中改变计费、扣费或余额更新逻辑。
- 不在本任务中新增数据库写入型业务流程；如需新增聚合列或迁移，应单独评估回填成本。

## Technical Notes

- 相关后端入口：
  - `backend/internal/handler/admin/dashboard_handler.go`
  - `backend/internal/pkg/usagestats/usage_log_types.go`
  - `backend/internal/repository/usage_log_repo.go`
  - `backend/internal/repository/dashboard_aggregation_repo.go`
- 相关前端入口：
  - `frontend/src/views/admin/DashboardView.vue`
  - `frontend/src/types/index.ts`
  - `frontend/src/api/admin/dashboard.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`
- `usage_dashboard_daily` 已有 `active_users`，昨日活跃用户可以优先从该表读取昨日行，避免扫描大表。
- 余额池可以从 `users` 表实时聚合，需注意是否排除已删除、禁用、冻结或测试用户。
- 订阅计划价格存储在 `subscription_plans.price`，用户订阅记录 `user_subscriptions` 只有 `group_id` 和有效期；实现时需要找到能确定“购买套餐价格与套餐有效期”的来源，优先考虑支付订单历史，其次才考虑同分组默认/在售套餐。
- 若后续实现进入 Phase 2，应将 `.trellis/spec/guides/downstream-fork-workflow.md` 保留在 `implement.jsonl` 与 `check.jsonl`。
