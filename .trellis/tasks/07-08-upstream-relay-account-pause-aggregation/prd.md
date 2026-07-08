# Upstream Relay Account Pause Aggregation

## Goal

修正上游倍率监控推荐中的账号暂停语义：暂停动作会写入账号级 `accounts.schedulable=false`，因此只有同一账号下没有任何健康可用候选分组时，才应生成账号暂停建议。单个分组异常只应影响该候选的推荐资格，不能阻断同账号其他健康低倍率分组继续参与 priority 推荐。

## Requirements

- 推荐预览先对候选进行健康/倍率排除判断，再按 `AccountID` 聚合是否存在健康可用候选。
- 同一账号存在至少一个健康可用候选时，该账号的异常候选只进入 exclusions，不生成 `account_pause`。
- 同一账号没有健康可用候选时，仍可基于现有暂停原因白名单生成一条 `account_pause`。
- 保留现有保护：不暂停同平台最后一个可调度账号、已有 active gate state 不重复暂停、账号闸门建议不自动应用。
- 同账号多健康候选仍只保留排序最优的一条 priority 建议。

## Acceptance Criteria

- [ ] 同账号一个失败候选、一个健康候选时，生成健康候选的 `priority_update`，不生成 `account_pause`。
- [ ] 同账号所有候选均不可用时，只生成一条 `account_pause`。
- [ ] 同平台最后一个可调度账号仍不生成暂停建议。
- [ ] gate active 且恢复健康的账号仍生成 `account_resume`。
- [ ] 自动应用遇到 account gate 建议仍返回 `account_gate_suggestion_requires_manual_apply`。

## Definition of Done

- 更新后端推荐生成逻辑和相关单元测试。
- 针对性 Go 测试通过。
- 不改 API、数据库 schema、前端字段。

## Technical Approach

在 `buildUpstreamRelayRecommendationPreview` 内引入候选评估结果：先记录每个候选的 exclusion/eligible 状态，并用账号级集合标记“该账号是否存在健康可用候选”。第二阶段只对没有健康候选的账号尝试生成暂停建议；健康候选继续进入现有排序和 priority 去重流程。

## Decision (ADR-lite)

Context: 当前实现按候选分组触发账号暂停，但实际应用写账号级 `schedulable`，导致一个异常分组会暂停整个账号。

Decision: 将暂停建议门槛提升到账号级聚合判断，不新增候选级真实流量开关。

Consequences: `account_pause` 数量会减少；异常候选仍可被解释为 exclusions；后续如需候选级真实暂停，需要另行设计调度层闸门。

## Out of Scope

- 不新增配置项或复杂权重。
- 不新增数据库迁移。
- 不实现候选级真实调度闸门。
- 不改前端展示字段。

## Technical Notes

- 相关实现：`backend/internal/service/upstream_relay_group_monitoring.go`。
- 相关测试：`backend/internal/service/upstream_relay_group_monitoring_test.go`。
- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
