# 实施计划

1. 调研现有 CI、Makefile、历史同步提交和可复用测试，固化 research 文件。
2. 实现 Git diff 解析、风险识别、检查矩阵加载与 text/JSON 报告，并先写 CLI 单元测试。
3. 实现并行检查器、lane 过滤、dry-run、超时和失败传播。
4. 增加 Makefile 入口与 `sync/upstream-*` 专用 GitHub Actions workflow。
5. 更新 downstream fork 工作流，记录 rerere、预检、定向检查、完整 CI 和回滚步骤。
6. 对历史 `2c503a333` 双亲执行 dry-run，确认风险与检查选择；运行脚本测试、配置解析和范围内质量检查。

## Validation Commands

```bash
python3 -m unittest discover -s tools/tests -p 'test_upstream_sync.py'
python3 tools/upstream_sync.py preflight --base 2c503a333^1 --upstream 2c503a333^2 --format text
python3 tools/upstream_sync.py check --base custom/main --head HEAD --dry-run
git diff --check
```

## Review Gates

- CLI 不得包含任何写 Git 的子命令。
- 检查命令必须只存在于 JSON 矩阵中。
- 历史 dry-run 必须选择 Dashboard integration 和 UsageView regression。
- workflow 不重复现有完整 CI。

## Rollback Points

- CLI/测试完成后可独立回退。
- CI 与文档仅在本地历史验证通过后加入。

## Verification Results

- CLI 单元测试 25 项通过；Python 编译、JSON 解析、workflow YAML 解析与 `git diff --check` 通过。
- 历史 `2c503a333^1...2c503a333^2` 预检约 0.7 秒，识别 72 个冲突候选、`usage_log_repo.go` 拆分和 UsageView 交叉修改，并选中 Dashboard integration 与用户 UsageView 回归。
- 五个后端 fallback 编译检查并行完成，阶段耗时约 5.5 秒，最长单项约 5.5 秒。
- `5a18ff2bb` 上 Dashboard handler、repository integration、DashboardView、UsageView 与 latency health 回归均通过；feature 分支在合入前需基于包含该提交的最新 `custom/main` 更新基线。
- 活动中心和排行榜后端定向检查通过。上游监控后端检查在当前 `custom/main` 基线暴露 `TestUpstreamRelayRepositoryUpsertsRecommendationPolicy` SQL mock 参数数量不匹配；该既有失败应由对应监控任务修复，不得从同步矩阵移除。
