# 修复 Plus/Pro Responses 渠道监控流式探测

## Goal

让 Plus（group 3）和 Pro（group 22）的 Responses 分组监控以“是否能在合理时间产出有效响应”为可用性依据，避免网关已经成功但探针因等待完整非流式响应而产生 `error`。同时把用户已确认的 60 秒等待上限落实到完整超时链路。

## Requirements

- Responses 监控请求使用流式响应，不改变 Chat Completions 或其他 provider 的探测行为。
- 流式探测必须区分首个有效输出、正常完成、上游失败、无输出和读取超时；首包成功但完整响应较慢时不能记为 `error`。
- 监控仍需校验 arithmetic challenge；收到有效文本后才允许判定为 operational/degraded，`response.failed` 或 challenge 不匹配仍按失败处理。
- 响应头等待上限提升到 60 秒；客户端总超时与 runner 外层超时必须覆盖该值，不能出现“配置 60 秒但 45 秒提前取消”。
- 不修改生产数据库、账号状态、API Key、代理或部署配置；本任务只提交可复现的代码和测试。

## Confirmed Evidence

- `backend/internal/service/channel_monitor_checker.go:248-257` 当前 Responses body 固定 `stream=false`。
- `backend/internal/service/channel_monitor_checker.go:528-551` 当前等待读完整 body 后才解析文本。
- OVH 2026-09-03 21:20:44（北京时间）的 Pro 监控请求在探针侧约 30 秒记为 `error`，网关同一 request 在 21:21:15 返回 HTTP 200，耗时 31.116 秒。
- Plus/Pro 账号池同时存在有效账号不足、无效 Key、403/429/5xx 等真实不稳定因素；流式探测只能消除判定假阴性，不能替代账号池治理。

## Acceptance Criteria

- [ ] Responses 探测请求发送 `stream=true`，并能解析现有 Responses SSE 数据行。
- [ ] 首个包含 challenge 答案的有效文本在 60 秒内到达、但完整流结束较慢时，结果不是 `error`；耗时超过 6 秒按现有 degraded 口径记录，且保留可诊断延迟信息。
- [ ] 无有效输出、`response.failed`、提前 EOF/协议不完整、最终非 2xx 或超过总超时时，结果仍为 `error`/`failed`，不会误判为 operational；只有客户端独立读超时且已有有效 challenge 输出时允许记为 `degraded`。
- [ ] Responses 流式探测的 challenge 校验、错误脱敏、响应体大小限制和历史落库行为保持有效。
- [ ] 现有 channel monitor checker 相关测试通过，并新增覆盖流式成功、慢完成、失败事件和超时的回归测试。
- [ ] 60 秒相关常量之间一致：响应头超时不超过客户端总超时，runner 外层超时不早于客户端总超时加 ping/缓冲时间。

## Out of Scope

- 不在本任务内清理或暂停 Plus/Pro 账号，不调整调度算法、熔断阈值或生产容器。
- 不把所有慢响应简单改成 operational；探针仍必须报告真实的无输出和协议失败。
