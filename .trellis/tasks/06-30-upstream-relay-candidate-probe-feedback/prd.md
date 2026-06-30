# fix: 候选映射手动探测反馈对齐账号测试

## Goal

候选映射的手动探测需要像账号管理的账号测试一样，给管理员明确的进行中、成功和失败反馈；失败时直接显示原因，成功时显示关键探测元数据，避免“点了按钮但不知道结果”的体验。

## What I already know

- 用户指出“候选映射的手动探测应该和账号管理的测试一样，而且要有成功、失败反馈”。
- 账号管理的 `AccountTestModal` 使用明确状态区域展示 `connecting / success / error`，失败时展示 `errorMessage`。
- 候选映射当前单次 `probe(candidate)` 只更新行内 `latest_probe`，成功无正反馈，接口异常只写页面顶部错误。
- `probeCandidate` 已返回 `success`、`latency_ms`、`http_status`、`error_class`、`error_message`、`probed_at`，无需改后端。

## Assumptions

- 本轮不改候选探测后端协议，只增强前端反馈。
- 候选探测不需要完整 SSE 终端，关键是状态明确、成功/失败原因可见。

## Requirements

- 点击候选“探测”后立刻显示正在探测的反馈。
- 探测返回成功时显示成功状态、候选映射、耗时、HTTP 状态和时间。
- 探测返回失败或请求异常时显示失败状态和错误原因。
- 保持行内 `latest_probe` 即时更新，以及后续健康聚合静默刷新。

## Acceptance Criteria

- [x] 单次候选探测开始时有明确进行中反馈。
- [x] 成功探测有成功反馈和关键元数据。
- [x] 失败探测有失败反馈和可读错误原因。
- [x] 前端类型检查通过。

## Out of Scope

- 新增后端接口或改探测协议。
- 改批量探测行为。
- 引入账号测试完整流式终端。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 `.trellis/spec/frontend/type-safety.md` 的上游 Relay 前端类型要求。
- 已读取 `.trellis/spec/backend/quality-guidelines.md` 的 Admin upstream relay group monitoring 场景。
