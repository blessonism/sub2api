# 用户排行榜邮箱脱敏显示

## Goal

用户侧 Token 排行榜中的邮箱只展示用户名部分的前 3 个字符和 `@` 前的最后 2 个字符，其余内容继续隐藏，降低排行榜公开信息泄露风险。

## What I Already Know

- 用户侧排行榜接口为 `/usage/dashboard/leaderboard`。
- 前端页面 `frontend/src/views/user/LeaderboardView.vue` 直接展示后端返回的 `masked_email`。
- 后端 `backend/internal/service/usage_service.go` 使用 `service.MaskEmail` 生成排行榜的 `masked_email`。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开仓库，变更需遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。

## Requirements

- 仅调整用户侧排行榜相关的邮箱脱敏展示规则。
- 邮箱用户名部分长度足够时，显示前 3 个字符和 `@` 前最后 2 个字符。
- 不在前端展示完整邮箱。
- 同步更新后端和前端现有测试样例。

## Acceptance Criteria

- [ ] 用户侧排行榜响应中的 `masked_email` 符合新展示规则。
- [ ] 当前用户排名和排行榜行使用一致的脱敏规则。
- [ ] 相关后端与前端测试通过。

## Out of Scope

- 不调整管理员侧排行榜的完整邮箱展示。
- 不调整其他登录、TOTP 或身份绑定场景的邮箱脱敏规则，除非它们共享同一个公共工具函数且测试确认行为合理。

## Technical Notes

- 前端只消费 `masked_email`，不应拿完整邮箱二次处理。
- 若公共 `MaskEmail` 被多个业务复用，需同步更新对应测试预期。
