# fix: 上游中继监控用本地统计刷新 usage

## Goal

轻量刷新上游中继连接器指标时，余额仍从上游 profile 刷新，但候选分组今日 usage 改为优先使用本地 `usage_logs` 统计，避免为监控页扫描上游 `/api/v1/usage` 明细导致响应过大或 JSON 截断。

## What I already know

- 用户反馈余额刷新成功后，usage 报 `parse upstream usage page: unexpected end of JSON input`。
- 当前 `RefreshConnectorMetrics` 会请求上游 `/api/v1/usage` 并按 `group_id` 聚合。
- 本地 `usage_logs` 已有 tokens、`actual_cost`、`account_id`、`group_id` 等统计字段。
- 现有后端规范仍写着候选 usage 不应聚合本地 `usage_logs`，这与本次产品方向冲突，需要同步修正。

## Requirements

- 轻量指标刷新不再调用上游 `/api/v1/usage` 作为主路径。
- 候选分组今日 usage 使用本地 `usage_logs`，经候选映射聚合到 `connector_id + upstream_group_id`。
- 余额刷新继续使用上游 `/api/v1/user/profile`。
- 统计失败时保持现有降级语义：清空今日 usage 字段并返回 sanitized `usage_error`。
- 更新后端规范与测试，避免后续实现回退到上游明细扫描。

## Acceptance Criteria

- [x] `RefreshConnectorMetrics` 成功刷新余额时不会请求上游 `/api/v1/usage`。
- [x] 本地 usage 统计按候选的 `account_id + target_group_id` 汇总，并写回对应 snapshot 的 `today_actual_cost`、`today_total_tokens`。
- [x] 没有本地 usage 的已有 snapshot 在成功统计后显示零值，而不是保留旧值。
- [x] 测试覆盖本地统计路径与不调用上游 usage 的行为。

## Definition of Done

- Tests added/updated where appropriate.
- Backend spec updated for the new source-of-truth decision.
- No git commit or push in this task.

## Technical Notes

- Branch: `feature/upstream-relay-policy-preview`
- Downstream fork guide: `.trellis/spec/guides/downstream-fork-workflow.md`
- Main files: `backend/internal/service/upstream_relay_group_monitoring.go`, `backend/internal/repository/upstream_relay_group_monitoring_repo.go`, related tests.
