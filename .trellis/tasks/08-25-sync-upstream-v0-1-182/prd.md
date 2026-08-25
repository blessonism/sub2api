# 同步官方 main v0.1.182 到二开主线

## Goal

在不丢失 `custom/main` 下游业务定制的前提下，将官方 `upstream/main`
（当前 `v0.1.182`，`aa2c4e8d1`）安全合入二开主线。

## Confirmed Facts

- 本地 `custom/main` 为 `30b9124b3`，与 `origin/custom/main` 一致。
- 工作区原有 5 处未提交部署文档改动，已暂存为
  `stash@{0}: wip: ovh databasus backup docs before upstream sync 2026-08-25`。
- 上次已合入官方 `v0.1.178`；相对 `upstream/main` 落后 286 个提交、超前 256 个二开提交。
- 预检 merge-base 为 `49504adc9`，直接重叠 90 个路径，无未覆盖高风险路径。
- 受影响下游能力：dashboard 运营指标、i18n overlay、Token 排行榜、公告邮件、分时倍率、用户公开分组账号绑定。
- 上游主要新增：OAuth outbound plugin、渠道分时/区间定价、Go 1.27、Responses Lite 并行工具、CN provider 自适应协议、Grok 兼容与 Realtime 修复。
- 本次只更新二开同步分支，不修改生产环境。本地 `main` 仅在确认后快进为官方镜像。

## Requirements

- 基于 `custom/main` 创建 `sync/upstream-2026-08-25`，合入同一个 `upstream/main` ref。
- 解决冲突时同时保留下游二开字段/行为和上游新能力。
- 运行预检命中的定向检查；integration 环境不可用时记录为未验证。
- 验证通过后再考虑推送与合入 `custom/main`。

## Acceptance Criteria

- [x] 同步分支包含官方 `upstream/main`（`aa2c4e8d1`），且无未解决冲突。
- [x] `backend/cmd/server/VERSION` 为 `0.1.182`。
- [x] 下游独有功能及中英文 locale overlay 未被覆盖或删除。
- [x] 预检命中的定向检查通过，或明确记录未验证原因。
  `custom/main..HEAD` 命中的 17 项定向检查均通过；公告邮件 3 项作为预检高风险补充检查另行执行。
- [x] 恢复公开设置 `token_leaderboard_user_visible` 与用户侧栏排行榜开关；对应 unit/Vitest 通过。
- [x] 本地 `custom/main` 快进到同步分支（含官方 v0.1.182 与上述回归修复）。

## Out of Scope

- 生产构建、部署、数据库迁移执行和流量切换。
- 将二开提交合入 `main` 或推送到 `upstream`。
- 未经确认的 `git push`。
