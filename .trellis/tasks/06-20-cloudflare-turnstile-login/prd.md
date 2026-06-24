# brainstorm: 接入 Cloudflare Turnstile 登录验证

## Goal

确认 sub2api 是否需要新增 Cloudflare Turnstile 登录验证代码，并给出可落地配置路径，让登录、注册、发验证码和忘记密码入口具备按钮附近的人机验证，同时不影响 OpenAI 兼容 API 客户端调用。

## What I already know

- 用户已经在 Cloudflare 创建了 Turnstile，准备自行填写 Site Key 和 Secret Key。
- sub2api 不适合对 `/v1/*` 等 OpenAI 兼容 API 路径做 Cloudflare 页面级质询，否则客户端会请求失败。
- 仓库现有代码已经包含 Turnstile 前端组件、后台安全设置、公共配置暴露和后端 `siteverify` 校验。

## Requirements

- 登录页在 Turnstile 启用且存在 Site Key 时展示验证码组件。
- 注册页在 Turnstile 启用且存在 Site Key 时展示验证码组件。
- 后端在登录、注册、发送邮箱验证码、忘记密码入口校验 `turnstile_token`。
- Turnstile 配置由管理员后台维护，不把 Secret Key 暴露给前端。
- 不对 OpenAI 兼容 API 调用链路增加 Turnstile 校验。

## Acceptance Criteria

- [x] 已确认 `frontend/src/views/auth/LoginView.vue` 会渲染 `TurnstileWidget` 并随登录请求发送 `turnstile_token`。
- [x] 已确认 `frontend/src/views/auth/RegisterView.vue` 会渲染 `TurnstileWidget` 并随注册/验证码请求发送 `turnstile_token`。
- [x] 已确认 `backend/internal/handler/auth_handler.go` 在登录、注册、发送验证码、忘记密码前调用 Turnstile 校验。
- [x] 已确认 `backend/internal/service/turnstile_service.go` 与 `backend/internal/repository/turnstile_service.go` 会调用 Cloudflare `siteverify`。
- [x] 已确认后台设置页已有 Cloudflare Turnstile 开关、Site Key 和 Secret Key 输入项。

## Definition of Done

- 无需新增业务代码。
- 给用户提供后台配置步骤和验证方式。
- 记录当前结论，避免重复实现已存在能力。

## Technical Approach

采用现有 Turnstile 能力：管理员在后台安全设置启用 Turnstile 并填写 Cloudflare Site Key / Secret Key；前端公共设置接口只返回 `turnstile_enabled` 和 `turnstile_site_key`，登录/注册等页面展示组件；后端 auth handler 收到 `turnstile_token` 后通过 service/repository 调 Cloudflare `siteverify` 校验。

## Out of Scope

- 不新增 WAF Custom Rules。
- 不对 `/v1/*`、`/api/v1/*` 等模型 API 做人机验证。
- 不提交真实 Site Key 或 Secret Key。

## Technical Notes

- 下游二开约束：已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，本次未执行 git commit/push、未修改生产 Cloudflare 或服务器配置。
- 当前代码已具备 Turnstile 管理后台配置：`frontend/src/views/admin/SettingsView.vue`。
- 当前登录页/注册页已接入 Turnstile 组件：`frontend/src/views/auth/LoginView.vue`、`frontend/src/views/auth/RegisterView.vue`。
- 当前后端校验入口：`backend/internal/handler/auth_handler.go`、`backend/internal/service/turnstile_service.go`、`backend/internal/repository/turnstile_service.go`。
