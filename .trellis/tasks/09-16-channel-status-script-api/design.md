# 渠道状态脚本 API 技术设计

## Boundaries

- **入口**：`GET /v1/sub2api/channel-status`，挂在网关 `/v1` 组、与 `/v1/sub2api/billing` 同级，位于 `groupModelAllowlist` / `requireGroupAnthropic` 之前，避免未分组 Key 被对话中间件拦掉。
- **协调**：新建 `service.ChannelStatusService`，只编排已有能力，不新增探测。
- **依赖**：`SettingService.GetChannelMonitorRuntime`、`APIKeyService.GetAvailableGroups`、`ChannelMonitorService.ListUserView`（V1）、`ChannelMonitorV2Service.Matrix`（V2，`group_by=platform_group`，`range=90m`）。
- **Handler**：新建 `handler.ChannelStatusHandler`，不要塞进已经过大的 `GatewayHandler`。
- **不改**：面板 JWT 的 `/api/v1/channel-monitors*` 与 `/api/v1/channel-monitor-v2*`。

## Auth

复用 `APIKeyAuthMiddleware`，在 `api_key_auth.go` 把本路径纳入只读自省：

1. `skipBilling` 增加 `/v1/sub2api/channel-status`（额度耗尽/过期仍可查）。
2. 本路径跳过 `abortIfAPIKeyGroupUnavailable` / `abortIfAPIKeyGroupNotAllowed`。状态看的是用户可见分组，不是这把 Key 当前能不能发对话。
3. 仍校验 Key 存在、未 disabled、用户 active、IP 黑白名单。
4. 仍拒绝 query `key` / `api_key`。
5. 成功后设置 `ContextKeyAPIKey` / `ContextKeyUser`，供 handler 取 `UserID`。
6. SimpleMode：同样放行只读查询；若监控功能关闭则按 R2 返回空列表。

路径判断抽成小函数（与 `isAsyncImageTaskRead` 同类），避免再复制一串 `== "/v1/..."`。

## Rate limit

独立 Redis 固定窗口，不复用面板 `UserRPM`/`HeavyRPM`，也不走对话 RPM。

- key：`channel-status:user:{userID}`
- limit：12 / 分钟（常量）
- 维度：用户 ID（多 Key 共用）
- Redis 异常 fail-open
- 管理员不豁免
- 超限：`429` + `Retry-After`，body 与网关一致：`{"error":{"type":"rate_limit_error","message":"..."}}`

`RegisterGatewayRoutes` 目前没有 Redis。从 `registerRoutes` 把 `redisClient` 传进去，用现有 `internal/middleware.RateLimiter.Allow`。限流中间件挂在该 GET 上、鉴权之后（需要 UserID）。

## Data flow

```
API Key → auth (skip billing / skip group gate)
       → rate limit (user 12/min)
       → ChannelStatusHandler
       → ChannelStatusService.Get(ctx, userID, keyGroupID, keyGroupName)
            → runtime = GetChannelMonitorRuntime
            → groups = GetAvailableGroups(userID)
            → if !runtime.Enabled or groups empty → empty items（仍可带 key_group_*）
            → v1: ListUserView → 保留 GroupName 命中可见分组名的条目
            → v2: Matrix(90m, platform_group, RestrictGroups=true, AllowedGroupIDs)
            → 按 keyGroupID（优先）或 keyGroupName 给 items 打 is_key_group
            → 汇总 key_group_connected / key_group_status
       → JSON
```

V1 监控只有 `GroupName` 没有 `group_id`：用可见分组 `Name` 精确匹配过滤；匹配到时补 `group_id`。未匹配的监控丢弃（防止把用户不能用的分组交给脚本）。空 `GroupName` 丢弃。

V2 已有 `group_id` / `group_name`，用 `AllowedGroupIDs` 约束。不返回 `buckets`、metric 计数、RPM/TPM。

## Contract

成功（HTTP 200，无信封）：

```json
{
  "object": "sub2api.channel_status",
  "schema_version": 1,
  "mode": "v2",
  "connected": false,
  "key_group_id": 12,
  "key_group_name": "plus",
  "key_group_connected": true,
  "key_group_status": "healthy",
  "checked_at": "2026-09-16T12:00:00Z",
  "item_count": 2,
  "connected_count": 1,
  "items": [
    {
      "group_id": 12,
      "group_name": "plus",
      "name": "plus",
      "provider": "openai",
      "status": "healthy",
      "connected": true,
      "is_key_group": true
    },
    {
      "group_id": 13,
      "group_name": "pro",
      "name": "pro",
      "provider": "openai",
      "status": "critical",
      "connected": false,
      "is_key_group": false,
      "error_category": "rate_or_capacity"
    }
  ]
}
```

`connected`（顶层）= `item_count > 0 && connected_count == item_count`。

`is_key_group`：`group_id` 等于当前 Key 的 `GroupID`；V1 无 ID 时退回 `group_name` 与 Key 分组名精确匹配。Key 未绑分组则全部为 false。

`key_group_connected`：
- Key 未绑分组 → `null`
- `items` 中没有任何 `is_key_group=true` → `null`（分组停用、无监控数据等）
- 否则：这些条目全部 `connected=true` 才为 `true`
- `key_group_status`：仅一条时用该条 `status`；多条时，全部可用用其中一条 status，否则优先不可用那条

Handler 从 `ContextKeyAPIKey` 取 `GroupID` / `Group.Name` 传给 service，不要只传 userID。

单条 `connected`：

| 模式 | 可用 | 不可用 |
|---|---|---|
| V1 | `status=operational` 且非 `rate_or_capacity` | `degraded` / `failed` / `error` / `rate_or_capacity` |
| V2 | `healthy` 或 `warning` 且非 `rate_or_capacity` | `critical` / `unknown` / `rate_or_capacity` |

`Cache-Control: no-store`。

鉴权失败保持现有 `AbortWithError` / 网关错误行为，不强行改成另一种信封。限流用网关 `rate_limit_error`。

## Compatibility

- 新路径，无存量客户端。
- 不改变 `/v1/sub2api/billing`、`/v1/usage` 的 skipBilling 行为，只是集合多一个路径。
- `schema_version=1`，以后加字段保持向后兼容。

## Rollout / rollback

- 纯只读新路由。回滚 = 撤掉路由与 skipBilling 分支。
- 限流 fail-open，不会因 Redis 把网关打挂。
- 监控关闭时 200 空列表，避免脚本把 404 当鉴权失败。

## Trade-offs

- 放在 `/v1/sub2api/*` 而不是 `/api/v1/*`：脚本与 Codex 共用 base URL + API Key；代价是要扩展网关鉴权白名单。
- 顶层 `connected` 表示全绿：信息完整，调用方按 `items` 自己判断目标分组。
- V1 用分组名匹配：受监控配置里 `GroupName` 质量约束；比把全部 enabled 监控泄漏给任意用户更安全。
- 12 RPM 硬编码：满足防刷，避免为 MVP 加设置项。
