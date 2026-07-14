# Implementation

1. 新增迁移、领域类型、仓储与共享缓存解析器，接入依赖注入。
2. 扩展管理员用户详情/更新契约，实现完整替换、校验与缓存失效。
3. 将策略接入 Anthropic、OpenAI/Grok、Gemini/Antigravity 和 OpenAI WebSocket 的候选选择路径。
4. 扩展前端类型、API、用户分组配置弹窗及中英文文案。
5. 添加迁移、仓储、服务调度、接口和组件测试；运行相关 Go 测试、前端测试与类型检查。

## Validation

- `go test ./internal/repository ./internal/service ./internal/handler/admin`
- `pnpm exec vitest run <targeted specs>`
- `pnpm typecheck`

## Rollback

代码回滚后保留新增空闲表不影响旧版本；如必须删除表，另行执行显式回滚 SQL，不在应用启动时自动降级。
