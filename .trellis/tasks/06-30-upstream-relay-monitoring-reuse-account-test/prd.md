# 复用账号管理测试探测上游倍率监控

## Goal

上游倍率监控的探测能力应复用账号管理中已有的账号测试流程，避免监控探测与账号测试在请求构造、鉴权、错误解析、超时和可用性判断上出现两套实现，降低后续维护成本。

## What I already know

- 用户希望“上游倍率监控的探测”复用“账号管理的测试那种”的能力。
- 当前仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，业务改动以 `custom/main` 为基线。
- 任务 base branch 为 `custom/main`，建议工作分支为 `feature/upstream-relay-monitoring-reuse-account-test`。

## Assumptions (temporary)

- 账号管理已有可复用的测试入口或可抽出的测试服务。
- 上游倍率监控当前已有独立探测逻辑，至少部分请求/错误处理逻辑可以收敛。
- 本任务优先改造后端探测逻辑；除非发现前端接口契约必须调整，否则不主动扩展 UI。

## Requirements

- 上游倍率监控探测复用账号管理账号测试的核心请求能力。
- 探测结果仍保留倍率监控所需的业务字段与状态判断。
- 错误信息、超时、鉴权失败、模型不可用等结果应尽量与账号管理测试保持一致。
- OpenAI API Key 候选项的 `probe_protocol` 应通过账号测试路径的临时账号配置体现，不持久化修改账号配置。
- 不引入生产密钥、真实环境数据或与上游主线无关的临时逻辑。

## Acceptance Criteria

- [x] 代码中不再维护两套明显重复的账号测试请求逻辑。
- [x] 上游倍率监控探测能通过账号管理测试能力获得连通性/可用性/错误结果。
- [x] 现有账号管理测试行为不回退。
- [x] 相关后端单元测试或路由测试覆盖复用后的成功与失败路径。

## Definition of Done

- Tests added/updated where appropriate.
- Focused backend tests pass within the 60s unit-test budget when feasible.
- PRD/任务上下文记录关键实现决策。
- 下游 fork 边界已遵守，不向 `main` 或 `upstream` 写入业务定制。

## Out of Scope

- 不重新设计账号管理页面交互。
- 不新增独立的外部探测服务或定时调度系统。
- 不调整生产部署配置或执行生产环境探测。

## Technical Notes

- 必读约束：`.trellis/spec/guides/downstream-fork-workflow.md`。
- 代码勘察：账号管理测试入口位于 `backend/internal/service/account_test_service.go`，上游倍率监控探测入口位于 `backend/internal/service/upstream_relay_group_monitoring.go`。
- 实现决策：在 `AccountTestService` 中新增按账号对象执行的后台测试能力，`UpstreamRelayGroupMonitoringService` 注入同一个账号测试服务实例并调用该能力；账号管理 SSE 测试入口保持原行为。
- 验证记录：聚焦服务测试、受影响 handler/routes/cmd 包测试已通过；服务包全量 `TestUpstreamRelay...` 曾暴露两条刷新 token 错误文案断言失败，和本次探测复用无关。
- 覆盖重点：监控探测复用账号测试服务、缺失账号测试服务时返回明确失败、`probe_protocol=responses` 时通过账号测试路径强制走 `/v1/responses`。
