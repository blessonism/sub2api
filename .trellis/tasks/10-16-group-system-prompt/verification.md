# 验证记录

## 交付状态

- 开发分支：`feature/group-system-prompt`，基于 `custom/main`；实现与审查修复完成。交付操作为本地提交后快进合并至 `custom/main`，不包含推送或部署。
- 管理员分组创建/编辑（含 simple mode）支持独立开关、前置/追加/替换、正文编辑与空白校验。
- 数据库迁移、Ent、CRUD/复制、仅管理员 DTO、认证查询投影、快照版本及 durable 失效均接通。
- HTTP、双向协议桥、透传、OAuth、两条 WS 路径、token 计数接通。保留原始请求进行每次尝试，分组正文最终加入系统指令；独立图片接口排除。

## 已通过

- `GOPROXY=https://goproxy.cn,direct go test -tags=unit ./... -run '^$'`：后端全仓编译通过。
- 最终测试：`GOPROXY=https://goproxy.cn,direct GOMAXPROCS=4 go test -tags=unit -p 2 ./internal/service ./internal/handler/admin ./internal/handler/dto ./internal/repository ./internal/handler -run 'GroupSystemPrompt|SystemPromptInjection|Test.*SimpleMode' -count=1`：5 个包通过，Go JSON 输出统计 360 个通过项、0 失败（包含父测试与子测试）。
- 较广回归中的既有 Claude passthrough、协议桥、请求构建、OAuth prompt、token 计数及 WS v2 测试通过；新增工具 ID 测试已改为验证调用/结果关联，最终测试通过。
- 前端：新增 API/公共组件/GroupsView 测试，以及既有 duplicate、Codex manifest、i18n collision/ops/completeness，8 文件 66 测试通过；审查后相关 6 文件 56 测试复验通过。
- `pnpm typecheck`、9 个变更 Vue/TS 文件 ESLint、`gofmt`、`git diff --check`、Trellis context validate 通过。
- Ent 已执行两次生成，第二次无额外变更；`backend/go.mod` / `go.sum` 未改动。

## 验证限制

- 真实 repository 集成：已执行 `go test -tags=integration ./internal/repository -run GroupSystemPrompt -count=1 -v`，但 harness 因 `/Users/suki/.orbstack/run/docker.sock` 不存在而提前退出；没有运行真实 PostgreSQL/Redis 用例，不能记为通过。相应用例已写入 `group_system_prompt_integration_test.go`。
- 使用 CI 固定版本 `golangci-lint v2.14.0` 检查相关后端包，当前改动无报告；完整结果仍返回 1，唯一报告为既有 `internal/service/gpt_intelligence_service.go:341` 中未使用的 `gptIntelligenceComparisonKey`。该函数在 HEAD 已存在，本任务未修改该文件。

## 使用约定

- 既有分组迁移后默认关闭；关闭保留正文，省略配置的更新不修改原值。
- 分组自定义与现有全局提示词独立；替换清客户端指令，保留协议身份及服务端全局内容。
- 保存后新 HTTP 请求和新 WS 连接生效；已有连接保留原快照。替换不重写上游已保存历史。
- 本次没有生产数据库操作；后续部署须执行新增迁移 `245_group_system_prompt_injection.sql`。
