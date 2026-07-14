# 上游同步快车道技术设计

## Components

### `tools/upstream_sync.py`

提供两个子命令：

- `preflight`: 读取 Git refs 和检查矩阵，生成风险与检查选择报告。
- `check`: 根据两个 refs 或已有报告选择检查，按 lane/并发数执行，支持 `--dry-run`。

所有 Git 调用使用参数数组和 `subprocess.run(..., shell=False)`；预检只允许只读命令。命令从仓库根目录执行，报告路径由调用方显式提供。

### `tools/upstream_sync_checks.json`

JSON 顶层包含 schema version、capabilities 和 checks。capability 记录稳定 ID、关注提交、路径、结构 successor 与 check IDs；check 包含稳定 ID、说明、lane、路径 glob、argv、工作目录和超时。CLI 是唯一解释器，Makefile、文档和 CI 不复制具体测试命令。

### 风险模型

数据流：

```text
merge-base
  -> downstream diff (merge-base..base)
  -> upstream diff (merge-base..upstream)
  -> path/status/numstat normalization
  -> direct overlap + structural signals
  -> affected downstream commits
  -> check matrix selection
  -> text/JSON report
```

风险信号：

- 双方直接修改同一路径。
- 下游触碰路径被上游删除或重命名。
- 上游对重叠文件发生大比例删除并在同目录新增同前缀文件，提示拆分/迁移。
- 已登记提交按其是否为下游/上游 ref 的祖先分类为 downstream-only、incoming-upstream、common 或 missing；其路径与对侧相交时提升对应 capability 风险。
- 变更命中二开关键路径但没有专属检查时，选择该 lane 的 fallback 检查并在报告中警告。

风险是信息，不阻止预检退出成功；非法 ref、Git 命令失败或配置无效返回非零。检查失败则 `check` 返回非零。

## Parallel Execution

检查按 lane 过滤并以线程池并发启动独立进程。每个 check 记录开始、结束、耗时、退出码和截断后的输出；收到中断时终止未完成子进程。并发默认 3，可通过参数调整。

理论关键路径从串行的 `sum(Ti)` 降为并行批次的 `max(Ti)` 加固定预检开销；人工冲突处理仍独立计时。

## CI Design

`.github/workflows/downstream-sync.yml` 仅匹配 `sync/upstream-*` push/PR 或手动触发：

- plan job: fetch-depth 0，生成并上传预检/选择报告。
- frontend/backend/integration jobs: 安装各自依赖并调用同一 CLI 的对应 lane。
- 使用 setup-go/setup-node 原生缓存。
- 快速 lane 全部成功后，通过 `workflow_call` 调用现有 backend-ci 一次；同步分支的普通 backend-ci 触发跳过执行，确保完整 unit/integration/lint 不在快速门禁前重复启动。
- 专用 workflow 只引用 reusable workflow，不复制完整门禁命令；plan 和各 lane 把选测原因与耗时写入 job summary。

## Compatibility And Safety

- Python 3 标准库，无 PyYAML 等额外依赖。
- JSON 报告包含 schema version，未来可兼容扩展。
- 不写 `.git`，rerere 配置由已确认的一次性 Git config 完成。
- 所有默认 ref 都可覆盖，历史 dry-run 不依赖当前 checkout。

## Rollback

删除 CLI、规则、workflow 和 Makefile 入口并回退工作流文档即可；rerere 可通过 `git config --local --unset rerere.enabled` 和 `--unset rerere.autoupdate` 单独撤销，不影响提交历史或工作树。
