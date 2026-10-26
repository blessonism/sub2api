# 实现与验证记录

## 交付
- 分支：feature/model-group-routes；基线 custom/main；原有 IQ 探针未提交改动保留。
- 后台逐条配置固定目标/本组优先；共享 HTTP/WS 选组、目标定价/RPM/用户倍率、原 Key 额度、两组白名单和权限。
- 普通/单模型/Codex 列表与目标能力、原上游 Codex 元数据、最终 ETag；WS passthrough/dedicated 多轮价格刷新与换组重连。
- 新增 source_group_id、JSONB 规则、auth snapshot 26、SQL durable invalidation；已提供生产启用 JSON 示例，但未应用。

## 通过的检查
- backend: go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin ./internal/handler/dto ./internal/server/routes ./internal/server/middleware ./internal/repository ./migrations -run 'ModelGroup|Group|RecordUsage|UsageLog|BillingCacheService|APIKey.*Cache|Codex.*Models|Models.*Codex|OpenAIResponsesWebSocket' -count=1。
  - service、handler/admin/dto、routes/middleware、migrations 通过；repository 两个旧 session_id 参数位置断言已随新列修正，单独重跑相关 repository Group/UsageLog 全部通过。
- backend: 最终 ModelGroupRoutes service/handler 测试与 Codex/OrdinaryPinnedModels 回归通过。覆盖目标用户倍率、账号绑定、协议能力、实际 HTTP 流/非流转发、WS 双模式、原提示词冻结、用量与单次扣费/RPM。
- backend: go test -tags=integration ./internal/repository -run TestModelGroupRoutes -count=1 -v，真实 PostgreSQL 18 / Redis 8 容器运行，两项测试通过（无 skip）。
- backend: go test ./cmd/server -run '^$' 编译通过；Ent 第二次生成 hash 不变；git diff --check 通过。
- frontend: pnpm typecheck；pnpm run build（包含 i18n 检查与 vue-tsc）通过。
- frontend: ModelGroupRouteFields、GroupsView.systemPromptInjection、UsageTable、localeKeyCompleteness 共 74 tests 通过；全部本次前端变更文件 eslint 通过。
- golangci-lint v2.14.0：按本次 tracked/untracked 变更 patch 检查，0 issues。
- Trellis implement/check context validation 通过。

## 已有工作区问题
全工作区 --new-from-rev=HEAD 包含用户此前未提交的 IQ 探针改动，报告 5 个 errcheck：
- backend/internal/repository/model_iq_probe_repo.go:153、184，rows.Close 返回值未检查。
- backend/internal/repository/model_iq_probe_repo_test.go:18、34、45，db.Close 返回值未检查。
这些不属于本任务，保持原样；本次新增的 errcheck/staticcheck 项已清零。

## 发布状态
未 commit、push、迁移生产数据库或重启 OVH。规则缺省为空，迁移不硬编码生产分组。生产建议配置见 production-routes.json，上线与启用仍按已确认计划单独确认。

## 第二轮 Review
- 发现并修复 5 类 P2：过滤后默认目录回填、WS 后启用规则漏检、WS Key/User 快照被整体更新、prefer_local 全局暂停/模型冷却漏检、固定来源目录无账号时目标模型不展示。
- 各问题均先写回归复现失败，再修正。新增回归与相关 service/handler/routes 网关测试通过；完整命令、失败证据与修正行为见 review-2.md。
- 本次功能变更 patch 的 golangci-lint v2.14.0：0 issues；git diff --check、Trellis context validation 通过。
- 本轮没有修改数据库结构、repository 或前端，未重复运行这些不受影响的检查。
