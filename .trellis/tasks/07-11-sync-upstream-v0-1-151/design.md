# 同步设计

## Integration Strategy

以 `custom/main` 为唯一基线创建 `sync/upstream-v0.1.151`，通过一次无提交标签合并把官方发布增量引入工作区。这样既保留官方提交拓扑，又能在提交前检查自动合并结果；若发现语义回归，可在未提交状态下恢复到基线。

官方 release workflow 在标签构建后才把版本文件回写到主线。由于下游 Docker 上下文排除 `.git`，精确标签无法在容器构建中解析，且 OVH 构建脚本未显式提供 `VERSION`；因此同步分支额外采用官方提交 `6c588bb95` 的单行结果，将嵌入版本更新为 `0.1.151`。

## Change Boundaries

- 已存在于 `custom/main` 的修复由共同祖先关系自然去重。
- 新增的 Fast/Flex `user_ids` 从管理端表单进入设置 DTO，经 API Key 鉴权写入可信 `context.Context`，再由策略求值器按“用户规则优先、组内首条命中”执行。
- Codex 身份收口以最终 `User-Agent` 为准推导 `originator`，覆盖 HTTP、透传、WS、账号测试和用量探针；第三方或非法 UA 回退到默认 Codex CLI 身份。
- Grok Responses 保留 OpenAI 兼容的 `reasoning_effort` 并写入转发结果，保证用量记录维度不丢失。
- 迁移 `173` 只扩展 `usage_logs.request_type` 约束，不修改既有数据。

## Downstream Preservation

- 不改 `main`，不 rebase/reset `custom/main`。
- 保留下游 `frontend/src/i18n/locales/<locale>/custom.ts` overlay 和现有活动、中继监控等业务模块。
- 对自动合并的重叠文件逐个审阅，重点确认下游字段与上游新增字段同时存在。

## Rollback

在未提交状态下如验证失败且无法就地修复，可中止合并并切回 `custom/main`；不涉及数据库执行、远端推送或生产状态变更。
