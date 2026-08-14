# 同步官方 main v0.1.176 到二开主线

## Goal

在不丢失 `custom/main` 下游业务定制的前提下，将官方 `upstream/main`
（当前 `v0.1.176` 及后续 5 个提交，`fbfdcef81`）安全合入二开主线。

## Confirmed Facts

- 本地 `custom/main` 与 `origin/custom/main` 一致，工作区干净。
- 上次已合入官方 `v0.1.175`；相对 `upstream/main` 落后 26 个提交。
- 预检 merge-base 为 `5935e674a`，直接重叠 32 个路径，无未覆盖高风险路径。
- 上游新增分组字段：`long_context_pricing_enabled`、`model_pricing`。
- 下游分组仍保留 `visible_rate_multiplier` 与分时倍率字段。
- 本次只更新二开同步分支，不修改 `main`，不执行生产部署或数据库迁移。

## Requirements

- 基于 `custom/main` 创建 `sync/upstream-2026-08-14`，合入同一个 `upstream/main` ref。
- 解决冲突时同时保留下游二开字段/行为和上游新能力。
- 运行预检命中的定向检查；integration 环境不可用时记录为未验证。
- 验证通过后再考虑推送与合入 `custom/main`。

## Acceptance Criteria

- [x] 同步分支包含官方 `upstream/main`（`fbfdcef81`），且无未解决冲突。
- [x] `backend/cmd/server/VERSION` 为 `0.1.176`，迁移序列包含 `221_group_model_pricing.sql`。
- [x] 下游独有功能及中英文 locale overlay 未被覆盖或删除。
- [x] 分组模型同时包含下游分时/可见倍率字段和上游逐模型定价字段。
- [x] 预检命中的定向检查通过，或明确记录未验证原因。
  矩阵内 `dashboard-repository-integration` 因 60s 超时未计入门禁；用 180s 单独复跑后 4 个 Dashboard SQL 用例均通过。

## Out of Scope

- 生产构建、部署、数据库迁移执行和流量切换。
- 将二开提交合入 `main` 或推送到 `upstream`。
- 未经确认的 `git push`。
