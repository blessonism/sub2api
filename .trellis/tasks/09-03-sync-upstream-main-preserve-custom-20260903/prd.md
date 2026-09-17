# 同步官方最新 main 并保留二开

## Goal

将官方仓库 `upstream/main` 的最新远程更新同步到本地下游仓库，同时保留 `custom/main` 上已有的二开提交，形成可审查、可回滚的同步分支。

## Background

- 当前工作分支为 `custom/main`，其长期承载下游业务定制。
- `upstream` 指向官方仓库，push 地址已禁用；`origin` 指向下游 fork。
- 工作区存在未跟踪目录 `.omo/`，不属于本次同步范围，必须保留。
- 根据下游 fork 规则，上游同步必须在 `sync/upstream-*` 分支完成，不得直接在 `custom/main` 上合并。

## Requirements

1. 分别以 `git fetch --no-tags --prune origin` 和 `git fetch --no-tags --prune upstream` 更新远程分支引用，不改写本地 tag。
2. 使用同一份已解析的 `upstream/main` 引用完成预检和合并，记录 merge-base、重叠路径及风险提示。
3. 从最新 `custom/main` 创建 `sync/upstream-20260903`（若已存在则使用不会覆盖其他工作的等价新名称），在该分支合并 `upstream/main`。
4. 解决并复核同步冲突，确保下游二开能力不被意外删除或覆盖；保留 `.omo/` 未跟踪内容。
5. 运行同步工具生成的定向检查，至少完成 `git diff --check`；检查失败或环境不可用时如实记录。
6. 不在本次操作中直接修改 `custom/main`，不执行 `git push`，不创建目标为 `custom/main` 的合并提交，除非用户另行确认。

## Acceptance Criteria

- [ ] `origin` 与 `upstream` 引用更新成功，且预检和合并使用同一 `upstream/main` 对象。
- [ ] 同步工作发生在 `sync/upstream-20260903` 分支，`custom/main` 指针保持不变。
- [ ] 合并结果无未解决冲突，`git diff --check` 通过。
- [ ] 上游同步定向检查计划已生成并执行；每个未执行项有明确原因。
- [ ] `.omo/` 未跟踪目录仍存在，未被修改或删除。
- [ ] 回滚路径清晰：未提交冲突可用 `git merge --abort`，已提交同步可在同步分支 `git revert -m 1`。

## Out of Scope

- 不推送远程、不创建或合并 PR。
- 不把同步分支合入 `custom/main`。
- 不处理与本次同步无关的既有 Trellis 任务、工作区文件或生产部署。

## Notes

- 任务 base branch 为 `custom/main`；同步分支遵循 `sync/upstream-*` 命名。
- 远程更新、分支切换和合并均需保留可回滚边界；共享历史操作（push、合入 `custom/main`）需单独确认。
