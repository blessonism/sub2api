# 实施计划：Token 自动策略累计用量常驻倍率

## 前置

- 从 `custom/main` 拉取 `feature/token-auto-policy-resident-rate`，不在 `main` 上直接开发。
- 实现前加载 `.trellis/spec/backend/quality-guidelines.md` 的 Token 自动策略场景与 `.trellis/spec/frontend/type-safety.md` 相关约定。

## 实施步骤

1. **数据库迁移 `backend/migrations/199_token_usage_auto_resident_rate.sql`**
   - `token_usage_auto_policy_tiers` 增加 `is_resident`，重建唯一约束 `(policy_id, is_resident, condition_mode, min_tokens, min_actual_cost)`。
   - 新建 `token_usage_auto_user_totals` 累计表。
   - `token_usage_auto_assignments` 增加 `resident_tier_id`、`last_total_token_usage`、`last_total_actual_cost`。
   - `token_usage_auto_run_changes` 增加 `total_token_usage`、`total_actual_cost`、`resident_tier_id`、`resident_tier_min_tokens`、`resident_tier_condition_mode`、`resident_tier_min_actual_cost` 及条件 CHECK。

2. **repository 层（`backend/internal/repository/token_usage_policy_repo.go`）**
   - tier 的 INSERT/SELECT/SCAN 同步 `is_resident`；assignment 与 run_changes 的 SELECT/INSERT/SCAN 同步新增列。
   - 新增 `RefreshUserUsageTotals`：按 `usage_logs.id` 水位做批量增量 upsert，返回最新累计值。
   - 单测覆盖：新用户全量回填、老用户增量、迟到落库（旧 created_at 新 id）不漏算、tier/assignment/run_changes 新列扫描。

3. **service 层（`backend/internal/service/token_usage_policy.go`）**
   - 领域模型新增字段；`normalizeTokenUsagePolicyInput` 支持 `is_resident` 与新的唯一性校验。
   - `buildPolicyChanges`：取窗口命中用户 ∪ assignment 用户 → 刷新/读取累计值 → 分维度选档位 → 生效倍率取 min → 生成变更与审计字段。
   - clear 仅发生在常驻与滚动都不命中时；手动接管/conflict_mode/封顶逻辑保持原样。
   - 单测覆盖：min 取优、滚动回落保留常驻、双维度均不命中清除、manual_priority 跳过、auto_priority 覆盖、downgrade 标签、纯滚动策略回归。

4. **handler / 前端**
   - `frontend/src/api/admin/tokenUsagePolicies.ts` 类型同步。
   - `TokenUsagePoliciesView.vue`：档位表单「常驻档位」开关、摘要/预览/历史双口径展示。
   - `frontend/src/i18n/locales/{zh,en}/custom.ts` 文案。

5. **验证与收尾**
   - `cd backend && go test ./internal/service ./internal/repository -count=1 -timeout=60s`
   - `cd frontend && pnpm typecheck`
   - `git diff --check`；确认迁移顺序、未提交无关改动、分支为 `feature/token-auto-policy-resident-rate`。
   - 按 `get_context.py --mode packages` 加载相关包 spec 的 Quality Check 做全量检查。

## 验证命令

```bash
cd backend && go test ./internal/service ./internal/repository -count=1 -timeout=60s
cd frontend && pnpm typecheck
git diff --check
```

## 风险与回滚点

- repository SQL 的 SELECT/SCAN/INSERT 参数顺序不一致会导致运行时扫描失败；新增列后必须跑对应仓储测试。
- 累计表刷新失败会使策略执行失败（不会写半成品）；可先只读校验 SQL 再上线。
- 回滚：先停用含常驻档位的策略，回滚代码；迁移为增量加列/加表，可安全保留或单独回滚。
