# 技术设计

## 分支与引用边界

- `custom/main`：下游二开主线，只读基线，不直接执行上游合并。
- `upstream/main`：官方同步源，fetch 后解析为本地 ref，用于预检和 merge。
- `sync/upstream-20260903`：本次同步工作分支，从当时最新的 `custom/main` 创建。
- `origin`：下游远程，仅更新本地引用，不推送。

## 数据流

1. 检查工作区和远程配置，确认 `.omo/` 等用户内容不纳入同步。
2. 分别 fetch `origin` 与 `upstream`，禁用 tag 跟随并清理失效分支引用。
3. 对 `custom/main` 与 `upstream/main` 执行 `tools/upstream_sync.py preflight`，记录共同祖先、规范化路径交集和高风险能力提示。
4. 从 `custom/main` 创建同步分支，并合并同一 `upstream/main` 对象。
5. 对冲突按“保留下游能力、吸收官方更新”的边界逐文件复核；完成后运行差异检查和同步矩阵中的命中检查。

## 兼容与回滚

- 不修改远程 tag、不改写已有分支历史。
- 合并未提交时使用 `git merge --abort`。
- 合并提交后只在同步分支使用 `git revert -m 1 <merge-commit>`。
- `custom/main` 在本次任务中不移动；只有用户后续明确确认后才考虑 push 或 PR 合入。

## 风险

- 上游与二开同时改动同一路径时可能出现文本或语义冲突，需要检查路由、DTO、仓储生产者及前端类型等跨层契约。
- 定向检查依赖本地环境；不可用时记录未验证，不以替代性 mock 检查冒充通过。
