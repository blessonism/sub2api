# brainstorm: 用户真实前台活跃时间

## Goal

将用户列表/管理端展示的 `last_active_at` 从“任意登录态请求经过后端”收敛为更接近“用户正在前台真实使用”的口径，避免浏览器后台挂着中转站、自动刷新接口持续把用户刷成活跃。

## What I already know

- 当前用户最后活跃时间存储在 `users.last_active_at`，对外 DTO 字段为 `last_active_at`。
- 登录成功时，`AuthService.touchUserLogin` 会同时更新 `last_login_at` 和 `last_active_at`。
- JWT 鉴权中间件在每次认证通过后调用 `UserService.TouchLastActiveForUser`，因此任何带 JWT 的受保护接口都可能刷新 `last_active_at`。
- `TouchLastActiveForUser` 有 10 分钟最小刷新间隔，失败后有 30 秒退避，更新失败不阻断正常请求。
- 前端登录态会启动用户信息自动刷新，每 60 秒刷新一次当前用户数据；浏览器页面未关闭且定时器仍运行时，即使用户没有查看页面，也可能触发后端刷新活跃时间。
- 任务后续会跨前端、后端、鉴权/接口契约和测试，属于中等复杂度跨层变更。

## Assumptions (temporary)

- “真实活跃”更适合定义为：页面处于前台可见状态，且用户近期存在鼠标、键盘、触摸、滚动等交互。
- 后台轮询、静默刷新、仅 token refresh 不应刷新用户最后活跃时间。
- 登录成功仍可视为一次用户活跃，因为它是明确的人为操作入口。
- API Key 调用不纳入用户网页登录活跃口径；API Key 自身仍使用现有 `last_used_at`。
- 已确认：直接将现有 `last_active_at` 切换为“最近前台真实活跃时间”，不新增并行字段保留旧口径。

## Requirements (evolving)

- 新增或调整一个明确的前台活跃上报入口，由前端在满足“页面可见 + 近期交互”时调用。
- 后端复用现有防抖写入能力，避免新增高频写库风险。
- 静默刷新接口，例如 `/auth/me` 自动刷新，不应继续把后台页面计为活跃。
- 前端心跳应在页面不可见、用户长时间无交互、未登录或退出登录后停止。
- 管理端用户列表继续展示 `last_active_at`，但语义更新为“最近前台活跃时间”。
- 保留失败不影响主流程的行为；活跃上报失败不应打断用户使用。

## Acceptance Criteria (evolving)

- [x] 用户登录成功后，`last_active_at` 会更新一次。
- [x] 浏览器页面保持后台、无用户交互时，自动刷新/轮询不会持续推进 `last_active_at`。
- [x] 页面前台可见且用户近期有交互时，前端按防抖/心跳策略上报活跃，后端刷新 `last_active_at`。
- [x] 后端活跃更新仍保留最小刷新间隔，避免高频写库。
- [x] 退出登录或 token 缺失时，前端不会发送活跃心跳。
- [x] 后端接口、服务、前端心跳逻辑均有匹配测试或可验证覆盖。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 不在本任务中重做在线状态系统或实时在线人数。
- 不把 API Key 使用行为合并进网页登录前台活跃口径。
- 不新增复杂设备维度、会话维度或在线用户 WebSocket。
- 不修改用量日志 `usage_logs` 的活跃用户统计口径，除非实现中发现强依赖冲突。

## Implementation Options

### Option A: 跳过静默刷新 touch

- 后端为 `/auth/me` 等静默接口加跳过活跃更新标记，其他 JWT 请求继续自动更新。
- 优点：改动小，见效快。
- 缺点：仍无法区分“后台页面触发其它受保护轮询”和“真实前台操作”，语义不够干净。

### Option B: 专用前台活跃心跳（推荐）

- 后端新增 `POST /api/v1/users/activity` 或相近专用接口，调用现有 `TouchLastActiveForUser`。
- JWT 中间件默认不再对所有接口自动 touch，或至少对静默刷新/轮询接口跳过。
- 前端监听 `visibilitychange` 与用户交互事件，仅在页面可见且近期有交互时按间隔上报。
- 优点：语义清晰，可维护，能解决“后台挂页面”核心问题。
- 缺点：涉及前后端和测试，改动比 Option A 略大。

### Option C: 保留自动 touch，同时新增前台活跃字段

- 新增字段保存真实前台活跃，保留 `last_active_at` 旧口径。
- 优点：兼容性最强，历史语义不破坏。
- 缺点：需要迁移/DTO/UI 文案调整，字段语义变多，当前问题可能被复杂化。

## Recommended MVP

采用 Option B：专用前台活跃心跳。实现时尽量复用现有 `TouchLastActiveForUser` 防抖逻辑，把变更控制在：

- 后端：新增用户活跃上报 handler/route，或为现有用户模块增加明确方法。
- 后端：调整 JWT 自动 touch 策略，避免静默请求刷新 `last_active_at`。
- 前端：新增登录态前台活跃心跳 composable/store 逻辑。
- 测试：覆盖后台无交互不刷新、前台交互会刷新、心跳停止条件。

## Open Questions

- 暂无阻塞问题；已确认直接切换 `last_active_at` 语义。

## Technical Notes

- 相关文件：
  - `backend/internal/service/auth_service.go`：登录成功更新时间。
  - `backend/internal/server/middleware/jwt_auth.go`：当前 JWT 鉴权后自动 touch。
  - `backend/internal/service/user_service.go`：`TouchLastActiveForUser` 和 10 分钟防抖逻辑。
  - `backend/internal/repository/user_profile_identity_repo.go`：`UpdateUserLastActiveAt` 写库。
  - `backend/internal/handler/dto/types.go`：用户 DTO 暴露 `last_active_at`。
  - `frontend/src/stores/auth.ts`：当前用户数据 60 秒自动刷新。
  - `frontend/src/stores/subscriptions.ts`：订阅状态 5 分钟轮询。
- 已读约束：
  - `.trellis/spec/guides/downstream-fork-workflow.md`
  - `.trellis/spec/backend/quality-guidelines.md`
  - `.trellis/spec/frontend/type-safety.md`
  - `.trellis/spec/guides/cross-layer-thinking-guide.md`
  - `.trellis/spec/guides/code-reuse-thinking-guide.md`
- 下游二开任务 base branch：`custom/main`。
- 任务工作分支：`feature/user-foreground-activity-time`。

## Implementation Summary

- JWT 鉴权中间件不再对普通受保护请求自动刷新 `last_active_at`。
- 新增 `POST /api/v1/user/activity` 专用前台活跃上报接口，后端复用 `UserService.TouchLastActive`。
- 前端登录态启动前台活跃心跳；仅在页面可见、已登录且 2 分钟内存在真实交互时，每 5 分钟最多上报一次。
- 隐藏页面中的交互事件不会污染近期交互状态；退出登录或清除认证状态会停止心跳与监听器。
- 已补充后端 handler、JWT 中间件、API contract、前端 API 和 auth store 心跳测试。
