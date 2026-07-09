# leaderboard auto rate cap

## Goal

当 Token 排行榜自动阶梯写入的用户专属分组倍率高于分组倍率时，真实扣费与用户侧排行榜展示都使用更低的分组倍率，避免自动阶梯把用户成本抬高。

## Requirements

* 仅限制 `token_usage_auto_assignments` 对应的自动阶梯倍率；管理员手动设置的用户专属倍率保持现有覆盖语义。
* 真实扣费路径读取到自动阶梯倍率时，最终倍率取 `min(自动阶梯倍率, 分组默认倍率)`。
* 用户可见倍率在未显式配置用户专属可见倍率、回退到自动阶梯真实倍率时，也不得高于分组侧有效可见倍率。
* 批量图片计价中绕过 resolver 的用户专属倍率读取也要应用相同封顶规则；独立图片倍率模式不受影响。
* 用户 Token 排行榜返回的 `discount_rate_multiplier` 与封顶后的可见倍率语义一致。

## Acceptance Criteria

* [ ] 自动阶梯倍率 `0.8`、分组倍率 `0.6` 时，真实扣费倍率为 `0.6`。
* [ ] 自动阶梯倍率 `0.6`、分组倍率 `0.8` 时，真实扣费倍率为 `0.6`。
* [ ] 手动用户专属倍率 `0.8`、分组倍率 `0.6` 时，真实扣费倍率仍为 `0.8`。
* [ ] 排行榜展示不会显示高于分组有效倍率的自动阶梯倍率。
* [ ] 批量图片非独立倍率模式使用封顶后的分组倍率快照。

## Definition of Done

* 后端服务与仓储测试覆盖自动阶梯封顶、手动倍率不封顶、可见倍率 fallback、排行榜 SQL、批量图片路径。
* 聚焦测试在单次命令 60 秒内完成。
* 不修改前端 API 字段，不新增数据库迁移，不触碰无关抽奖倒计时变更。

## Technical Approach

* 使用可选内部接口判断某个 `user_id + group_id + rate_multiplier` 是否来自启用中的自动阶梯 assignment，避免扩大 `UserGroupRateRepository` 主接口。
* 在 resolver 内部封装自动阶梯封顶 helper，真实倍率和可见倍率 fallback 共用同一判断语义。
* 在批量图片路径按需使用同一可选接口完成封顶。
* 排行榜 SQL 的自动阶梯 CTE 对自动阶梯有效可见倍率与分组有效可见倍率取 `LEAST`。

## Decision (ADR-lite)

**Context**: 自动阶梯倍率写入的是用户专属分组倍率表，当前通用倍率解析会让它覆盖分组倍率，可能导致排行榜奖励反而抬高用户成本。

**Decision**: 只对可识别为自动阶梯 assignment 的倍率按分组倍率封顶，手动专属倍率保留原能力。

**Consequences**: 自动阶梯倍率的持久化值仍表示阶梯命中结果，但真实扣费和排行榜展示会在读取时封顶；未来如果需要在写入时归一化，可另开任务处理审计语义。

## Out of Scope

* 不改变管理员手动用户专属倍率语义。
* 不调整前端展示组件或 API 字段。
* 不修改数据库结构或历史数据。

## Technical Notes

* 相关规范：`.trellis/spec/backend/quality-guidelines.md`、`.trellis/spec/guides/downstream-fork-workflow.md`。
* 重点入口：`backend/internal/service/user_group_rate_resolver.go`、`backend/internal/service/batch_image_public.go`、`backend/internal/repository/usage_log_repo.go`。
