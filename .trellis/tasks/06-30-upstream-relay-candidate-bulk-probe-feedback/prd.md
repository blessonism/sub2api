# fix: 候选映射一键探测反馈

## Goal

候选映射的一键探测需要和单个手动探测一样有明确反馈：开始时可见、完成后显示成功/部分失败/失败，失败项能直接看到候选和原因。

## What I already know

- 单个候选探测已经有 `candidateProbeFeedback` 反馈面板。
- 一键探测当前调用 `probeAllCandidates()`，返回 `UpstreamRelayBulkOperationResult`，包含 `total`、`success`、`failed` 和失败 `items`。
- 现有逻辑只把批量结果写入 `bulkOperationResult`，并在部分失败时设置页面顶部 `error`。
- 当前工作树存在其他未提交改动，本任务只追加候选一键探测反馈，不回退其他人的改动。

## Requirements

- 点击“一键探测”后立即显示进行中反馈。
- 成功完成时显示总数、成功数、失败数。
- 部分失败或失败时显示失败项摘要和错误原因。
- API 异常时显示失败反馈，不只依赖页面顶部错误。

## Acceptance Criteria

- [x] 一键探测开始时有明确进行中反馈。
- [x] 一键探测成功/部分失败/失败都有明确状态。
- [x] 失败项可读，至少展示前几条失败原因。
- [x] 前端类型检查通过。

## Out of Scope

- 修改后端批量探测接口。
- 改变自动监控批量探测逻辑。
- 回退当前工作树中的其他未提交改动。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 `.trellis/spec/frontend/type-safety.md` 的上游 Relay 前端类型要求。
