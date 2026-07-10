# 现有同步与检查工具链调研

## Query

确认仓库现有 Makefile、GitHub Actions、历史同步任务和可复用测试命令，确定上游同步预检与快速门禁的集成位置。

## Findings

- 根目录 `Makefile` 已统一封装前后端检查：`test-backend`、`test-frontend`、`test-frontend-critical`。完整前端检查会串行执行 lint、typecheck 和关键 Vitest，不适合作为每次冲突修正后的内循环。
- `backend/Makefile` 提供 `test-unit` 与 `test-integration`，现有 `.github/workflows/backend-ci.yml` 已把后端、前端、golangci-lint 拆成三个并行 job，并通过 setup-go/setup-node 复用依赖缓存。
- backend-ci 对所有 push / pull_request 触发，已经承担完整门禁。新的 sync workflow 应提供更快的下游定向反馈，不应再次执行相同的全量 unit/integration/lint。
- `frontend/package.json` 已有 `typecheck`、`lint:check`、`test:run`，可直接以 argv 形式写入检查矩阵，无需新增 npm 依赖或包装脚本。
- Dashboard 真正数据口径由 `backend/internal/repository/usage_log_repo_integration_test.go` 覆盖；handler/view mock 测试不能替代 repository integration。
- 用户用量延迟列应同时选择 `frontend/src/views/user/__tests__/UsageView.spec.ts` 与 `frontend/src/utils/__tests__/latencyHealth.spec.ts`。
- i18n 模块化已有 `localesNoKeyCollision.spec.ts` 与 `opsLocaleKeys.spec.ts`，适合作为 locale split/overlay 的定向门禁。
- 历史同步任务已使用 `git merge-tree` 和文件交集做人工作前评估，但没有形成通用报告、能力映射和自动选测，因此合并后的语义错配仍需人工发现。

## Recommended Integration

- CLI 和 JSON 规则放在 `tools/`，与现有 Python 审计工具同属开发工具，不进入生产二进制。
- 根 Makefile 只增加薄入口，具体检查命令保持在 JSON 中作为单一来源。
- 新增 `.github/workflows/downstream-sync.yml`，仅针对 `sync/upstream-*` 或手动触发；plan job 生成报告，frontend/backend/integration lane 复用同一 CLI。
- 使用 `concurrency.cancel-in-progress` 取消同一同步分支的旧运行，避免冲突修正后多个完整 workflow 并行浪费时间。
- CLI 使用 Python 标准库、NUL-safe Git diff 和 `shell=False`，避免引入 PyYAML 或依赖 shell 文本解析。

## Baseline Caveats

- 当前完整 repository unit 套件存在合并后 SQL mock 漂移噪声；新快速门禁必须使用已知可运行的定向命令，不能把已有全包失败误判为本次同步回归。
- integration tests 依赖 CI 或本地 PostgreSQL 测试环境；环境缺失必须报告未验证，不能降级为只跑 mock view/handler。
- 现有 backend-ci 是否持续全绿应单独治理；本任务不修改业务 SQL mock，也不放宽已有 CI。

## Related Specs

- `.trellis/spec/guides/downstream-fork-workflow.md`
- `.trellis/spec/guides/code-reuse-thinking-guide.md`
