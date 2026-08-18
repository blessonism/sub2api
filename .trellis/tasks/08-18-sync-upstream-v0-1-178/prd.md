# 同步官方 main v0.1.178 到二开主线

## Goal

在不丢失 `custom/main` 下游业务定制的前提下，将官方 `upstream/main`
（当前 `v0.1.178`，`49504adc9`）安全合入二开主线。

## Confirmed Facts

- 本地 `custom/main` 为 `ab4df1086`，比 `origin/custom/main` 超前 1 个提交（自动策略审计空条件修复），工作区干净。
- 上次已合入官方 `v0.1.176`；相对 `upstream/main` 落后 120 个提交、超前 251 个二开提交。
- 预检 merge-base 为 `fbfdcef81`，直接重叠 97 个路径，无未覆盖高风险路径。
- 受影响下游能力：dashboard 运营指标、i18n overlay、Token 排行榜、公告邮件、分时倍率、用户公开分组账号绑定。
- 上游主要新增：渠道监控配额模式、国内平台渠道定价、Grok 用量聚合、邀请码 TOCTOU 修复、Codex/OpenAI 工具桥接。
- 本次只更新二开同步分支，不修改 `main`，不执行生产部署或数据库迁移。

## Requirements

- 基于 `custom/main` 创建 `sync/upstream-2026-08-18`，合入同一个 `upstream/main` ref。
- 解决冲突时同时保留下游二开字段/行为和上游新能力。
- 运行预检命中的定向检查；integration 环境不可用时记录为未验证。
- 验证通过后再考虑推送与合入 `custom/main`。

## Acceptance Criteria

- [x] 同步分支包含官方 `upstream/main`（`49504adc9`），且无未解决冲突。
- [x] `backend/cmd/server/VERSION` 为 `0.1.178`。
- [x] 下游独有功能及中英文 locale overlay 未被覆盖或删除。
- [x] 预检命中的定向检查通过，或明确记录未验证原因。
  20 项定向检查（含 frontend / backend / integration）均通过。

## Out of Scope

- 生产构建、部署、数据库迁移执行和流量切换。
- 将二开提交合入 `main` 或推送到 `upstream`。
- 未经确认的 `git push`。
