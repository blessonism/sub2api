# brainstorm: 上游倍率监控上游 token 过期自动刷新

## Goal

让上游倍率监控连接器在上游 Sub2API access token 过期后，能够使用已保存的 refresh token 自动刷新并重试一次；同时允许手动会话模式录入浏览器拿到的 refresh token，减少管理员频繁重新登录或重新导入会话的运维成本。

## What I Already Know

- 当前报错示例：`fetch upstream account balance: upstream HTTP 401: {"code":"TOKEN_EXPIRED","message":"Token has expired"}`。
- 该 401 来自被监控的上游 Sub2API JWT 中间件，不是当前管理后台浏览器登录态过期。
- 密码登录连接器创建或更新时，会调用上游 `/api/v1/auth/login` 并保存 `access_token` 与 `refresh_token`。
- 普通用户在浏览器登录受 Cloudflare/2FA 保护的上游后，也可能从登录响应或浏览器存储中拿到 `refresh_token`；当前项目的手动会话模式尚未提供显式录入 refresh token 的入口。
- 后续余额、分组、倍率、上游 API Key 选项和 usage stats 都通过 `getUpstreamJSON()` 使用已保存的 `BearerTokenPlain` 发起 GET 请求。
- `getUpstreamJSON()` 当前只发请求并返回清洗后的非 2xx 错误，没有在 401/TOKEN_EXPIRED 后自动 refresh。
- 上游 Sub2API 已有 `/api/v1/auth/refresh`，请求体为 `{ "refresh_token": "..." }`，响应通过统一 envelope 的 `data` 返回新 `access_token`、`refresh_token`、`expires_in` 和 `token_type`。
- refresh token 采用轮换机制，每次刷新都会让旧 refresh token 失效。

## Research References

- [`research/token-refresh-investigation.md`](research/token-refresh-investigation.md) — 本地代码调查、根因、刷新契约、并发风险和测试建议。

## Requirements

- 对已保存 refresh token 的连接器，在上游 GET 请求返回 401 且错误体表示 token 过期时，自动调用上游 refresh 接口。
- `manual_session` 应允许录入可选 refresh token，用于“浏览器登录后复制 access token + refresh token”的场景。
- refresh 成功后必须同时加密并持久化新的 access token 和 refresh token，并更新当前内存中的 connector 凭据。
- 原始上游 GET 请求最多重试一次，避免 refresh 失败或上游持续 401 时进入循环。
- `manual_session` 没有 refresh token 时不做自动 refresh，仍按现有逻辑返回清洗后的错误并提示重新认证；有 refresh token 时应参与同一套自动 refresh。
- refresh 失败、refresh token 缺失、refresh token 失效、用户被禁用、上游开启 backend mode 拒绝普通用户刷新等场景，应返回脱敏错误，并让连接器进入可诊断状态。
- 轻量 metrics refresh 的 partial 语义应保持：余额失败或 usage 失败不应无故升级为未处理 500。
- 全量 sync 的 `needs_reauth` 状态应继续用于表达不可恢复的认证问题。
- 自动刷新不得泄露 access token、refresh token、cookie、authorization header 或密码到前端错误消息、日志或任务文档。

## Acceptance Criteria

- [ ] 已保存 refresh token 的连接器在 `/api/v1/user/profile` 首次返回 `TOKEN_EXPIRED` 时，后端调用 `/api/v1/auth/refresh`，保存新 token，并成功重试余额请求。
- [ ] 同一刷新机制覆盖 `/api/v1/groups/available`、`/api/v1/groups/rates`、`/api/v1/keys`、`/api/v1/usage/stats` 等共用 `getUpstreamJSON()` 的调用。
- [ ] refresh token 轮换后，新 access token 和新 refresh token 均被持久化；下一次过期仍可继续刷新。
- [ ] refresh 失败时错误内容经过 `sanitizeUpstreamRelayError()` 清洗，且不会把失败的未知 usage 写成 0。
- [ ] `manual_session` 可选录入 refresh token；缺失 refresh token 时保持现有行为，不尝试调用 `/auth/refresh`。
- [ ] 并发刷新场景不会用旧 token 覆盖新 token；至少通过 `credential_version` 或连接器级锁避免明显竞态。
- [ ] 后端服务测试覆盖成功刷新重试、refresh 失败、manual_session 不刷新、并发/版本冲突和错误脱敏。
- [ ] 仓储测试覆盖专用 token 更新方法的版本条件与 `credential_version` 递增。

## Definition of Done

- 后端单元测试和仓储测试覆盖新增行为。
- 与上游倍率监控专项规范保持一致，尤其是 metrics refresh 的 partial 行为和 usage unknown 不落零。
- 下游 fork 工作流上下文已写入 implement/check。
- 如进入实现阶段，分支应基于 `custom/main`，不要混入当前前端弹窗修复分支的未提交代码。

## Out of Scope

- 不在本任务中修改上游 Sub2API 的 JWT 或 refresh token 机制。
- 不改变前端登录态刷新拦截器。
- 不重做上游倍率监控 UI；如需文案优化只作为后续小任务处理。
- 不为缺失 refresh token 的 `manual_session` 设计模拟登录、绕过 Cloudflare 或浏览器挑战机制。

## Technical Notes

- 核心服务：`backend/internal/service/upstream_relay_group_monitoring.go`
- 核心仓储：`backend/internal/repository/upstream_relay_group_monitoring_repo.go`
- 本地 refresh handler：`backend/internal/handler/auth_handler.go`
- 本地 refresh token 轮换逻辑：`backend/internal/service/auth_service.go`
- JWT 过期错误来源：`backend/internal/server/middleware/jwt_auth.go`
- 前端已有 `needs_reauth`、`last_error`、`balance_error`、`usage_error` 展示能力；本任务需要补充手动会话 refresh token 录入能力。
