# 同步最新上游 main 并保留二开

## Goal

将官方仓库最新的 `upstream/main` 更新隔离到同步分支，在不直接改写 `custom/main` 的前提下保留并验证本地二开能力。

## Background

- 当前分支为 `custom/main`，工作区干净。
- `upstream` 是官方仓库，`origin` 是二开仓库。
- 仓库规定上游同步必须在 `sync/upstream-*` 分支完成。

## Requirements

- 使用 `git fetch --no-tags --prune` 分别更新 `origin` 与 `upstream` 的远端引用。
- 以同一个已解析的 `upstream/main` 依次执行预检和合并，避免检查对象与合并对象不一致。
- 从 `custom/main` 创建 `sync/upstream-20260826`，仅在该分支合入上游更新。
- 冲突解决必须保留二开行为；无法可靠判断时停止合并并报告，不猜测处理。
- 使用仓库现有 `tools/upstream_sync.py` 生成并执行定向检查。

## Acceptance Criteria

- [x] `origin` 与 `upstream` 的远端分支引用已更新，未抓取或改写本地标签。
- [x] `custom/main` 提交指针保持不变，工作区未丢失任何二开内容。
- [x] 当前分支为 `sync/upstream-20260826`，其历史包含最新的 `upstream/main`；若存在未解决冲突，则保持可诊断状态并明确报告。
- [x] 定向检查通过；环境不可用的检查被明确记录为未验证。

## Validation

- `custom/main` 保持在 `4fe24141ae4c`。
- 已合入 `upstream/main@6ca1e15b0ad2`，同步 merge commit 为 `c08b387380b8`。
- 5 项定向检查在热缓存复跑后全部通过；首次运行的 4 项检查因冷缓存超过 60 秒上限而超时，没有断言失败。
- 两个重叠文件均由 Git 无冲突自动合并，并已人工复核上游增量与二开逻辑可共存。

## Out of Scope

- 不执行 `git commit`、`git push`、创建 PR、合入 `custom/main` 或生产部署。
- 不升级 Trellis，不顺带修改业务代码或重构。
