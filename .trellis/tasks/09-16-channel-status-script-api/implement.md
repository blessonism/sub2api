# 渠道状态脚本 API 实现清单

## Checklist

1. **鉴权白名单**
   - 在 `backend/internal/server/middleware/api_key_auth.go` 抽出只读自省路径判断。
   - `/v1/sub2api/channel-status`：`skipBilling=true`，跳过分组停用/无权拦截。
   - 单测：过期/额度耗尽/分组停用的 Key 可以进入 handler；disabled Key / 错误 Key / query key 仍拒绝。

2. **限流中间件**
   - 新增按用户 12/min 的固定窗口中间件（Redis `Allow`，fail-open）。
   - `RegisterGatewayRoutes` 增加 `redisClient` 参数，仅挂在本 GET。
   - 单测：第 13 次 429 + `Retry-After`；Redis 错误放行。

3. **Service**
   - 新增 `ChannelStatusService.Get(ctx, userID, keyGroupID, keyGroupName)`。
   - V1：`ListUserView` + 可见分组名过滤 + operational 映射。
   - V2：`Matrix(90m, platform_group)` + `RestrictGroups`。
   - 给匹配当前 Key 分组的条目打 `is_key_group`，并汇总 `key_group_*`。
   - 关闭/无可见分组/无匹配：空 `items`，`connected=false`；有绑定分组时仍返回 `key_group_id/name`，`key_group_connected=null`。
   - 单测：可见范围、全绿/部分异常、`rate_or_capacity`、空数据、V1 名匹配补 `group_id`、Key 绑定分组标识、未绑定、绑定分组不在 items 中。

4. **Handler + 路由**
   - 新增 `ChannelStatusHandler`，注册 `GET /v1/sub2api/channel-status`（billing 同级）。
   - Wire 进 `Handlers`。
   - 路由测试：路径存在；未鉴权 401。

5. **契约测试**
   - 成功 JSON 字段、脱敏（无 endpoint/quota/metrics 计数）。
   - `api_contract_test` / gateway 路由表补一条，避免注册遗漏。

6. **文档**
   - handler 注释写清路径、鉴权、字段。
   - 若已有用户向 API 说明（如 README_CN 或渠道状态相关 docs），补路径与一行 curl；不写重试脚本。

## Validation

```bash
cd backend && go test ./internal/server/middleware/ ./internal/server/routes/ ./internal/handler/ ./internal/service/ -count=1
```

聚焦：

- `api_key_auth` 只读路径
- `channel_status` service/handler
- gateway 路由注册

前端无改动，不必跑 pnpm。

## Risk / rollback

- 改 `RegisterGatewayRoutes` 签名：所有测试构造点要一起改（参考 `gateway_key_billing_test.go`）。
- skipBilling 路径判断写错会让对话请求跳过计费：必须用精确 path，单测覆盖 `/v1/messages` 不受影响。
- 回滚：还原路由、鉴权白名单、限流中间件。

## Before start

- 从 `custom/main` 拉 `feature/channel-status-script-api`。
- `implement.jsonl` / `check.jsonl` 已含下游 fork 工作流。
- 实现前读 `trellis-before-dev`。
