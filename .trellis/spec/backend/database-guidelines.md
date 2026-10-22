# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

<!--
Document your project's database conventions here.

Questions to answer:
- What ORM/query library do you use?
- How are migrations managed?
- What are the naming conventions for tables/columns?
- How do you handle transactions?
-->

(To be filled by the team)

---

## Query Patterns

<!-- How should queries be written? Batch operations? -->

### 渠道定价区间的可空标签

- `channel_pricing_intervals.tier_label` 和 `channel_account_stats_pricing_intervals.tier_label` 允许 SQL NULL；旧定价迁移也可能生成这种记录。
- `batchLoadIntervals` / `batchLoadAccountStatsIntervals` 使用 `sql.NullString` 接收标签，再将 `.String` 写入 `service.PricingInterval.TierLabel`：NULL 和空串均对应空标签，非空标签原样保留。
- 不得将可空列直接扫描到 Go `string`；也不能跳过该区间或修改价格来规避错误，否则会影响渠道列表和定价缓存。
- 回归覆盖两处读取中的 NULL、空串和非空标签，并验证价格、区间边界、倍率及顺序保持原值。

---

## Migrations

<!-- How to create and run migrations -->

(To be filled by the team)

---

## Naming Conventions

<!-- Table names, column names, index names -->

(To be filled by the team)

---

## Common Mistakes

<!-- Database-related mistakes your team has made -->

(To be filled by the team)
