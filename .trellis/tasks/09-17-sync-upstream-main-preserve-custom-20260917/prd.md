# 同步最新官方 main 并保留二开

## Goal

将 `upstream/main` 的最新官方更新同步到下游二开代码，生成可审查、可回滚的本地同步分支，同时不覆盖 `custom/main` 和当前工作树的既有改动。

## Background

- `upstream` 是官方仓库 `Wei-Shaw/sub2api`，`origin` 是下游 fork；`upstream` push 地址已禁用。
- `custom/main` 是二开长期主线，`main` 仅作为官方镜像。
- 当前工作树位于 `custom/main`（`adc5237f2`），存在未跟踪的 Trellis/工具目录，不能通过切换或清理工作树来规避。
- fetch 后固定引用：`upstream/main` = `881f3202694c6bc932446931a30c27d9675178b9`（官方 0.2.5）；merge-base = `98d86915becae9fe9491a91ffc6defd5235c8d2b`（官方 0.2.4）。
- 预检：下游 283 提交 / 1523 路径，上游 197 提交 / 447 路径，规范化路径交集 105；`uncovered_risk_paths` 为空。

## Requirements

1. 使用 `git fetch --no-tags --prune origin` 和 `git fetch --no-tags --prune upstream` 更新远程分支引用，不改写本地 tag。
2. 以 fetch 后解析出的同一 `upstream/main` 引用运行 `tools/upstream_sync.py preflight`，并记录预检结果。
3. 从 `custom/main` 创建 `sync/upstream-20260917`，在独立 worktree 中以同一上游引用合并。
4. 保留 `custom/main` 指针、当前工作树修改及未跟踪内容；冲突必须逐项复核，优先保留二开能力，不使用 reset、checkout 覆盖或删除用户内容。
5. 生成并执行同步工具命中的定向检查；因环境不可用的检查必须明确记录为未验证。
6. 本次不执行 `git push`，不把同步分支合入 `custom/main`，不修改生产环境。

## Acceptance Criteria

- [x] `upstream/main` 引用更新成功，预检与合并使用同一上游对象 `881f3202694c`。
- [x] 同步分支基于最新 `custom/main`，且 `custom/main` 指针保持不变。
- [x] 合并结果无未解决冲突，`git diff --check` 通过。
- [x] 定向检查计划已生成并执行；integration 因 testcontainers 下载超时记为未验证。
- [x] 当前已跟踪修改、未跟踪目录和任务记录在操作前后均保留。
- [x] 回滚路径明确：未提交合并使用 `git merge --abort`，已提交同步仅在同步分支使用 `git revert -m 1`。

## Out of Scope

- 推送同步分支或创建/合并 PR。
- 将同步分支合入 `custom/main` 或更新生产部署。
- 快进本地 `main` 镜像分支（可另开操作）。
