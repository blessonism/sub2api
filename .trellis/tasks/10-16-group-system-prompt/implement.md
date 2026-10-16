# 实施步骤

1. [x] 配置持久化/API/auth 缓存链路和定向测试（真实数据库集成环境未启动，见 verification.md）。
2. [x] 管理端公共配置组件、GroupsView 创建/编辑、类型及 zh/en 文案和测试。
3. [x] 公共请求策略及 OpenAI/Claude HTTP、WS、token 计数接入，模拟上游验证。
4. [x] 整合检查，审查重试幂等、协议转换顺序、缓存/DTO 链路；更新可执行契约。

## 检查
- backend: go test -tags=unit ./internal/service ./internal/handler/... ./internal/repository/... 配合定向 -run；go test -tags=integration ./internal/repository -run GroupSystemPrompt；go test ./... -run '^$' 编译检查。
- frontend: pnpm exec vitest run 本任务组件/View/helper 测试及 i18n 回归；pnpm typecheck；pnpm exec eslint 变更的 Vue/TS 文件。
- git diff --check；清理本任务不再使用的临时产物。

## 边界
分支 feature/group-system-prompt，基于 custom/main。保留原有未提交文件。按用户授权提交并合并到本地 custom/main；不执行 push 或生产部署。
