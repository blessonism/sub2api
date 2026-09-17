# 渠道状态脚本 API

用户可用 API Key 查询**当前这把 Key 绑定分组**的渠道状态。接口只提供状态，不包含轮询或重试脚本。

```bash
curl -sS -H "Authorization: Bearer $API_KEY" "$BASE_URL/v1/sub2api/channel-status"
```

成功响应示例：

```json
{
  "object": "sub2api.channel_status",
  "schema_version": 2,
  "checked_at": "2026-09-17T10:58:09Z",
  "group_id": 40,
  "group_name": "luna 分组",
  "connected": false,
  "status": "degraded"
}
```

- 鉴权：`Authorization: Bearer` 或 `x-api-key`。不要把 key 放在 query。
- 限流：同一用户 12 次/分钟；超限 `429` + `Retry-After`。
- 额度耗尽或 Key 所属分组停用时仍可查询。
- `connected`：这把 Key 的渠道当前是否可用。没有对应监控时为 `null`。
- `status`：`operational` / `degraded` / `failed` / `error` / `unknown`。
- 可选 `?scope=visible`：额外返回 `items`，列出当前用户可见分组的状态。顶层 `connected` / `status` 仍然只描述这把 Key。
