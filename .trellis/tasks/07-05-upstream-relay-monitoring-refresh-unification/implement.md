# 执行计划

1. 完善候选校验、配置完整度筛选、usage 结构化部分成功契约和修复后自动刷新闭环。
2. 提取并接入统一操作结果面板，覆盖全局刷新、批量同步、批量探测和调度优先级应用。
3. 完成健康详情、Runner 定格失败、局部反馈清理、术语及 zh/en i18n 一致性。
4. 增补 Go/Vitest 断言，运行聚焦测试、前端 typecheck 与 i18n 检查。

## 验证记录

- `go test ./internal/service -run 'TestUpstreamRelay' -count=1 -timeout=60s`：通过。
- 前端 7 个聚焦 Vitest 文件：96 项测试通过。
- `pnpm typecheck`、任务相关文件 ESLint、Go `gofmt`、`git diff --check`：通过。
- 全量 `go test ./internal/service -count=1 -timeout=60s` 仍有两项与本任务无关的既有失败：`TestOpenAIGatewayServiceRecordUsage_UsesUserSpecificGroupRate` 与 `TestOpenAIGatewayServiceRecordUsage_FallsBackToGroupDefaultRateOnResolverError`；均为计费倍率调用次数断言，不涉及上游监控改动。

## 回滚点

- 后端契约保持 `issue` 兼容字段，前端新面板可独立回退而不影响刷新 API。
- 不改迁移和持久化模型；问题可按后端、共用组件、页面接入三个提交边界回滚。
