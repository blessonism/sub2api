# brainstorm: 上游分组倍率监控保存候选失败原因详情

## Goal

管理员在上游分组倍率监控中保存候选失败时，应看到接口返回的具体原因，避免只看到“保存候选失败”这类无法诊断的泛化提示。

## What I Already Know

- 用户反馈当前保存候选失败只显示“保存候选失败”，原因不够详细。
- 前端 API 拦截器会把后端失败包装成普通对象，例如 `{ status, code, message }`。
- 当前页面保存候选 catch 分支使用 `err instanceof Error`，普通对象错误会被误判并落到 fallback 文案。
- 项目已有 `extractApiErrorMessage`，可统一读取普通对象、Axios legacy shape 和标准 Error。
- 当前任务遵循下游二开工作流，base branch 为 `custom/main`，工作分支为 `feature/upstream-relay-policy-preview`。

## Requirements

- 保存候选失败时优先展示后端返回的 `message` / `error` / `detail`。
- 当错误对象没有可展示详情时，继续使用现有 fallback 文案。
- 不改变候选保存接口、请求 payload 或后端错误语义。

## Acceptance Criteria

- [ ] API reject 普通对象并携带 `message` 时，页面展示该详细原因。
- [ ] API reject 无可读详情时，页面仍显示 `saveCandidateFailed` fallback。
- [ ] 现有轻量刷新提示测试不受影响。

## Definition of Done

- 聚焦单测通过。
- 变更范围保持在上游分组倍率监控页面和相关测试内。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取前端 `type-safety.md` 和后端 `quality-guidelines.md` 中上游中继监控相关规则。
- 根因是前端错误对象形态判断过窄，不是后端未返回详情。
