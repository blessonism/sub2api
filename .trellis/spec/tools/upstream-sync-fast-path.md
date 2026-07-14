# 上游同步快车道可执行契约

## 1. Scope / Trigger

修改 `tools/upstream_sync.py`、`tools/upstream_sync_checks.json`、同步分支 workflow 或下游能力回归矩阵时适用。目标是在真实 merge 前只读识别风险，在 merge 后只运行命中的快速检查，并在快速门禁通过后执行一次完整 CI。

## 2. Signatures

```text
python3 tools/upstream_sync.py preflight [--base REF] [--upstream REF] [--format text|json] [--output PATH]
python3 tools/upstream_sync.py check [--base REF --head REF | --report PATH] [--lane LANE] [--jobs N] [--dry-run] [--format text|json] [--output PATH]
```

- `preflight` 默认比较 `custom/main` 与 `upstream/main`，只允许只读 Git 子命令。
- `check` 默认比较 `custom/main..HEAD`；`--report` 复用已生成的 `selected_checks` 和 `selection_reasons`。
- 退出码：成功 `0`，检查失败或超时 `1`，存在未覆盖高风险路径 `2`，参数/ref/配置错误 `3`，用户中断 `130`。

## 3. Contracts

- 检查矩阵顶层必须包含 `schema_version`、`capabilities` 和 `checks`。
- capability 必须声明稳定 `id`、`paths`、`tracked_commits`、`checks`；结构迁移可声明 `structural_successors`。
- check 必须声明 `id`、`lane`、`paths`、argv 数组、仓库内 `cwd` 和 `1..60` 秒超时；通用检查显式设置 `fallback=true`。
- JSON 报告稳定输出 refs、双方 changes、risks、冲突候选数、selected checks/lanes、selection reasons、实际 argv、warnings 和阶段耗时。
- CLI 不读取网络或环境变量，不执行 fetch/merge/checkout/reset/commit/push；仅在显式 `--output` 时写报告。

## 4. Validation & Error Matrix

- ref 不存在、无 merge-base、Git 读取失败或 schema 无效 -> 退出 `3`。
- 高风险路径没有专属检查且没有同 lane fallback -> 报告 `uncovered_risk_paths`，退出 `2`。
- 计划引用未知 check、lane 不存在、`jobs < 1` -> 退出 `3`。
- 任一检查非零或超过自身超时 -> 报告失败/超时，整体退出 `1`。
- 没有命中检查 -> 退出 `0`，但必须警告仍依赖完整 CI。

## 5. Good / Base / Bad Cases

- Good：仓储源文件大幅删除并拆成同 stem 文件，报告结构迁移、命中对应 capability、选择 repository integration。
- Base：普通前端或后端交叉路径没有专属能力，选择对应 fallback 并记录具体路径原因。
- Bad：把普通路径交集称为确定文本冲突，或因没有专属测试而静默跳过检查。

## 6. Tests Required

- NUL 分隔的 add/delete/rename/copy 与 numstat 解析，包括换行路径。
- 只读 Git allowlist、非法配置、未知 check/lane、超时和失败码传播。
- fallback 选择、选测原因、实际命令、并行关键路径和空选择警告。
- 历史 `2c503a333` 必须识别 `usage_log_repo.go` 拆分并选择 Dashboard integration 与 UsageView 回归。
- workflow 必须保证 frontend/backend/integration 快速 lane 并行，成功后只调用一次 reusable 完整 CI。

## 7. Wrong vs Correct

错误：`pnpm test:run -- path/to/test.spec.ts`。该写法会把多余的 `--` 传给 Vitest，文件过滤可能失效并运行完整套件。

正确：矩阵 argv 使用 `pnpm test:run path/to/test.spec.ts`，并由单元测试禁止 `pnpm test:run` argv 中出现 `--`。
