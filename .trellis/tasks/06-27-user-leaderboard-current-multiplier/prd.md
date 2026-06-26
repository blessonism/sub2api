# fix: 用户排行榜展示当前生效倍率

## Goal

修复用户侧 Token 排行榜“阶梯倍率”展示口径，避免继续使用已过期的自动策略 assignment 快照，让用户看到的倍率与当前实际生效倍率一致。

## What I Already Know

- 最近提交让用户排行榜返回 `discount_rate_multiplier`，并用 `token_usage_auto_assignments.last_rate_multiplier` 作为自动策略命中倍率。
- 真实扣费当前倍率由 `user_group_rate_multipliers(user_id, group_id).rate_multiplier` 解析，缺失时回退分组默认倍率。
- 自动策略被禁用、或管理员手动调整用户专属倍率后，assignment 可能仍保留旧 `last_rate_multiplier`。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开，变更必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。

## Requirements

- 用户排行榜只把仍然有效的自动策略命中视为自动倍率来源。
- 自动策略倍率必须基于当前生效倍率，而不是仅基于历史 `last_rate_multiplier` 快照。
- 已禁用策略、目标组失效、人工接管或当前倍率已偏离自动策略快照时，应回退到管理员配置的通用分组倍率；未配置时回退 `1.0`。
- 普通用户接口仍只返回脱敏身份，不暴露用户 ID、邮箱或管理员字段。

## Acceptance Criteria

- [ ] 自动策略仍启用、目标组有效且当前用户倍率与自动策略快照一致时，排行榜展示该自动倍率。
- [ ] 管理员手动改动用户当前倍率后，排行榜不继续展示旧自动倍率。
- [ ] 策略禁用后，排行榜不继续展示旧自动倍率。
- [ ] 未命中自动倍率时，排行榜展示配置的通用分组倍率；未配置时展示 `1.0`。
- [ ] 后端聚焦测试通过。

## Definition of Done

- 更新 repository SQL 与相关回归测试。
- 不修改前端 UI，除非后端响应契约必须变化。
- 不触碰当前工作区已有无关前端改动。

## Technical Approach

在 `GetUserTokenLeaderboard` 的 SQL 中，将自动倍率来源改为“启用策略 + 有效目标组 + 当前用户组倍率仍等于 assignment 快照”的组合；否则让该用户走通用分组 fallback。

## Out of Scope

- 不改变自动策略执行、清退、禁用或手工接管流程。
- 不新增用户端字段。
- 不调整管理员配置 UI。

## Technical Notes

- 主要文件：`backend/internal/repository/usage_log_repo.go`。
- 测试文件：`backend/internal/repository/usage_log_repo_request_type_test.go` 或同层现有排行榜测试。
- 已读取：`.trellis/spec/backend/quality-guidelines.md`、`.trellis/spec/guides/code-reuse-thinking-guide.md`、`.trellis/spec/guides/cross-layer-thinking-guide.md`、`.trellis/spec/guides/downstream-fork-workflow.md`。
