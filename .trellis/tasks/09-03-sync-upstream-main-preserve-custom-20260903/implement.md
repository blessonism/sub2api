# 执行计划

1. [x] 完成远程和工作区预检，确认 `custom/main`、`origin/main`、`upstream/main` 的当前关系。
2. [x] 分别 fetch 两个远程（`--no-tags --prune`），固定本次同步使用的 `upstream/main` 对象并运行 preflight。
3. [x] 从 `custom/main` 创建 `sync/upstream-20260903`，合并预检所用的同一上游 ref；逐项处理冲突并复核二开能力。（8 个冲突文件已解决并暂存，保留本地二开字段与行为。）
4. [x] 运行 `git diff --check` 与 `tools/upstream_sync.py check`，保存检查计划和结果。（检查计划见 `/tmp/sub2api-upstream-sync-plan.json`；当前未提交合并无法命中定向 lane，报告已记录警告。）
5. [x] 复核验收条件，记录回滚点；不执行 push 或合入 `custom/main`。（merge commit 仍待明确确认。）

## 验证命令

```bash
git status --short --branch
git rev-list --left-right --count origin/main...upstream/main
python3 tools/upstream_sync.py preflight --base custom/main --upstream upstream/main
git diff --check
python3 tools/upstream_sync.py check --base custom/main --head HEAD --dry-run --format json --output /tmp/sub2api-upstream-sync-plan.json
python3 tools/upstream_sync.py check --base custom/main --head HEAD --jobs 3
```

## 回滚点

- 分支创建后、merge 前：删除本地同步分支前先确认无其他工作使用。
- merge 冲突未提交：`git merge --abort`。
- merge 已提交：仅在同步分支 `git revert -m 1 <merge-commit>`。
