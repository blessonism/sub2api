# Token 自动化阶梯策略最低花费条件

## Goal

为 Token 自动化阶梯策略增加可选的最低条件，避免用户通过低价模型制造大量 Token 触发高倍率档位。管理员可为每个档位选择只使用 Token 门槛、只使用实际消费额门槛，或同时使用两项；档位只要求已启用的条件满足即可命中。

## Confirmed Facts

- 现有策略按用户最近 7 天或 30 天滚动窗口聚合 `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`。
- 聚合只统计 `usage_logs.actual_cost > 0` 的成功落账请求；当前 `actual_cost` 只用于过滤，不参与档位判断。
- 档位目前只有 `min_tokens` 和 `rate_multiplier`，服务层选择满足 Token 阈值的最高档位。
- 策略支持预览、手动/定时执行、自动降档/清除、执行历史和用户级变更明细。
- 相关实现集中在 `backend/internal/service/token_usage_policy.go`、`backend/internal/repository/token_usage_policy_repo.go`、`backend/migrations/154_token_usage_auto_group_multiplier.sql`、`backend/migrations/157_token_usage_auto_run_changes.sql` 及管理员页面/API 类型。
- 本仓库是下游二开仓库，任务基线为 `custom/main`，实现分支为 `feature/token-auto-policy-min-cost`。

## Requirements

1. 每个阶梯档位可分别配置 `min_tokens` 和最低实际消费额条件；管理员可只填其中一项，也可同时填写两项。
2. 档位至少启用一项条件；命中时只校验已配置的条件：`token_usage >= min_tokens` 和/或 `actual_cost >= min_cost`。
3. 最低实际消费额使用统计窗口内用户的 `usage_logs.actual_cost` 累计值，单位沿用现有成本 API 的美元数值；不使用用户当前钱包余额 `users.balance`。
4. 通过档位条件模式明确启用 Token、实际消费额或两者；旧策略默认为 Token-only 模式，继续按现有行为运行。
5. 当用户不满足某个高档位的已启用条件时，继续匹配更低且满足其已启用条件的档位；若无档位满足，则按现有语义清除本策略产生的配置。
6. 预览、执行历史和用户级变更明细必须能解释实际使用的 Token、实际消费额、启用的条件和命中档位阈值。
7. 创建/更新校验已配置的最低条件必须是有限且不小于 0 的数值；档位必须至少配置一项条件，Token 阈值唯一的约束保持不变。
8. 保持现有策略筛选条件、冲突处理、调度、手动清退、落库和权限行为不变。
9. 前端策略表单、摘要、预览/历史类型和展示同步支持两种可选条件，中英文文案保持一致。

## Acceptance Criteria

- [ ] 管理员可以只配置 Token 条件、只配置实际消费额条件，或同时配置两项。
- [ ] 档位只按已配置条件判断；两项都配置时必须同时满足。
- [ ] 各已启用条件恰好达到阈值时视为满足，低于阈值时视为不满足。
- [ ] 旧策略未配置余额/花费条件时，行为与当前 Token-only 策略一致。
- [ ] 窗口聚合使用成功落账请求的实际消费额累计值，并与 Token 聚合使用相同筛选范围。
- [ ] 预览与执行历史返回并展示用户实际 Token、实际消费额、档位已启用条件及对应阈值。
- [ ] 非法最低条件（负数、NaN、无穷大）被后端拒绝；前端不能提交明显非法值。
- [ ] 现有自动策略的降档、清除、手动优先、自动优先和并发执行行为不回归。
- [ ] 相关后端单元/仓储测试及前端类型检查通过。

## Out of Scope

- 不新增独立的策略级全局门槛；可选条件只属于档位，与现有阶梯模型对齐。
- 不改变 `actual_cost` 的成功落账判定，也不引入新的价格或模型成本来源。
- 不重做 Token 自动策略页面的信息架构，不修改无关的策略执行行为。
