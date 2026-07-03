# brainstorm: 上游账号流量闸门自动策略文档

## Goal

把“上游倍率监控自动策略”从单纯 priority 调整，收敛成贴近真实运营的“账号 / 候选流量闸门”设计：在低成本优先之外，重点解决高倍率账号仍被 fallback 打到、余额不足账号继续承接流量、策略关闭和恢复不可解释等问题。

## What I already know

- 用户明确指出：只调整 priority 仍可能打到高倍率账号，需要支持开启或关闭账号 / 候选来更有效控制管理。
- 当前候选映射已有 `enabled` 字段，适合第一阶段作为账号视角的候选级流量闸门。
- 当前候选已按项目账号维度绑定上游连接器和上游分组；账号级开关可以先表现为“批量暂停 / 恢复该账号下的候选承接”。
- 当前连接器已有 `paused` 状态，但影响面较大，更适合作为人工运维动作或后续阶段能力。
- 当前系统已有倍率快照、探测结果、健康窗口、账号余额、今日用量、历史日用量和推荐应用记录。
- 当前自动推荐主要生成 priority 建议，自动应用也主要围绕 priority 建议做批量限制。

## Assumptions

- 第一阶段优先提供账号级运营入口，但执行层控制候选是否承接流量，而不是直接启停本地账号或连接器。
- 自动关闭比自动恢复更适合先落地；恢复先做建议和人工确认。
- 运营侧最需要的是可解释、可回滚、不会误伤最后可用候选的策略，而不是复杂权重编辑器。

## Requirements

- 输出正式业务设计文档，落在 `docs/UPSTREAM_RELAY_ACCOUNT_TRAFFIC_GATE_POLICY_CN.md`。
- 文档必须贴近现有 Sub2API 上游中继监控业务，不做抽象规则引擎式过度设计。
- 文档需要明确第一阶段范围、自动关闭规则、恢复建议、兜底保护、与现有字段关系和分阶段落地路线。
- Trellis 任务上下文需要包含下游 fork 工作流约束和本设计文档。

## Acceptance Criteria

- [x] 文档说明为什么 priority 不足以控制高倍率账号流量。
- [x] 文档明确优先复用候选 `enabled`，按账号聚合管理，谨慎使用连接器 `paused`。
- [x] 文档给出最小可用规则，而不是大而全的规则引擎。
- [x] 文档包含自动关闭、恢复建议、兜底保护和自动应用边界。
- [x] 文档列出后续实现前检查项。

## Definition of Done

- 文档已创建或更新。
- Trellis 任务 PRD 已记录本轮需求收敛结果。
- `implement.jsonl` / `check.jsonl` 已加入必要上下文。
- 未修改生产配置、未执行提交、未推送。

## Out of Scope

- 本轮不实现代码。
- 本轮不创建数据库迁移。
- 本轮不调整分支或提交。
- 本轮不设计复杂策略权重编辑器。

## Technical Notes

- 正式设计文档：`docs/UPSTREAM_RELAY_ACCOUNT_TRAFFIC_GATE_POLICY_CN.md`
- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
- 现有 UI 专题文档：`docs/UPSTREAM_RELAY_MONITORING_UI_REDESIGN_CN.md`
- 关键现有表：`upstream_relay_candidates`、`upstream_relay_connectors`、`upstream_relay_recommendation_runs`、`upstream_relay_recommendation_suggestions`
- 当前任务 `base_branch` 仍是 `fix/upstream-relay-candidate-group-select`，后续进入实现前应按下游 fork 工作流校正为 `custom/main`。
