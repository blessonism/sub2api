# Upstream Relay Configurable Pause Strategies

## Goal

将上游倍率监控的 `account_pause` 建议改为由管理员显式开启的可验证策略触发，避免仅因候选不可用就轻易暂停整个账号。

## Requirements

- 推荐策略新增三个 pause 策略开关：成本差、连续失败、低成功率，默认全部关闭。
- 成本差策略只在跨账号可替代时暂停高倍率账号：不同账号、同平台、同探测协议、同探测模型，且倍率差达到阈值。
- 连续失败策略只在账号下没有健康候选，且连续失败次数达到阈值时触发。
- 低成功率策略只在账号下没有健康候选，且样本数/成功率达到既有门槛时触发。
- `missing_fresh_rate`、`stale_probe`、普通 `latest_probe_failed` 只作为 exclusion，不直接触发 pause。
- 保留现有保护：不暂停同平台最后一个可调度账号、已有 active gate state 不重复暂停、account gate 建议不自动应用。
- 前端策略页允许管理员开关策略和调整阈值。

## Acceptance Criteria

- [ ] 所有 pause 策略关闭时不生成 `account_pause`。
- [ ] 成本差策略开启且跨账号低倍率候选可替代时生成 `account_pause`，原因包含当前倍率、替代倍率、差值和阈值。
- [ ] 同账号存在不可替代健康候选时，成本差策略不生成 pause。
- [ ] 连续失败达到阈值时生成 pause，低于阈值只 exclusion。
- [ ] 低成功率策略开启且样本数达标、成功率低于阈值时生成 pause。
- [ ] 策略 API 能保存/返回新增字段，前端能展示/提交。

## Definition of Done

- 后端迁移、策略模型、仓储、服务逻辑和测试更新完成。
- 前端 API 类型、策略表单和测试更新完成。
- 针对性后端和前端验证通过。

## Technical Approach

在 `upstream_relay_recommendation_policy` 增加 pause 策略字段；服务层基于已计算的候选 eligibility/exclusion 生成策略证据，只在显式策略命中时调用账号级 `account_pause`。前端复用现有策略页面，新增 Pause 策略控制区。

## Out of Scope

- 不新增候选级真实调度闸门。
- 不自动应用 `account_pause` / `account_resume`。
- 不新增硬失败错误类型策略。

## Technical Notes

- 相关后端实现：`backend/internal/service/upstream_relay_group_monitoring.go`、`backend/internal/repository/upstream_relay_group_monitoring_repo.go`。
- 相关前端实现：`frontend/src/api/admin/upstreamRelayGroupMonitors.ts`、`frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`。
- 需遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
