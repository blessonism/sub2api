# 同步上游 main 到 custom/main

## Goal

在保留当前 `custom/main` 本地二开提交的前提下，把官方 `upstream/main` 的最新变更同步进来，并让 `custom/main` 最终包含双方变更。

## What I Already Know

- 当前工作区干净，当前分支为 `custom/main`。
- `custom/main` 领先 `origin/custom/main` 11 个本地提交。
- `main` / `origin/main` 当前为 `4a5665da5`，`upstream/main` 当前为 `12d811bd7`。
- `main` 可以 fast-forward 到 `upstream/main`。
- `custom/main` 合入 `upstream/main` 的 dry-run 显示存在冲突，主要集中在 Ent 生成代码、后端设置/用户/网关/用量统计、前端用量页与 i18n 文件。
- `git fetch --all --tags --prune` 因本地 `v0.1.145` tag 与 upstream 同名 tag 不一致失败；本任务只同步分支，不处理 tag 归一化。

## Requirements

- 不使用 `git reset --hard` 或其他会丢弃本地提交的操作。
- 先把本地 `main` fast-forward 到 `upstream/main`，保持主线镜像接近官方。
- 在同步分支 `sync/upstream-main-2026-07-10` 上合入最新 `main` 并解决冲突。
- 冲突解决时优先保留下游二开业务能力，同时吸收官方安全修复、结构调整和兼容性修复。
- 同步完成后，将 `sync/upstream-main-2026-07-10` 合入 `custom/main`。

## Acceptance Criteria

- [ ] `main` 指向最新 `upstream/main`。
- [ ] `custom/main` 包含本地 11 个二开提交和最新官方 `upstream/main`。
- [ ] 工作区最终干净，没有未解决冲突。
- [ ] 与冲突范围匹配的后端/前端检查通过，或记录无法通过的具体原因。

## Definition of Done

- 任务上下文记录了下游 fork 分支边界。
- 合并冲突已解决并提交。
- 合并后状态可追溯，最终提交或 merge commit 可通过 `git log` 查看。
- 不执行 push、生产迁移或部署。

## Out of Scope

- 不处理本地与 upstream 同名 tag 冲突。
- 不推送 `origin/main` 或 `origin/custom/main`。
- 不调整生产数据库、容器、DNS、反代或服务器配置。

## Technical Notes

- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 后端冲突需参考 `.trellis/spec/backend/quality-guidelines.md`。
- 前端冲突需参考 `.trellis/spec/frontend/type-safety.md`。
