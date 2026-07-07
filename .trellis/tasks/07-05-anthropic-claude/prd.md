# 上游倍率监控支持 Anthropic Claude 模型探测

## Goal

让上游倍率监控候选账号除现有 OpenAI `chat_completions` / `responses` 探测外，也能使用 Anthropic Messages 原生格式探测 Claude 模型，从而验证 Claude 上游模型可用性，并继续服务于候选账号健康度、用量差分倍率推导和推荐排序。

## What I Already Know

- 用户希望“增加 anthropic 的格式来探测 claude 的模型”，核心不是仅增加模型名，而是新增 Anthropic 原生请求协议。
- 现有候选绑定包含 `probe_model` 和 `probe_protocol`，当前协议前后端类型只支持 `chat_completions` 与 `responses`。
- 后端 `runCandidateProbe` 统一复用 `AccountTestService.RunAccountTestBackground`，再由账号平台分流到 OpenAI / Gemini / Antigravity / Claude 测试逻辑。
- Anthropic Claude API Key 测试路径已经存在，会向 `/v1/messages?beta=true` 发送 Claude Code 风格 payload，并设置 `anthropic-version`、`anthropic-beta`、`x-api-key` 等请求头。
- OpenAI 候选探测目前会通过 `accountForUpstreamRelayProbe` 强制选择 chat completions 或 responses；新增 Anthropic 协议时应复用这个选择点，避免复制完整探测链路。
- 本仓库是下游二开 fork，任务 base branch 已设置为 `custom/main`，工作分支记录为 `feature/anthropic-claude-probe`。

## Assumptions

- 新协议名暂定为 `anthropic`，表示 Anthropic Messages API 原生格式。
- 该协议主要服务 Claude / Anthropic 兼容上游；OpenAI 的 `responses` 语义不应扩展到 Anthropic。
- 探测成功/失败仍沿用现有 `UpstreamRelayProbeResult`、候选健康度和批量探测反馈，不新增独立结果表。
- 用量差分采样如果当前账号和上游分组支持计费数据，仍沿用现有 `buildUsageDeltaSample` 逻辑。

## Requirements

- 候选探测协议枚举新增 Anthropic Messages 格式。
- 后端候选输入校验接受新协议，并给出清晰错误文案。
- 候选执行探测时，`probe_protocol=anthropic` 应以 Anthropic Messages 请求体和请求头探测 `probe_model`。
- 前端候选新增/编辑表单允许选择 Anthropic 协议，类型定义同步更新。
- 候选列表、单个探测、批量探测、自动探测和推荐逻辑继续兼容旧数据。
- 默认值保持向后兼容：旧候选未设置协议时仍归一为 `chat_completions`。

## Acceptance Criteria

- [ ] 后端可创建/更新 `probe_protocol=anthropic` 的上游候选。
- [ ] 使用 Claude / Anthropic API Key 账号时，候选探测请求走 `/v1/messages` Anthropic Messages 格式。
- [ ] OpenAI 候选现有 `chat_completions` 与 `responses` 探测行为不回归。
- [ ] 前端候选表单显示并提交 Anthropic 协议选项。
- [ ] 单测覆盖协议校验、Anthropic 协议探测路由、现有 OpenAI 协议不回归。

## Out Of Scope

- 不在本任务内重做上游登录抓取或倍率快照采集。
- 不在本任务内新增独立 Anthropic 价格表、模型列表同步或模型自动发现。
- 不改变推荐排序策略，只补齐可用性探测协议。
- 不触碰生产部署、真实密钥或数据库迁移执行。

## Technical Notes

- 现有探测入口：`backend/internal/service/upstream_relay_group_monitoring.go`
- 现有账号测试入口：`backend/internal/service/account_test_service.go`
- 现有监控 API mode 常量与校验参考：`backend/internal/service/channel_monitor_const.go`、`backend/internal/service/channel_monitor_validate.go`
- 前端候选协议类型与表单：`frontend/src/api/admin/upstreamRelayGroupMonitors.ts`、`frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`

## Open Questions

- Anthropic 协议是否只允许 Anthropic/Claude 平台账号使用，还是允许任意账号选择但由后端按账号能力失败？

## Definition Of Done

- 代码实现完成且覆盖核心测试。
- lint / typecheck / 单测按变更范围通过或记录阻塞原因。
- `implement.jsonl` 与 `check.jsonl` 包含相关 spec / research context。
- 任务 PRD 与关键设计决策保持同步。
