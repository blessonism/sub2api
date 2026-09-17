# 执行计划

1. [x] 完成当前分支、工作树、远程配置和现有同步任务预检。
2. [x] 分别 fetch `origin` 与 `upstream`，固定 `upstream/main` OID。
3. [x] 运行 preflight，确认双方 merge-base、重叠路径和高风险能力。
4. [ ] 从 `custom/main` 创建 `sync/upstream-20260910` 独立 worktree，并合入同一上游 ref；逐项解决和复核冲突。当前进入剩余后端与前端契约复核。
5. [ ] 生成同步检查计划，执行命中的定向检查，记录未验证项。
6. [ ] 验收同步分支和工作树保护结果；不 push、不合入 `custom/main`。

## 验证命令

```bash
git status --short --branch
git remote -v
git rev-list --left-right --count origin/main...upstream/main
python3 tools/upstream_sync.py preflight --base custom/main --upstream upstream/main
git diff --check
python3 tools/upstream_sync.py check --base custom/main --head HEAD --dry-run --format json --output /tmp/sub2api-upstream-sync-plan-20260910.json
python3 tools/upstream_sync.py check --base custom/main --head HEAD --jobs 3
```

## 回滚点

- 创建 worktree 或分支失败：删除新建的未使用 worktree/分支，不触碰 `custom/main`。
- 合并未提交：`git merge --abort`。
- 合并已提交：仅在 `sync/upstream-20260910` 使用 `git revert -m 1 <merge-commit>`。
