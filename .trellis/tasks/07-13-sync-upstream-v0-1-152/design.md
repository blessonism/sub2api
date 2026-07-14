# 同步设计

## Integration Strategy

从当前 `custom/main` 创建版本化同步分支，通过 `git merge --no-ff v0.1.152`
保留官方标签的提交拓扑。所有冲突先在同步分支处理并验证，随后再将同步分支
合回 `custom/main`，避免直接在二开主线上调试大规模上游变更。

## Conflict Policy

- `backend/ent/group.go`：以生成模型的完整字段集合为准，同时保留下游已有字段
  与上游 `web_search_price_per_call` 字段，并核对 schema、mutation、create/update
  生成文件一致。
- `backend/internal/service/api_key_auth_cache_impl.go`：按 API Key 鉴权数据流合并，
  不以简单选择 ours/theirs 解决；逐项核对缓存 DTO 构造和可信上下文写入。

## Preservation Checks

- 通过祖先和差异检查确认抽奖、活动中心、排行榜、账号集合及下游部署文件仍在。
- 核对 `frontend/src/i18n/locales/*/custom.ts` overlay 未被上游模块化 locale 覆盖。
- 核对迁移 174 追加在现有迁移序列之后，不执行迁移。

## Rollback

推送前可中止合并或删除同步分支，不影响 `custom/main`。推送后使用 revert 合并
提交回滚，不重写远程历史。
