# 技术设计：Token 自动策略累计用量常驻倍率

## 方案概述

在现有 `token_usage_auto_policy_tiers` 上新增 `is_resident` 布尔标记：

- `is_resident = false`（默认）：现有滚动档位，按近 7/30 天窗口用量判断，行为不变。
- `is_resident = true`：常驻档位，按用户**全历史累计、全站口径**（忽略策略筛选）判断，条件模式复用 `token` / `actual_cost` / `both`。

每次策略执行对同一用户分别选出「滚动命中档位」与「常驻命中档位」，**生效倍率 = min(两者命中倍率)**；仅常驻命中时使用常驻倍率；两者都不命中才清除。累计数据由新增的增量累计表维护，避免每次执行扫描全量 `usage_logs`。

## 数据流

1. 新增 `token_usage_auto_user_totals(user_id, total_tokens, total_actual_cost, last_processed_id, updated_at)`，按用户保存全站累计值与 `usage_logs.id` 水位。
2. 策略执行时先取「窗口聚合命中的用户 ∪ 已有 assignment 的用户」，批量刷新累计表（只处理 `ul.id > last_processed_id` 的增量），再读取最新累计值。
3. `buildPolicyChanges` 分维度选档位：滚动侧用窗口用量，常驻侧用累计值；生效倍率取 min；写入 assignment 与 run_changes 时同时记录窗口、累计双口径。
4. 前端每个档位增加「常驻档位」开关；摘要、预览、执行历史展示窗口与累计双口径。

## 数据库变更（迁移 199）

```sql
ALTER TABLE token_usage_auto_policy_tiers
    ADD COLUMN IF NOT EXISTS is_resident BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE token_usage_auto_policy_tiers
    DROP CONSTRAINT IF EXISTS uq_token_usage_auto_policy_tiers_condition;

ALTER TABLE token_usage_auto_policy_tiers
    ADD CONSTRAINT uq_token_usage_auto_policy_tiers_condition
    UNIQUE (policy_id, is_resident, condition_mode, min_tokens, min_actual_cost);

CREATE TABLE IF NOT EXISTS token_usage_auto_user_totals (
    user_id            BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_tokens       BIGINT NOT NULL DEFAULT 0,
    total_actual_cost  DECIMAL(20,10) NOT NULL DEFAULT 0,
    last_processed_id  BIGINT NOT NULL DEFAULT 0,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE token_usage_auto_assignments
    ADD COLUMN IF NOT EXISTS resident_tier_id BIGINT NULL
        REFERENCES token_usage_auto_policy_tiers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS last_total_token_usage BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_total_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0;

ALTER TABLE token_usage_auto_run_changes
    ADD COLUMN IF NOT EXISTS total_token_usage BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS resident_tier_id BIGINT NULL
        REFERENCES token_usage_auto_policy_tiers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS resident_tier_min_tokens BIGINT NULL,
    ADD COLUMN IF NOT EXISTS resident_tier_condition_mode TEXT NULL,
    ADD COLUMN IF NOT EXISTS resident_tier_min_actual_cost DECIMAL(20,10) NULL;

ALTER TABLE token_usage_auto_run_changes
    DROP CONSTRAINT IF EXISTS chk_token_usage_auto_run_changes_resident_condition;

ALTER TABLE token_usage_auto_run_changes
    ADD CONSTRAINT chk_token_usage_auto_run_changes_resident_condition
    CHECK (resident_tier_condition_mode IS NULL
           OR resident_tier_condition_mode IN ('token', 'actual_cost', 'both'));
```

语义约定：

- `token_usage_auto_assignments.tier_id` 保持为「最终生效档位」（min 那一侧）；`resident_tier_id` 记录常驻侧命中档位（未命中为 NULL）。
- `token_usage_auto_run_changes` 的 `tier_*` 字段沿用现有含义记录生效档位；新增 `total_*` 与 `resident_tier_*` 记录累计口径和常驻侧命中详情。
- 迁移只加列/表，不做全量回填；累计表由策略执行按需增量填充，避免大表迁移锁表。

## 累计表刷新 SQL（repository）

```sql
WITH delta AS (
    SELECT ul.user_id,
           COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0)
                        + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)), 0)::bigint AS add_tokens,
           COALESCE(SUM(ul.actual_cost), 0)::double precision AS add_cost,
           MAX(ul.id) AS max_id
    FROM usage_logs ul
    JOIN users u ON u.id = ul.user_id AND u.deleted_at IS NULL
    LEFT JOIN token_usage_auto_user_totals t ON t.user_id = ul.user_id
    WHERE ul.user_id = ANY($1)
      AND ul.actual_cost > 0
      AND (t.user_id IS NULL OR ul.id > t.last_processed_id)
    GROUP BY ul.user_id
)
INSERT INTO token_usage_auto_user_totals (user_id, total_tokens, total_actual_cost, last_processed_id, updated_at)
SELECT user_id, add_tokens, add_cost, max_id, NOW() FROM delta
ON CONFLICT (user_id) DO UPDATE SET
    total_tokens      = token_usage_auto_user_totals.total_tokens + EXCLUDED.total_tokens,
    total_actual_cost = token_usage_auto_user_totals.total_actual_cost + EXCLUDED.total_actual_cost,
    last_processed_id = GREATEST(token_usage_auto_user_totals.last_processed_id, EXCLUDED.last_processed_id),
    updated_at        = NOW()
RETURNING user_id, total_tokens, total_actual_cost;
```

要点：

- 以 `usage_logs.id`（BIGSERIAL）作为增量水位，而不是 `created_at`，避免异步落库导致的旧时间戳行漏算。
- 首次遇到用户时 `last_processed_id = 0`，等价于全历史计算一次；依赖现有 `idx_usage_logs_user_id` / `idx_usage_logs_user_created`。
- 刷新在策略执行的只读计算阶段执行（独立事务），失败则本次执行失败，不产生半成品变更。
- 刷新在事务内使用 `pg_advisory_xact_lock` 串行化，防止不同策略并发执行时基于同一水位重复累加同一批日志。

## 服务契约

新增/修改字段：

- `TokenUsageAutoPolicyTier`：新增 `IsResident bool`（JSON `is_resident`）。
- `TokenUsageAutoPolicyUsageRow`：新增 `TotalTokenUsage int64`、`TotalActualCost float64`。
- `TokenUsageAutoPolicyChange`：新增 `TotalTokenUsage int64`、`TotalActualCost float64`、`ResidentTierID *int64`、`ResidentTierMinTokens *int64`、`ResidentTierConditionMode string`、`ResidentTierMinActualCost *float64`。
- `TokenUsageAutoAssignment`：新增 `ResidentTierID *int64`、`LastTotalTokenUsage int64`、`LastTotalActualCost float64`。
- 新增服务侧结构 `TokenUsageAutoUserTotal`（UserID / TotalTokenUsage / TotalActualCost）。
- 新增 repository 方法 `RefreshUserUsageTotals(ctx, userIDs []int64) (map[int64]TokenUsageAutoUserTotal, error)`。

输入校验（`normalizeTokenUsagePolicyInput`）：

- `IsResident` 默认 `false`；校验与现有条件模式/阈值校验一致。
- 唯一性键改为 `(condition_mode, min_tokens, min_actual_cost, is_resident)`，常驻与滚动档位可复用相同阈值。
- 允许纯滚动策略（全部非常驻）、纯常驻策略或混合；空档位仍拒绝。

## 档位选择与变更判定

- `selectTokenUsageTier(tiers, tokenUsage, actualCost)` 只遍历非常驻档位，逻辑不变（最后一个命中生效）。
- 新增 `selectResidentTier(tiers, totalTokenUsage, totalActualCost)` 遍历常驻档位，条件模式判断与滚动侧一致。
- 生效倍率：两侧都命中取 `min`；只命中一侧取该侧；两者都不命中 → 沿用现有 clear 语义。
- 生效档位（`tier_id`）记录 min 那一侧；两侧倍率相等时优先记常驻侧（稳定、可解释）。
- `isDowngradeTier` 保持现有 sort_order 比较：常驻/滚动档位都在同一 `tiers` 数组内有序，跨维度降档可自然表达（例如从更高倍率滚动档位回到常驻保底档位即 sort_order 变小）。
- 手动接管（skip_manual）分支位于选档位之前，天然对两个维度生效；`manual_priority` 下常驻不覆盖手动倍率。
- 自动倍率封顶 `capTokenUsageAutoRateMultiplier` 与显式清退逻辑不改：常驻倍率是自动策略产物，写入后同样受封顶、只按 assignment 足迹清除。

## 前端

- `frontend/src/api/admin/tokenUsagePolicies.ts`：`TokenUsagePolicyTier` 增加 `is_resident: boolean`；`TokenUsagePolicyChange` 增加 `total_token_usage`、`total_actual_cost`、`resident_tier_*` 字段。
- `TokenUsagePoliciesView.vue` 档位表单：每行增加「常驻档位」开关；开启后文案说明「按全历史累计用量判断，长期保底」；条件模式与阈值输入保持不变。
- 档位摘要：常驻档位显示「累计」前缀（如「累计 Token ≥ 50M → 0.7x」），滚动档位显示「近 N 天」前缀。
- 预览/执行历史明细：新增「累计用量」「常驻阈值/条件」「生效倍率」列；保留现有窗口用量与生效档位列。
- i18n：`zh/custom.ts` 与 `en/custom.ts` 同步新增 `admin.tokenUsagePolicies` 下常驻相关文案。

## 兼容与回滚

- 旧策略 `is_resident = false`：`selectTokenUsageTier` 只遍历非常驻档位，命中结果与现状一致；clear/降档/手动接管路径不变。
- 回滚顺序：先停用含常驻档位的策略，再回滚代码；迁移只新增列/表与约束，删除常驻档位配置后旧代码可正常运行。
- 若需彻底回滚数据库，删除 `token_usage_auto_user_totals` 及新增列即可，不触碰历史数据。

## 风险与缓解

- 累计表首次回填：对老用户首次计算会做一次全历史聚合；按用户 ID 批量 + 增量水位控制，后续执行只处理增量。
- run_changes / assignments 的 SELECT/SCAN 与 INSERT 列顺序：新增列必须同步 repository 中所有读写位置（参照迁移 198 的实现经验），并用仓储单测覆盖扫描。
- 双维度 changeType：滚动/常驻切换时降档判定依赖档位 sort_order；若管理员乱序配置（阈值/倍率不单调），审计标签可能不如 rate 比较直观，但不影响实际倍率正确性。
