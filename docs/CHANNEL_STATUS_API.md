# 渠道状态脚本 API

用户可用 API Key 查询当前可见分组的渠道状态。本接口只提供状态，不包含轮询或重试脚本。

```bash
curl -sS -H "Authorization: Bearer $API_KEY" "$BASE_URL/v1/sub2api/channel-status"
```

- 鉴权：`Authorization: Bearer` 或 `x-api-key`。不要把 key 放在 query。
- 限流：同一用户 12 次/分钟；超限 `429` + `Retry-After`。
- 额度耗尽或 Key 所属分组停用时仍可查询。
- `items` 为该用户全部可见分组。`is_key_group=true` / `key_group_connected` 标识当前 Key 绑定分组。
- 顶层 `connected` 表示全部条目都可用。判断「这把 Key 的渠道好了没」请看 `key_group_connected`。
