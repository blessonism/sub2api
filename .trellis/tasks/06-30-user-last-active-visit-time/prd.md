# brainstorm: 用户最后活跃时间贴近网站访问

## Goal

让管理员用户管理中的“最后活跃时间”尽可能贴近用户最后访问、查看网站的时间，而不是只在登录、调用 API Key 或其他后台行为时才更新。

## What I already know

- 用户希望“最后活跃时间”更接近用户实际浏览网站的时间。
- 后端已存在 `last_active_at` 字段，以及带节流保护的用户活跃时间触达逻辑。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开仓库，任务基线为 `custom/main`。

## Assumptions (temporary)

- 该需求优先面向浏览器登录用户，不把 API Key 转发请求视为“查看网站”。
- 不新增并行字段，优先复用现有 `last_active_at`。
- 活跃时间写入需要节流，避免用户停留页面时产生过多数据库写入。

## Open Questions

- 暂无阻塞问题；先按“登录态用户打开或回到网站时触达一次，并由后端节流”推进。

## Requirements (evolving)

- 管理员用户列表/详情继续展示 `last_active_at`。
- 登录用户访问前端页面、回到可见页面或发生真实交互时，应尽快触发一次活跃时间更新。
- 页面长期处于后台或无交互闲置时，不应仅靠定时器持续刷新活跃时间。
- 活跃时间更新失败不应影响用户正常访问网站。
- 后端必须保留写入节流与并发收敛，避免高频页面访问造成写放大。

## Acceptance Criteria (evolving)

- [x] 登录态用户打开网站或从后台回到前台后，后端会尝试更新该用户 `last_active_at`。
- [x] 用户频繁切换页面或刷新时，不会绕过后端节流策略。
- [x] 页面后台挂起或纯闲置时，不会持续推后 `last_active_at`。
- [x] 未登录用户不会触发用户活跃时间写入。
- [x] 相关后端/前端类型检查或定向测试通过，已知无关 handler 包编译问题已记录。

## Definition of Done (team quality bar)

- Tests added/updated where appropriate.
- Lint / typecheck / targeted checks pass or failures are explained.
- Trellis task records the downstream fork context.
- Rollout/rollback risk considered.

## Out of Scope (explicit)

- 不调整管理员用户管理的整体信息架构。
- 不引入实时在线状态、在线人数统计或 WebSocket 心跳。
- 不把 API Key 使用行为改定义为网页访问活跃。

## Technical Notes

- Required context: `.trellis/spec/guides/downstream-fork-workflow.md`
- Relevant existing code discovered:
  - `backend/internal/service/user_service.go`: `TouchLastActive` / `TouchLastActiveForUser`
  - `backend/internal/repository/user_profile_identity_repo.go`: `UpdateUserLastActiveAt`
- Implementation decision:
  - 前端在登录态页面开始监听时立即上报一次，表示用户进入网站查看。
  - 前端在 `visibilitychange` 回到 `visible` 时上报一次，表示用户从后台回到网站查看。
  - 前端在真实交互事件时尝试上报，但不再使用后台定时器自动续写，避免长时间闲置被误判为持续活跃。
  - 后端继续通过 `TouchLastActive` 的最小间隔防抖和 singleflight 收敛写入。
- Verification:
  - `pnpm exec vitest run src/stores/__tests__/auth.spec.ts src/api/__tests__/auth.activity.spec.ts`
  - `pnpm exec vue-tsc --noEmit`
  - `GOCACHE="/Users/suki/code/sub2api/backend/.cache/go-build" go test -tags=unit ./internal/server/middleware -run 'TestJWTAuth_ValidToken_DoesNotTouchLastActive|TestJWTAuth_ValidToken'`
  - `GOCACHE="/Users/suki/code/sub2api/backend/.cache/go-build" go test -tags=unit ./internal/service -run 'TestTouchLastActive'`
  - `GOCACHE="/Users/suki/code/sub2api/backend/.cache/go-build" go test -tags=unit ./internal/handler -run 'TestUserHandlerReportActivity'` 当前受 `internal/handler/openai_images_failover_test.go` 中 `NewOpenAIGatewayHandler` 旧签名调用影响，包编译未通过，和本任务改动无关。
