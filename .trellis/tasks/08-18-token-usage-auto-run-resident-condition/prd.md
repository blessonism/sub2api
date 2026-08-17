# 修复自动策略 run_changes 常驻条件空串约束冲突

## Goal

自动 Token 策略执行在写入 `token_usage_auto_run_changes` 时，未命中常驻档位不得再因空字符串违反 `chk_token_usage_auto_run_changes_resident_condition` 而整次失败。

## Background / Confirmed Facts

- 生产报错：`pq: new row for relation "token_usage_auto_run_changes" violates check constraint "chk_token_usage_auto_run_changes_resident_condition"`。
- 迁移 `199` 约束定义为：`resident_tier_condition_mode IS NULL OR resident_tier_condition_mode IN ('token', 'actual_cost', 'both')`。
- `TokenUsageAutoPolicyChange.ResidentTierConditionMode` 是 `string`。未命中常驻档位时 `fillTokenUsagePolicyResidentInfo` 不赋值，Go 零值是 `""`。
- `insertTokenUsageRunChanges` 直接绑定该字段，未像 `user_name` / `reason` 一样做 `NULLIF(..., '')`，PostgreSQL 收到的是空串而不是 `NULL`。
- 同类问题也存在于滚动侧 `tier_condition_mode`（clear / skip_manual 且无生效滚动档位时同样是 `""`）。
- 该失败发生在 apply/audit 同一事务内，一次用户行写入失败会导致整次策略执行回滚。

## Requirements

1. 未命中常驻档位时，`resident_tier_condition_mode` 必须按 `NULL` 落库。
2. 未命中滚动档位时，`tier_condition_mode` 必须按 `NULL` 落库。
3. 命中档位时仍写入 `token` / `actual_cost` / `both`，语义不变。
4. 不改约束定义，不做数据回填（失败插入不会留下脏行）。

## Acceptance Criteria

- [x] 无常驻命中的 create/update/clear/skip 变更可以成功写入 `token_usage_auto_run_changes`。
- [x] 无滚动档位的 clear/skip 变更不会触发 `chk_token_usage_auto_run_changes_tier_condition_mode`。
- [x] 命中常驻/滚动档位时 condition_mode 仍按原值写入。
- [x] repository 单测覆盖空 condition_mode 以 `NULL` 绑定，且现有 apply/finish 审计测试通过。
