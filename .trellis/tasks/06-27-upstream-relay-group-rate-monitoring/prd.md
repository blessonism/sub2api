# brainstorm: 上游中转站分组倍率监控与托管

## Goal

把上游中转站（同样是 sub2api 实例）的分组倍率、账号可用性和调度优先级管理做成系统内置能力：系统持续监控上游各分组的真实倍率与健康状态，生成可解释的优先级调整方案，支持管理员一键应用，也支持按策略自动托管。

## What I already know

- 用户的上游不是普通单账号 API，而是另一个 sub2api 中转站。
- 用户关心的对象不是单纯账号余额扣费，而是上游中转站中的“各个分组倍率情况”和“可用性情况”。
- 用户希望系统能给出一键修改优先级的配置。
- 用户希望后续支持自动托管，而不是只手动运行一次校准。
- 上游中转站无法提供管理员 API，不能直接读取上游后台的分组、倍率、账号状态或真实调度状态。
- 用户可以为每个上游候选通道提供一个独立的本地上游账号和 API Key，因此系统可以对指定候选执行稳定的黑盒探测，而不是被上游随机调度到未知分组。
- 这些独立 API Key 在上游 sub2api 中已经分别绑定到固定上游分组，因此“一个本地候选账号”可以视为“一个可监控的上游分组候选”。
- 如果上游新增分组，系统无法自动发现；必须先为新上游分组创建/提供独立 API Key，再在本地新增候选通道配置。
- 实测上游 `GET /api/v1/groups/available` 支持用户登录态读取当前用户可见分组列表，返回 `id`、`name`、`platform`、`rate_multiplier` 等字段，可作为分组倍率的主事实源。
- 实测上游 `GET /api/v1/groups/rates` 支持用户登录态读取当前用户专属分组倍率覆盖；返回空对象 `{}` 表示没有专属覆盖，此时使用 `/groups/available` 中的 `rate_multiplier`。
- 实测上游 `GET /v1/usage` 支持普通网关 API Key 自查用量，返回 `cost` 与 `actual_cost`；同一 Key 的增量 `cost / actual_cost` 可作为登录态不可用时的候选有效倍率兜底读数。
- 普通 `sk-*` 网关 Key 不能访问 `/api/v1/groups/available` 和 `/api/v1/groups/rates`；这两个接口需要用户登录态，不需要上游管理员 API。
- 旧的一次性成本校准方向已经废弃；当前保留并推进的是上游 sub2api 分组倍率、健康探测和自动托管方向。

## Assumptions (temporary)

- “上游中转站”会作为本系统里的一个或多个上游账号/渠道存在。
- 上游 sub2api 不需要管理员 API；系统通过上游用户登录态读取 `/api/v1/groups/available` 与 `/api/v1/groups/rates`，把用户可见分组倍率作为主事实源。
- 对没有 Turnstile / 2FA 的上游，可以支持账号密码登录并保存返回的 token；对开启 Turnstile / Cloudflare 浏览器挑战的上游，后台服务不尝试绕过挑战，改由管理员在浏览器完成登录后粘贴登录态。
- 候选 Key 的 `/v1/usage` 成本增量倍率采样只作为兜底或交叉校验，不作为主事实源。
- 每个候选通道应绑定一个本地 `Account`，这个账号持有专用上游 API Key；探测任务必须强制使用该账号发起请求。
- 每个候选通道需要显式映射到上游 `group_id`；系统用登录态读到的 `group_id -> rate_multiplier` 表为候选补全倍率。
- 候选通道的覆盖范围以本地配置为准；上游新增但未配置 API Key 的分组不进入监控、建议或自动托管。
- 本系统的建议/托管动作可以写入本地 `account_groups.priority`，也可以在需要临时摘出调度时写入本地账号级 `accounts.status` / `accounts.schedulable`。
- 自动托管需要可回滚、可审计、可暂停，不能默认静默修改生产调度。
- 自动托管动作需要按错误类型灵活配置：用户可以选择某类错误只降低 priority，也可以选择临时禁用/摘出调度。
- 临时禁用后的候选仍需要继续参与后台探测；达到配置的连续成功次数后，系统自动恢复原 `status` / `schedulable`。

## Requirements (evolving)

- 通过本地配置维护“上游站点连接器”：包括 `base_url`、认证方式、登录态凭证、最近同步状态、最近分组倍率快照。
- 上游站点连接器支持两种认证方式：
  - `manual_session`：管理员从浏览器登录后的请求中粘贴 `Authorization` token、必要的 Cookie（例如 `cf_clearance`）和 User-Agent；推荐用于 Turnstile / Cloudflare / 2FA 场景。
  - `password_login`：保存上游账号密码并由系统调用 `/api/v1/auth/login` 换取 token；仅用于没有 Turnstile / 2FA 的上游，失败时回退到 `manual_session`。
- `password_login` 第一版采用更安全的“当次换取 token”模式：管理员提交上游邮箱和密码后，后端立即调用上游 `/api/v1/auth/login`，成功后保存加密的 `access_token`/`refresh_token`，默认不长期保存明文或加密密码。
- `password_login` 遇到 Turnstile、2FA、Cloudflare 浏览器挑战、账号禁用、登录失败或响应缺少 `access_token` 时，连接器不得标记为可用，应返回可读错误并提示切换 `manual_session`。
- `password_login` 换得 token 后仍必须立即同步 `/api/v1/groups/available` 与 `/api/v1/groups/rates`；只有同步成功才允许连接器进入 `active`。
- 通过本地配置维护“上游候选通道”的逻辑映射：一个候选对应一个独立本地上游账号/API Key、一个上游 `group_id`、一个本地 `account_groups.priority` 写入目标，以及期望模型/协议。
- 提供候选通道的手动新增/停用能力，用来覆盖上游新增、下架或换绑分组的情况。
- 通过登录态自动同步上游分组倍率：调用 `/api/v1/groups/available` 获取用户可见分组及默认可见倍率，调用 `/api/v1/groups/rates` 获取当前用户专属覆盖倍率。
- 倍率合并规则：若 `/groups/rates` 中存在对应 `group_id`，使用专属覆盖倍率；否则使用 `/groups/available` 返回的 `rate_multiplier`。
- 倍率采样需要记录来源与置信度：`login_available_groups` 表示来自 `/groups/available`，`login_user_group_rates` 表示来自 `/groups/rates` 覆盖，`usage_cost_delta` 表示由 `/v1/usage` 增量兜底推导。
- 当 `actual_cost` 增量为 0、用量日志尚未刷新、请求被缓存/拒绝或样本混入非探测请求时，本次倍率读数应标记为样本不足，不应用于自动调价。
- 监控上游候选可用性：定期发轻量测试请求，记录成功率、延迟、错误类型、是否限流或疑似余额不足。
- 生成优先级建议：基于倍率、可用性、稳定性和延迟，给出本地账号分组 priority 调整建议。
- 一键应用建议：管理员确认后批量更新本地 `account_groups.priority`，并保留原值、新值、操作者、原因和时间。
- 自动托管：按策略定时运行监控与调整，支持按错误类型配置动作、阈值、冷却时间、最大调整幅度、失败保护、手动暂停。
- 错误动作策略：对限流、余额不足、模型不可用、认证失败、网络超时、5xx 等错误类别，允许分别配置“仅告警”“降低 priority”“临时禁用/摘出调度”“多次探测成功后自动恢复”等动作。
- 临时禁用/摘出调度的落点为本地账号级状态：更新 `accounts.status` 或 `accounts.schedulable`；priority 调整仍用于可用候选之间的排序优化。
- 自动恢复：被系统临时禁用的候选继续按恢复探测频率运行；连续成功达到阈值后恢复禁用前的 `accounts.status` / `accounts.schedulable`，并记录恢复审计。
- 可观测性：展示当前状态、最近监控结果、建议原因、自动托管动作历史。

## Phased Delivery

- Phase 1：交付可人工掌控的完整闭环，包括 `manual_session` 上游登录态连接器、分组倍率同步、候选通道到上游 `group_id` 的手动映射、倍率快照展示、手动刷新、候选 API Key 基础健康探测、priority 建议和一键应用。
- Phase 2：补强探测与建议质量，包括错误类型归因、连续失败/连续成功计数、延迟与成功率窗口、`/v1/usage` 成本增量兜底和更细粒度的建议原因。
- Phase 3：补充 `password_login` 认证方式；当登录失败、遇到 Turnstile / 2FA 或 token 失效时，提示切换或刷新 `manual_session`。
- Phase 4：实现自动托管策略引擎，按错误类型和倍率变化自动执行 priority 降权、账号临时禁用、连续成功恢复和审计记录。

## Phase 3 Planning: Password Login Connector

### Goal

让无 Turnstile、无 2FA、无 Cloudflare 浏览器挑战的上游 sub2api 可以直接用账号密码初始化连接器，减少管理员手动复制 Bearer token 的步骤；仍然保持“保存即验证并同步倍率”的安全闭环。

### Proposed MVP

- 前端连接器表单增加认证模式切换：
  - `manual_session`：沿用 Bearer token、Cookie、User-Agent。
  - `password_login`：显示上游邮箱、上游密码；隐藏 Bearer token 输入。
- 后端 `UpstreamRelayConnectorInput` 增加 `login_email`、`login_password`、可选 `turnstile_token` 字段，但第一版不在后台尝试绕过 Turnstile。
- 后端 `normalizeConnectorInput` 允许 `auth_mode=password_login`，并在保存前/保存后立即调用上游登录：
  - 请求：`POST {base_url}/api/v1/auth/login`
  - Body：`{"email":"...","password":"..."}`
  - 成功条件：响应中存在 `access_token`，且不是 `requires_2fa=true`。
  - 成功后把 `access_token` 写入现有 `bearer_token_encrypted`；若有 `refresh_token`，新增字段加密保存。
- 连接器响应只展示状态：`has_bearer_token`、`has_refresh_token`、脱敏邮箱；不回显密码、access token 或 refresh token。
- 登录成功后立即调用现有倍率同步流程；同步失败时连接器保持 `needs_reauth` 或 `invalid`，并记录脱敏后的错误。

### Backend Changes

- Migration:
  - `upstream_relay_connectors` 新增 `login_email_encrypted TEXT` 或 `login_email_masked TEXT`。
  - 新增 `refresh_token_encrypted TEXT`。
  - 不新增 `password_encrypted`，除非后续明确要后台自动重登。
- Service:
  - 新增 `loginUpstreamRelay(ctx, connector, email, password)`，复用现有 HTTP client 和错误脱敏。
  - 解析普通登录成功响应、2FA 响应、Turnstile/403/HTML challenge 响应。
  - `SyncConnector` 优先使用已保存 access token；后续可加 refresh token 自动刷新。
- Repository:
  - 创建/更新连接器时持久化新增 token/email 字段。
  - `credential_version` 在账号密码换 token 成功时递增。
- Handler:
  - 接收 `auth_mode=password_login` 表单输入。
  - 所有错误继续走 `response.ErrorFrom`，避免泄露密码/token。

### Frontend Changes

- `UpstreamRelayConnectorInput` 增加 `login_email`、`login_password`。
- 连接器表单增加认证模式分段控件：
  - 手动登录态
  - 账号密码登录
- password 模式下提示：
  - 仅适用于未开启 Turnstile/2FA/Cloudflare challenge 的上游。
  - 密码仅用于当次登录换 token，默认不长期保存。
  - 登录失败时请切换手动登录态。

### Tests Required

- Service:
  - password_login 成功换取 access token 后可同步倍率。
  - 上游返回 `requires_2fa=true` 时返回明确错误，不标记 active。
  - 上游 401/403/HTML challenge 时错误脱敏且不泄露账号密码。
  - password_login 不提供密码且没有既有 token 时返回 400。
- Repository:
  - 新增 token/email 字段创建、更新、脱敏读取。
  - credentialsUpdated 时递增 `credential_version`。
- Handler:
  - password_login 创建连接器使用操作者 ID，响应不包含密码/token。
- Frontend:
  - auth mode 切换时只提交对应字段。
  - `pnpm typecheck` 通过。

### Rollout / Rollback

- Rollout：新增字段向后兼容，现有 `manual_session` 连接器不受影响。
- Rollback：若 password_login 不稳定，可在前端隐藏该模式；后端仍保留 manual_session 可用。
- 安全边界：不保存密码，避免回滚或数据库泄露时扩大敏感面。

## Implementation Granularity

### 1. Upstream Site Connector

- 职责：读取上游分组倍率事实，不负责发模型探测请求。
- 核心字段：`base_url`、认证方式、加密后的登录态凭证、连接器状态、最近验证时间、最近同步时间、最近错误。
- 第一版只要求 `manual_session` 稳定可用；`password_login` 作为后续增强。
- 状态建议：`active`、`needs_reauth`、`invalid`、`paused`。

### 2. Upstream Group Rate Snapshot

- 职责：保存上游登录态同步出来的分组倍率快照。
- 同步来源：`/api/v1/groups/available` 与 `/api/v1/groups/rates`。
- 核心字段：连接器 ID、上游 `group_id`、分组名称、平台、最终倍率、默认倍率、专属覆盖倍率、分组状态、`last_seen_at`。
- 合并规则：`rates[group_id]` 存在时优先使用；否则使用 `available.rate_multiplier`。

### 3. Candidate Channel Mapping

- 职责：把本地候选账号/API Key 与上游分组倍率事实连接起来。
- 核心字段：本地 `account_id`、连接器 ID、上游 `group_id`、探测模型、探测协议、本地 priority 写入目标、候选启停状态。
- 约束：一个候选必须显式选择一个上游 `group_id`；未映射候选不得进入建议或自动托管。

### 4. Candidate Health Probe

- 职责：检测候选通道是否可用，不负责读取倍率。
- 探测必须强制使用候选绑定的本地账号/API Key，不走普通调度池随机选择。
- 记录字段：成功/失败、延迟、HTTP 状态、上游错误码、错误分类、连续失败次数、连续成功次数、最近探测时间。
- 第一版允许只做手动/定时轻量探测；后续再细化错误归因与窗口统计。

### 5. Recommendation Engine

- 职责：基于倍率与健康状态生成本地调度建议，默认不直接修改配置。
- 输入：上游最终倍率、候选健康状态、延迟、错误类型、当前本地 priority、`accounts.status`、`accounts.schedulable`。
- 输出：priority 调整建议、临时禁用建议、恢复建议、建议原因、影响范围、是否可一键应用。
- 第一版必须支持一键应用 priority 建议，并记录原值、新值、操作者、原因和时间。

### 6. Managed Autopilot

- 职责：按管理员策略自动执行建议。
- 第一版不默认开启，只预留策略模型和审计边界。
- 策略维度：错误类型、连续失败阈值、连续成功恢复阈值、冷却时间、最大 priority 调整幅度、是否允许临时禁用。
- 临时禁用落点：本地 `accounts.status` / `accounts.schedulable`；priority 只用于可用候选之间的排序优化。

## Authentication & Secret Handling

- 登录态保存的是浏览器已登录后可访问上游用户侧 API 的凭证，不保存一次性的 Turnstile token。
- `access_token`、`refresh_token`、Cookie、账号密码都必须按敏感凭证处理：数据库加密保存或不落库，接口响应和日志打码，审计只记录凭证版本与状态。
- `password_login` 第一版默认不落库密码；密码仅用于一次性调用上游登录接口换取 token。
- 前端只展示凭证状态、过期时间、最后验证时间和脱敏片段，不回显完整 token/cookie/password。
- 保存或更新连接器时必须立即调用 `/api/v1/groups/available` 做连通性校验；失败则不允许标记为可用。
- 若 `access_token` 过期且存在 `refresh_token`，系统可尝试刷新；若刷新失败、Cookie 过期或 Cloudflare 挑战失效，连接器进入 `needs_reauth` 状态并提示管理员重新粘贴登录态。
- 后台服务不尝试绕过 Cloudflare Turnstile、2FA 或浏览器挑战；这类站点只支持管理员手动完成登录后粘贴登录态。

## Acceptance Criteria (evolving)

- [ ] 管理端能配置上游 sub2api 候选通道及其独立本地账号/API Key 映射，不依赖上游管理员 API。
- [ ] 管理端能配置上游站点连接器，并支持 `manual_session` 保存浏览器登录态读取 `/api/v1/groups/available` 与 `/api/v1/groups/rates`。
- [ ] 管理端能使用 `password_login` 为无 Turnstile/2FA/Cloudflare challenge 的上游换取登录 token，并在成功同步倍率后标记连接器可用。
- [ ] `password_login` 不回显、不记录、不长期保存密码；登录失败时错误信息脱敏并提示切换手动登录态。
- [ ] 保存上游登录态时会立即验证连通性并同步一次分组倍率快照；验证失败时不标记连接器可用。
- [ ] 系统能按 `groups/rates` 覆盖优先、`groups/available.rate_multiplier` 兜底的规则计算最终上游分组倍率。
- [ ] 上游新增分组时，管理员可以通过新增候选通道配置把它纳入监控；未配置的新分组不会被系统误判为已覆盖。
- [ ] 探测运行能强制使用候选绑定的本地账号，不走普通调度池随机选择。
- [ ] 候选通道能显式映射到上游 `group_id`；系统能用登录态倍率快照为候选补全当前倍率。
- [ ] 当登录态不可用但候选 Key `/v1/usage` 可用时，系统可用成本增量推导有效倍率作为兜底，并记录采样前后快照、模型、请求 ID、`cost_delta`、`actual_cost_delta` 和推导倍率。
- [ ] 系统能对上游候选执行黑盒可用性探测，并记录成功率、延迟和错误。
- [ ] 系统能生成本地 priority 调整建议，但未确认前不修改配置。
- [ ] 管理员能一键应用建议，且只更新建议涉及的本地账号分组 priority。
- [ ] 自动托管模式下，系统按策略自动应用低风险调整，并保留完整审计。
- [ ] 自动托管支持按错误类型配置动作；同一候选在不同错误原因下可以触发不同处理。
- [ ] 触发临时禁用/摘出调度时，系统更新候选绑定的本地账号 `status` / `schedulable`，并保留恢复所需的原始状态快照。
- [ ] 被临时禁用的候选仍会被定时探测；连续多次成功后自动恢复原账号状态。
- [ ] 任何自动动作都可暂停、回滚或人工接管。

## Definition of Done

- Tests added/updated for backend service/repository/handler behavior where practical.
- Frontend typecheck/build relevant checks pass or known blockers are documented.
- Migration/schema changes are explicit and do not touch production data outside normal migration files.
- Rollout/rollback considered for automatic priority changes.
- Downstream fork workflow is respected; no commit/push without explicit confirmation.

## Out of Scope (explicit)

- 不在需求澄清阶段直接实现自动托管。
- 不默认对生产调度做静默自动修改。
- 不把非 sub2api 上游站点的网页抓取作为第一优先级。
- 不依赖上游 sub2api 管理员 API。
- 不自动发现上游新增分组。
- 不绕过 Cloudflare Turnstile、2FA、验证码或浏览器挑战。
- 不把一次性 `turnstile_token` 当作长期凭证保存。

## Technical Notes

- 旧成本校准任务仅作为历史背景保留，当前实现入口是上游倍率监控。
- 当前实现重点：持续监控上游 sub2api 分组状态与自动托管。
- 关键约束：无上游管理员 API，因此 MVP 应以“上游登录态连接器 + 分组倍率同步 + 本地候选映射配置 + 健康探测 + 本地调度动作”为核心。
- 登录态倍率事实源：`/api/v1/groups/available` 返回用户可见分组和 `rate_multiplier`；`/api/v1/groups/rates` 返回当前用户专属覆盖倍率，空对象表示没有覆盖。
- Turnstile 场景结论：账号密码后台直连登录不可靠；应由管理员浏览器完成挑战后粘贴 `Authorization`、必要 Cookie 和 User-Agent。
- 已验证测试上游：普通网关 Key 可访问 `/v1/usage`，返回 `cost` 与 `actual_cost`；样本中 `cost / actual_cost = 16.6666666667`，可作为有效倍率读数。
- 已验证测试上游：普通网关 Key 不能访问 `/api/v1/groups/available` 与 `/api/v1/groups/rates`；用户登录态可以访问这两个接口。
- 已验证测试上游：`/api/v1/groups/available` 返回分组 `id/name/platform/rate_multiplier`；`/api/v1/groups/rates` 返回 `{}` 时表示没有用户专属覆盖，最终倍率取 `available.rate_multiplier`。
