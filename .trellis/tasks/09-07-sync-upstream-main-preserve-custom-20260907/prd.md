# 同步最新官方 main 并保留二开

## Goal

将官方仓库 `upstream/main` 的最新更新同步到下游二开代码，形成可审查、可回滚的本地同步分支，同时不移动 `custom/main`，不覆盖当前工作树中的本地改动。

## Background

- 当前基线是 `custom/main`，长期承载下游二开。
- `upstream` 指向官方仓库，`origin` 指向下游 fork；`upstream` push 地址已禁用。
- 当前工作树有 1 个已跟踪修改和多个未跟踪目录/任务记录，均不属于本次同步范围，必须保留。
- 仓库已有同步规范：预检和合并必须使用同一份已解析的 `upstream/main` 引用，并在 `sync/upstream-*` 分支完成。

## Requirements

1. 分别执行 `git fetch --no-tags --prune origin` 和 `git fetch --no-tags --prune upstream`，不改写本地 tag。
2. 记录 fetch 后的 `upstream/main` 对象，并以该对象运行 `tools/upstream_sync.py preflight --base custom/main --upstream upstream/main`。
3. 从 fetch 后的 `custom/main` 创建日期命名的 `sync/upstream-20260907`（若已存在则使用不覆盖现有工作的等价名称），在该分支合入同一 `upstream/main`。
4. 在合并过程中保留下游二开行为和当前未提交/未跟踪内容；发生冲突时逐项复核，不使用破坏性重置。
5. 合并结果通过 `git diff --check`，生成并执行同步工具选择的定向检查；无法运行的检查必须记录原因。
6. 本次不执行 `git push`，不直接修改或合入 `custom/main`，不处理无关任务和生产环境。

## Acceptance Criteria

- [ ] 两个远程引用更新成功；预检与合并使用同一 `upstream/main` 对象。
- [ ] 同步工作位于 `sync/upstream-*` 分支，`custom/main` 指针保持不变。
- [ ] 合并无未解决冲突，`git diff --check` 通过。
- [ ] 定向检查计划已生成并执行，未执行项有明确记录。
- [ ] 当前已跟踪修改、未跟踪目录和任务记录在同步前后均保留。
- [ ] 回滚路径明确：未提交合并可用 `git merge --abort`，已提交同步仅在同步分支使用 `git revert -m 1`。

## Out of Scope

- 推送同步分支或创建/合并 PR。
- 将同步分支合入 `custom/main`。
- 修改生产数据库、部署环境或远程服务器。
