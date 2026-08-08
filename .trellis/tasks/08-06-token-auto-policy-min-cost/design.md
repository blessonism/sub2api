# 技术设计

## 方案概述

在现有 Token 自动策略档位上增加显式条件模式和实际消费额阈值：

- `token`：只判断窗口 Token，兼容现有策略。
- `actual_cost`：只判断窗口累计 `usage_logs.actual_cost`。
- `both`：Token 和实际消费额同时达到阈值。

每个档位保留 `min_tokens`，新增 `min_actual_cost`（美元，`DECIMAL(20,10)`）。不参与判断的字段仍保存为 0，具体由 `condition_mode` 决定，避免用空值同时表达“未启用”和“零阈值”。

## 数据流

1. `AggregatePolicyUsage` 在现有时间窗口和筛选条件下同时聚合四类 Token 与 `SUM(actual_cost)`。
2. Service 根据档位 `sort_order` 从前到后检查条件，记录最后一个满足条件的档位。
3. `buildPolicyChanges` 将 Token、实际消费额、命中模式和两个档位阈值写入预览/执行变更。
4. Repository 持久化档位条件、assignment 最近一次实际消费额，以及 run change 的实际消费额和条件信息。
5. 前端用显式模式选择器控制阈值输入，预览和历史展示两项实际指标。

## 数据库变更

新增序号迁移（不修改历史迁移文件）：

- `token_usage_auto_policy_tiers.condition_mode TEXT NOT NULL DEFAULT 'token'`，约束为 `token`、`actual_cost`、`both`。
- `token_usage_auto_policy_tiers.min_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0`，约束不小于 0。
- 删除旧的 `(policy_id, min_tokens)` 唯一约束，改为 `(policy_id, condition_mode, min_tokens, min_actual_cost)` 唯一约束，允许不同条件模式复用相同 Token 值。
- `token_usage_auto_assignments.last_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0`。
- `token_usage_auto_run_changes.actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0`。
- `token_usage_auto_run_changes.tier_condition_mode TEXT`、`tier_min_actual_cost DECIMAL(20,10)`，用于审计命中条件。

旧数据的默认 `condition_mode=token`、`min_actual_cost=0`，行为保持不变。

## 服务契约

- `TokenUsageAutoPolicyTier`、前端 `TokenUsagePolicyTier` 增加 `condition_mode` 和 `min_actual_cost`。
- `TokenUsageAutoPolicyUsageRow` 增加 `ActualCost`。
- `TokenUsageAutoPolicyChange` 增加 `ActualCost`、`TierConditionMode`、`TierMinActualCost`。
- `TokenUsageAutoAssignment` 增加 `LastActualCost`。
- `normalizeTokenUsagePolicyInput` 默认模式为 `token`，校验模式、非负有限消费额和条件组合唯一；保留请求档位顺序并重新生成 `SortOrder`。
- `selectTokenUsageTier(tiers, tokenUsage, actualCost)` 使用模式判断：`token` 检查 Token，`actual_cost` 检查消费额，`both` 检查两者。

## 兼容与边界

- 现有策略读取为 `token` 模式，不改变原有 Token-only 行为。
- 聚合仍保留 `actual_cost > 0` 的成功落账过滤；实际消费额阈值是在同一过滤后的窗口数据上求和。
- 消费额相等时视为满足条件。
- 用户 Token 为 0 但实际消费额大于 0 时，`actual_cost` 模式可以命中；这是该模式的预期用途。
- 混合条件没有跨单位的自然排序，因此以档位数组顺序/`sort_order` 作为优先级，命中最后一个满足条件的档位。前端不再按 Token 值重排档位。
- 不改变策略调度、冲突模式、清退和权限行为。

## 回滚

代码回滚前需先停止使用新模式；数据库迁移只新增列和约束，不删除历史数据。若需要回滚代码，旧代码读取新增列不受影响，但旧代码不会理解非 `token` 模式，因此上线顺序应为先迁移、后应用代码，回滚时先禁用新模式策略。
