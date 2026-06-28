# brainstorm: 上游倍率监控显示账号余额

## Goal

在管理员的上游倍率监控视图中，当上游账号已经绑定连接器时，增加一列展示当前上游账号余额，帮助管理员在查看倍率、可用性和消耗指标时同步判断账号资金状态。

## What I already know

- 用户已经为上游倍率监控添加了连接器，并且存在“绑定上游账号”的场景。
- 当前需求聚焦在监控表格中新增账号余额可见性。
- 当前分支为 `feature/group-visible-rate-multiplier`，已有与上游监控 UI 相关的未提交修改，需要在现有变更基础上增量实现。
- 本仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，修改代码前必须遵守下游 fork 工作流。

## Assumptions (temporary)

- 余额应优先复用连接器或上游账号已有的数据字段，不新增独立轮询或敏感凭据传输。
- 没有绑定连接器或余额不可用时，前端应显示明确的空状态，而不是误导性的 `0`。
- MVP 只展示当前可获得余额，不处理充值、余额告警或历史余额曲线。

## Open Questions

- 已关闭：余额字段需要由后端在连接器同步成功后补齐远端 profile 读取；候选今日用量必须来自上游 Sub2API 普通用户使用记录快照。

## Requirements (evolving)

- 在上游倍率监控的连接器页签中新增“账号余额”列。
- 仅在连接器绑定的上游登录账号可获取余额时展示余额值。
- 连接器余额不可用时展示稳定的空状态文案。
- 候选映射页签展示该候选对应上游连接器与上游分组的今日用量，包括 `actual_cost` 和四类 token 总量；不能从本站 `usage_logs` 聚合。
- 复用现有国际化、格式化和表格展示风格。

## Acceptance Criteria (evolving)

- [x] 管理员打开上游倍率监控页面时，可以在连接器页签看到账号余额列。
- [x] 已绑定连接器并返回余额的上游账号显示格式化后的余额。
- [x] 未绑定或余额未知的连接器显示明确空状态。
- [x] 管理员打开候选映射页签时，可以看到每条候选今日消耗额度和 token 数。
- [x] 中英文文案完整。
- [x] 相关静态检查或针对性构建验证通过，或明确记录无法验证的原因。

## Definition of Done

- Tests added/updated where appropriate.
- Lint / typecheck / targeted build checks pass where practical.
- Docs/notes updated if behavior changes.
- Rollout/rollback considered if risky.

## Out of Scope

- 余额充值、余额告警、余额历史趋势。
- 新增生产连接器凭据或修改生产配置。
- 改动上游账号绑定流程。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，并加入 implement/check 上下文。
- 余额来源为连接器登录到远端 Sub2API 后的 `/api/v1/user/profile`，同步成功后尽力刷新本地快照。
- 本地新增 `upstream_relay_connectors.upstream_account_balance` 和 `upstream_account_balance_checked_at`，仅连接器 DTO / 连接器页签展示这些字段。
- 候选映射 DTO 不暴露上游账号余额；候选页签展示可空的 `today_actual_cost`、`today_total_tokens` 与 `today_usage_checked_at`。
- 候选今日用量在连接器同步时使用上游普通用户 `/api/v1/usage` 明细分页按 `group_id` 聚合后写入 `upstream_relay_group_rate_snapshots`；候选查询按 `connector_id + upstream_group_id` 读取该快照。
- token 口径为 `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`，费用口径为 `actual_cost`；若上游接口不可用、分页过大或字段不完整，安全失败为空，不回退到本站 `usage_logs`。
- 远端 profile 余额读取失败不阻断倍率快照同步；前端显示未同步空状态，避免将未知余额误显示为 `0`。
- 验证记录：`pnpm typecheck`、`pnpm lint:check`、`go vet ./internal/repository ./internal/service ./internal/handler/admin ./internal/server/routes`、`go test -timeout 60s ./internal/repository -run 'RelayCandidateSelect|UpstreamRelay'`、`go test -timeout 60s ./internal/service -run 'UpstreamRelay|ConversationCapture'`、`go test -timeout 60s ./internal/handler/admin -run UpstreamRelay`、`go test -timeout 60s ./internal/server/routes -run UpstreamRelay` 均已通过。
