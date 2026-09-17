# 执行计划

1. [x] 完成当前 Git 状态、远程配置和工作树保护预检。
2. [x] 分别 fetch `origin` 与 `upstream`，固定 `upstream/main` OID 并运行 preflight。
3. [x] 从 `custom/main` 创建 `sync/upstream-20260907`，以同一上游 ref 完成未提交合并并复核全部冲突。
4. [x] 运行 `git diff --check`，执行后端定向检查；前端检查因同步 worktree 缺少 `node_modules` 未验证。
5. [ ] 输出验收与回滚信息；不 push、不合回 `custom/main`，merge commit 等待明确确认。

## 验证命令

```bash
git status --short --branch
git remote -v
git rev-list --left-right --count origin/main...upstream/main
python3 tools/upstream_sync.py preflight --base custom/main --upstream upstream/main
git diff --check
python3 tools/upstream_sync.py check --base custom/main --head HEAD --dry-run --format json --output /tmp/sub2api-upstream-sync-plan-20260907.json
python3 tools/upstream_sync.py check --base custom/main --head HEAD --jobs 3
```

## 风险点与回滚

- 工作树路径碰撞：先停止切换或合并，保护现有内容后再继续。
- 未提交冲突：`git merge --abort`。
- 已提交同步：仅在同步分支 `git revert -m 1 <merge-commit>`。

## 本次记录

- `upstream/main` 固定为 `ab99d56e9626e6cd731592dae8553c9758a0efa2`，`custom/main` 保持 `0518e0a81eef502436f78e97f6c0b22ca25e7bb8`。
- 同步分支位于 `/private/tmp/sub2api-sync-upstream-20260907`，合并仍未提交，`MERGE_HEAD` 保留同一上游对象。
- 18 个文本冲突已逐文件复核并暂存，索引未解决项为 0；Ent 生成重复运行无差异。
- 修复了两个合并后回归：usage log 两处静态 INSERT 补齐 `$63`，pinned Codex handler helper 补齐配额仓储参数。
- 后端 handler 编译、repository integration（含 dashboard）、usage-log shape、Codex pinned tests 均通过。
- 前端定向测试和 `pnpm typecheck` 因同步 worktree 没有 `frontend/node_modules` 未验证；主工作树依赖未触碰。
- 初始检查报告为 `/tmp/sub2api-upstream-sync-check-20260907.json`；其中两个代码失败项已由上述回归命令复核通过。
