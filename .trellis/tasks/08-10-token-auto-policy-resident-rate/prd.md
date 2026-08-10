# Token 自动策略：累计用量常驻倍率

## Goal

为 Token 自动化阶梯策略新增「常驻档位」：按用户**全历史累计 Token 用量（全站口径）**判断，命中后给用户在目标分组一个更低的长期专属倍率作为保底；滚动档位可临时给出更低倍率，滚动回落时保留常驻倍率、不自动清除。

## Background / Confirmed Facts

- 现有策略按用户近 7/30 天滚动窗口聚合四项 Token（`input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`）与实际消费额，命中档位后写入 `user_group_rate_multipliers(user_id, group_id).rate_multiplier`。
- 档位当前条件：`condition_mode`（`token` / `actual_cost` / `both`）、`min_tokens`、`min_actual_cost`、`rate_multiplier`；档位按 `sort_order` 有序，命中最后一个满足条件的档位。
- 同一目标分组同一时间只允许一条启用策略（`idx_token_usage_auto_policies_enabled_target_group` 部分唯一索引）；策略级 `conflict_mode` 区分 `manual_priority` / `auto_priority`。
- 自动策略写入的倍率在真实扣费与排行榜展示中受 `capTokenUsageAutoRateMultiplier` 封顶（不高于分组默认/可见倍率）；手动专属倍率不受封顶。
- 自动降档/清除只处理本策略产生的 assignment（`token_usage_auto_assignments`）；`manual_takeover` 时保留手动倍率。
- 代码与文案中目前没有「常驻倍率」概念；仓库没有按用户的全历史累计聚合表。
- **已确认决策**：常驻档位为保底、滚动可更优（生效倍率取两者更低）；累计口径为全历史、全站、成功落账；常驻档位遵循 `conflict_mode`；常驻档位复用 `token` / `actual_cost` / `both` 三种条件模式。

## Requirements

1. 档位新增常驻标记（`is_resident`）。常驻档位按全历史累计、全站口径（忽略策略筛选）、成功落账口径判断，复用 `token` / `actual_cost` / `both` 条件模式。
2. 同一用户同时命中常驻与滚动档位时，生效倍率取两者更低者；仅常驻命中时使用常驻倍率；两者都不命中才执行现有清除逻辑。
3. 常驻档位遵循策略 `conflict_mode`（`manual_priority` / `auto_priority`），与滚动档位一致；手动接管时不覆盖手动倍率。
4. 常驻与滚动写入都只影响本策略产生的 assignment；显式清退只移除本策略足迹；自动倍率封顶与同一分组单启用策略约束保持不变。
5. 管理员可配置常驻档位（条件模式、累计阈值、倍率）；预览、执行历史、用户级明细展示窗口与累计双口径及各自命中侧。
6. 旧策略（无常驻档位）的命中、降档、清除行为与现状一致。

## Acceptance Criteria

- [ ] 管理员可为档位开启「常驻」，配置累计阈值与倍率（支持 `token` / `actual_cost` / `both`）。
- [ ] 常驻与滚动同时命中时生效倍率为两者更低值；滚动回落时保留常驻倍率，不自动清除。
- [ ] 常驻与滚动都不命中时，按现有语义清除本策略产生的配置。
- [ ] `manual_priority` 下常驻不覆盖手动倍率/手动接管；`auto_priority` 下可覆盖。
- [ ] 累计口径 = 全历史、全站、`actual_cost > 0` 成功落账；累计数据随策略执行增量刷新，不依赖窗口。
- [ ] 预览与执行历史返回并展示窗口用量、累计用量、生效倍率以及常驻/滚动各自阈值与条件模式。
- [ ] 纯滚动策略（无常驻档位）的命中、降档、清除行为与现状一致。
- [ ] 后端 service/repository 单测、前端 typecheck、lint 通过；迁移 `199` 可安全应用/回滚。

## Out of Scope

- 不新增第二种策略类型，不放开同一分组多启用策略约束。
- 不改变窗口聚合口径、四项 Token 定义与成功落账过滤。
- 不重做自动策略页面信息架构；不涉及上游倍率监控等其他模块。

## Technical Notes

- 迁移 `199`：新增 `is_resident`、`token_usage_auto_user_totals` 及 assignment/run_changes 常驻字段；档位唯一约束扩展为 `(policy_id, is_resident, condition_mode, min_tokens, min_actual_cost)`。
- 累计表按 `usage_logs.id` 水位增量刷新；首次遇到用户时按需全量计算。
- 任务基线 `custom/main`，实现分支 `feature/token-auto-policy-resident-rate`。
