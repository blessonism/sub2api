# 同步官方 v0.1.152 到二开主线

## Goal

在不丢失 `custom/main` 下游业务定制的前提下，将官方签名标签
`v0.1.152` 安全合入二开主线并推送到 `origin/custom/main`。

## Confirmed Facts

- `custom/main` 已包含当前二开功能，并比 `origin/custom/main` 领先两个提交。
- `v0.1.152` 基于 `v0.1.151`，相对 `custom/main` 有 56 个官方提交。
- 合并预演涉及 128 个文件，发现两个内容冲突：`backend/ent/group.go` 与
  `backend/internal/service/api_key_auth_cache_impl.go`。
- 本次只更新 `custom/main`，不修改 `main`，也不执行生产部署或数据库迁移。

## Requirements

- 基于 `custom/main` 创建 `sync/upstream-v0.1.152`，保留官方提交拓扑。
- 解决两个预检冲突，同时保留下游分组字段和 API Key 鉴权缓存行为。
- 审阅上游新增的 Grok、Codex 搜索、Web Search 计费及迁移 174 数据流。
- 保留下游抽奖、活动中心、账号集合、排行榜及 locale overlay 等定制。
- 执行范围匹配的后端测试、迁移检查、前端类型检查和组件测试。
- 验证通过后提交同步分支，合入并推送 `origin/custom/main`。

## Acceptance Criteria

- [ ] 合并提交包含官方签名标签 `v0.1.152`，且无未解决冲突。
- [ ] `backend/cmd/server/VERSION` 为 `0.1.152`，迁移序列包含 174。
- [ ] 下游独有功能及中英文 locale overlay 未被覆盖或删除。
- [ ] 分组模型同时包含下游字段和上游 Web Search 单次价格字段。
- [ ] API Key 鉴权缓存同时保留下游计费字段和上游新增规则字段。
- [ ] 范围匹配的验证通过，或明确记录与本次同步无关的既有失败。
- [ ] 同步结果合入并推送到 `origin/custom/main`，不修改 `origin/main`。

## Out of Scope

- 生产构建、部署、数据库迁移执行和流量切换。
- 将二开提交合入 `main` 或推送到 `upstream`。
