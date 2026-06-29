# Research: 上游 Relay Token 过期刷新调查

- Query: 调查“上游倍率监控报错 `fetch upstream account balance: upstream HTTP 401: {"code":"TOKEN_EXPIRED","message":"Token has expired"}`”的本地根因与修复方向。
- Scope: internal
- Date: 2026-06-29

## Findings

### 文件与职责

- `backend/internal/service/upstream_relay_group_monitoring.go`：上游 Relay 连接器服务，包含登录、凭据解密、同步分组、刷新余额/用量、拉取上游 API Key 列表，以及 `getUpstreamJSON` 通用 GET 调用。
- `backend/internal/repository/upstream_relay_group_monitoring_repo.go`：上游 Relay 仓储，包含连接器整行创建/更新、同步状态标记、余额更新。
- `backend/internal/handler/auth_handler.go`：本仓库自身 `/api/v1/auth/refresh` handler 与响应 DTO。
- `backend/internal/service/auth_service.go`：Refresh Token 轮换逻辑，`RefreshTokenPair` 每次刷新都会生成新的 refresh token，并让旧 refresh token 失效。
- `backend/internal/server/middleware/jwt_auth.go`：JWT 过期时返回 `401 TOKEN_EXPIRED Token has expired`。
- `backend/internal/repository/aes_encryptor.go`：AES-256-GCM 加密实现，密文格式为 `base64(nonce + ciphertext + tag)`，每次加密使用随机 nonce。

### 现象与根因判断

现象路径是：上游倍率监控刷新余额时，`fetchUpstreamAccountBalance` 调用 `getUpstreamJSON(ctx, connector, "/api/v1/user/profile")`，若上游返回非 2xx，则包装为 `fetch upstream account balance: ...`。对应代码在 `backend/internal/service/upstream_relay_group_monitoring.go:1810` 与 `backend/internal/service/upstream_relay_group_monitoring.go:1840`。

根因判断：`password_login` 模式保存的是上游 Sub2API 登录拿到的 access token 和 refresh token，但后续所有上游读接口都只使用 `connector.BearerTokenPlain` 作为 Bearer access token；`getUpstreamJSON` 对 `401/TOKEN_EXPIRED` 没有 refresh/retry 分支。access token 过期后，上游 Sub2API 的 JWT 中间件会返回 `{"code":"TOKEN_EXPIRED","message":"Token has expired"}`，本地直接把它当普通上游 HTTP 错误冒泡。

本仓库 JWT 中间件也能说明该错误形态：`backend/internal/server/middleware/jwt_auth.go:52` 在 access token 过期时返回 `TOKEN_EXPIRED` 和 `Token has expired`。上游若同为 Sub2API 实例，会产生相同响应。

### 关键调用链

- 登录保存链路：
  - `normalizeConnectorInput` 在 `password_login` 且提供密码时调用 `loginUpstreamRelay`，见 `backend/internal/service/upstream_relay_group_monitoring.go:1361`。
  - `loginUpstreamRelay` POST 到上游 `/api/v1/auth/login`，成功后调用 `parseUpstreamRelayLoginToken`，见 `backend/internal/service/upstream_relay_group_monitoring.go:1546`。
  - `parseUpstreamRelayLoginToken` 支持统一 `data` 包装，并读取 `access_token`/`token` 与 `refresh_token`，见 `backend/internal/service/upstream_relay_group_monitoring.go:2775`。
  - `applyPasswordLoginToken` 归一化 access token，加密写入 `BearerTokenEncrypted`，如有 refresh token 则加密写入 `RefreshTokenEncrypted`，见 `backend/internal/service/upstream_relay_group_monitoring.go:1523`。

- 运行时使用链路：
  - `SyncConnector` 读取连接器后调用 `decryptConnector`，再调用 `fetchGroupSnapshots`；失败会 `MarkConnectorSync(... needs_reauth ...)`，见 `backend/internal/service/upstream_relay_group_monitoring.go:640`。
  - `RefreshConnectorMetrics` 读取并解密连接器，然后调用 `fetchUpstreamAccountBalance` 与 `fetchUpstreamGroupTodayUsage`，见 `backend/internal/service/upstream_relay_group_monitoring.go:701`。
  - `ListConnectorAPIKeys` 读取并解密连接器，然后调用 `fetchUpstreamAPIKeyOptions`，见 `backend/internal/service/upstream_relay_group_monitoring.go:916`。
  - `decryptConnector` 解密 bearer/refresh/login_email/cookie/user_agent 到 plain 字段，见 `backend/internal/service/upstream_relay_group_monitoring.go:1471`。
  - `fetchGroupSnapshots` 连续调用 `getUpstreamJSON` 访问 `/api/v1/groups/available` 和 `/api/v1/groups/rates`，见 `backend/internal/service/upstream_relay_group_monitoring.go:1585`。
  - `fetchUpstreamGroupTodayUsage` 通过绑定的上游 API key 逐个调用 `fetchUpstreamAPIKeyUsageStats`，后者使用 `getUpstreamJSON` 访问 `/api/v1/usage/stats`，见 `backend/internal/service/upstream_relay_group_monitoring.go:1694` 与 `backend/internal/service/upstream_relay_group_monitoring.go:1768`。
  - `fetchUpstreamAPIKeyOptions` 分页调用 `getUpstreamJSON` 访问 `/api/v1/keys`，见 `backend/internal/service/upstream_relay_group_monitoring.go:1737`。
  - `fetchUpstreamAccountBalance` 调用 `getUpstreamJSON` 访问 `/api/v1/user/profile`，见 `backend/internal/service/upstream_relay_group_monitoring.go:1810`。
  - `getUpstreamJSON` 设置 `Authorization: Bearer <BearerTokenPlain>`，可附带 cookie/user-agent；非 2xx 直接返回 `upstream HTTP <status>: <body>`，见 `backend/internal/service/upstream_relay_group_monitoring.go:1822`。

### 上游 refresh 契约

本地 Sub2API 的 refresh API 是 `POST /api/v1/auth/refresh`。路由 handler 的请求体是：

```json
{ "refresh_token": "..." }
```

`RefreshTokenResponse` 返回字段为 `access_token`、`refresh_token`、`expires_in`、`token_type`，并通过 `response.Success` 包成统一响应，因此客户端实际读取路径应为 `data.access_token`、`data.refresh_token`、`data.expires_in`、`data.token_type`。证据见 `backend/internal/handler/auth_handler.go:649`、`backend/internal/handler/auth_handler.go:663`、`backend/internal/handler/auth_handler.go:683`，统一响应格式见 `backend/internal/pkg/response/response.go:32`。

`AuthService.RefreshTokenPair` 明确实现 refresh token 轮换：注释说明“每次刷新都会生成新的 Refresh Token，旧 Token 立即失效”；代码先删除旧 refresh token，再调用 `GenerateTokenPair` 生成新 token 对。证据见 `backend/internal/service/auth_service.go:1523`、`backend/internal/service/auth_service.go:1583`、`backend/internal/service/auth_service.go:1589`。

### 当前仓储能力缺口

`UpstreamRelayRepository` 当前只有 `UpdateConnector(ctx, connector, credentialsUpdated)`，没有“只更新 access/refresh token”的专用方法。接口定义见 `backend/internal/service/upstream_relay_group_monitoring.go:521`。

仓储 `UpdateConnector` 是连接器整行更新：会写 `name/base_url/auth_mode/bearer_token_encrypted/refresh_token_encrypted/login_email_encrypted/cookie_encrypted/user_agent_encrypted/status`，并在 `credentialsUpdated=true` 时执行 `credential_version + 1`。证据见 `backend/internal/repository/upstream_relay_group_monitoring_repo.go:120`。

schema 已有 `credential_version BIGINT NOT NULL DEFAULT 1`，可用于 refresh token 落库时的乐观版本控制，见 `backend/migrations/164_upstream_relay_group_monitoring.sql:12`。但当前 `UpdateConnector` 的 `WHERE` 只有 `id=$1 AND deleted_at IS NULL`，没有按旧 `credential_version` CAS，因此不能直接解决并发 refresh 竞争。

AES-GCM 加密有随机 nonce：`Encrypt` 每次生成随机 nonce，输出 `base64(nonce + ciphertext + tag)`，见 `backend/internal/repository/aes_encryptor.go:35` 与 `backend/internal/repository/aes_encryptor.go:48`。因此同一个明文 token 每次加密后的密文也会不同，不能用“密文是否相等”判断 DB 中 token 是否已被其他 worker 更新。

### 推荐方案

建议对存在 `RefreshTokenPlain` 的连接器启用自动刷新。`password_login` 会在账密登录成功时自动保存 refresh token；`manual_session` 如果由用户从浏览器登录响应中复制了 refresh token，也应允许保存并参与同一套自动刷新。缺失 refresh token 的 `manual_session` 仍然不能自动 refresh，因为它可能只是人工粘贴的短期会话 token/cookie/user-agent 组合，本地没有可靠续期凭据。

建议把 `getUpstreamJSON` 拆成“执行一次请求”和“带 refresh 的包装调用”：

- 首次请求若成功，直接返回。
- 若返回 `401`，且 body 可识别为 `TOKEN_EXPIRED`，且连接器保存了 refresh token，则调用上游 `POST /api/v1/auth/refresh`。
- refresh 成功后解析统一 response 的 `data.access_token`、`data.refresh_token`、`data.expires_in`、`data.token_type`。
- 重新加密并落库新的 access token 和 refresh token；refresh token 可能轮换，必须同时保存新的 refresh token。
- 用新的 access token 更新当前内存中的 `connector.BearerTokenPlain`，然后原请求重试一次。
- 只重试一次，避免 refresh 接口或目标接口异常时形成循环。

refresh 失败时建议清洗错误后返回业务友好的错误；如果 refresh token 无效、过期、缺失或上游明确拒绝，应标记连接器 `needs_reauth`，让管理 UI 提示重新登录。短暂网络错误或上游 5xx 可只返回错误，不一定立即改为 `needs_reauth`。

### 并发风险

refresh token 会轮换，多个 worker 同时遇到 access token 过期时，可能同时拿旧 refresh token 调上游。一个 worker 成功后旧 refresh token 立即失效，另一个 worker 会得到 invalid/expired refresh token。

推荐新增仓储方法，例如按旧 `credential_version` 更新 token：

- 输入：connector id、旧 `credential_version`、新的 `bearer_token_encrypted`、新的 `refresh_token_encrypted`、状态/错误清理字段。
- SQL：`WHERE id=$1 AND credential_version=$oldVersion AND deleted_at IS NULL`。
- 成功时 `credential_version = credential_version + 1`，并清理 token 过期类 `last_error`。
- 若 RowsAffected 为 0，说明并发中已有别的 worker 更新；应重新读取连接器并解密，优先复用最新 token 重试原请求，而不是继续使用旧 refresh token。

不要使用密文相等做 CAS，因为 AES-GCM 随机 nonce 会让相同明文每次产生不同密文。

### 状态与 UI 影响

后端已有 `active/needs_reauth/invalid/paused` 状态常量，见 `backend/internal/service/upstream_relay_group_monitoring.go:28`。`SyncConnector` 失败时会把连接器标为 `needs_reauth`，见 `backend/internal/service/upstream_relay_group_monitoring.go:649`；`RefreshConnectorMetrics` 当前只把余额/用量错误放入结果，并不会因 `TOKEN_EXPIRED` 自动修复。

推荐 refresh 成功后保持或恢复 `active`，清理过期 token 错误；refresh 不可恢复时标记 `needs_reauth`。管理端已有 `needs_reauth` 状态展示与统计，新增后端状态变更会直接反映到现有 UI，无需先扩展新的状态枚举。前端手动会话表单需要增加可选 refresh token 输入，支持用户从浏览器登录响应中复制 `access_token + refresh_token` 的场景。

### 测试建议

- 服务单元测试：`password_login` 连接器首次 `/api/v1/user/profile` 返回 `401 TOKEN_EXPIRED`，随后 `/api/v1/auth/refresh` 返回新 access/refresh token，原请求重试一次成功，余额刷新成功。
- 服务单元测试：`fetchGroupSnapshots`、`fetchUpstreamGroupTodayUsage`、`fetchUpstreamAPIKeyOptions` 均复用同一个 refresh/retry 机制，避免只修复余额接口。
- 服务单元测试：缺失 refresh token 的 `manual_session` 遇到 `401 TOKEN_EXPIRED` 不调用 refresh，不重试，错误保持清洗后的上游错误。
- 服务单元测试：带 refresh token 的 `manual_session` 遇到 `401 TOKEN_EXPIRED` 可刷新并重试成功。
- 前端测试：手动会话模式可提交可选 refresh token 字段。
- 仓储测试：新增 token 专用更新方法只改 token/status/last_error/credential_version，不整行覆盖 name/base_url/cookie/user_agent/login_email。
- 并发测试：两个 worker 使用同一旧 `credential_version` 更新 token，只有一个成功；失败方重读后使用最新 token 重试。
- refresh 失败测试：refresh token 失效、缺失、上游返回 401/403 时标记 `needs_reauth` 并返回不含敏感 token 的错误。

## Caveats / Not Found

- 本调查只基于本地代码，不做外部网络确认；上游 refresh 契约来自本仓库自身 Sub2API 实现，适用于上游也是兼容 Sub2API 的场景。
- 未修改业务代码，也未新增测试。
- 当前未发现上游 Relay 连接器已有 token refresh/retry 实现；原有逻辑只在登录时保存 refresh token，运行时读取接口没有消费它，手动会话表单也没有显式 refresh token 输入。
