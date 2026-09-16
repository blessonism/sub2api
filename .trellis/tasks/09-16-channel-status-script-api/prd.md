# 用户 API Key 渠道状态查询接口

## Goal

开放一个用户 API Key 可调用的只读渠道状态接口，把该用户所有可见分组的当前状态传清楚。用户自己写轮询/重试脚本；本任务不提供脚本。

## Background

用户端渠道状态页已有只读接口，但走面板 JWT：`GET /api/v1/channel-monitors`（V1）和 `GET /api/v1/channel-monitor-v2/snapshot`（V2）。JWT 会过期，不适合长期脚本。用户 API Key 目前只用于网关 `/v1/*`（对话转发、`/v1/usage`、`/v1/sub2api/billing`），不能查渠道状态。

## Confirmed Facts

- 必须用户 API Key 鉴权，不允许匿名。
- 必须独立限流防刷。
- 只读；复用现有渠道监控数据，不新增探测。
- 不返回探测凭据、上游账号、配额快照、原始错误正文、请求量绝对值。
- 返回该用户所有可见分组，不只是当前 API Key 绑定分组。可见分组与 `GetAvailableGroups` 一致（公开非专属、用户被允许的标准分组、有效订阅分组）。
- 当前 Key 绑定分组必须特别标识，方便脚本直接判断「这把 Key 对应的渠道」是否可用。
- 不写轮询或 Codex 重试脚本；接口把分组身份和是否可用传清楚即可。
- 工作分支：`feature/channel-status-script-api`，合入 `custom/main`。

## Requirements

### R1 鉴权

- 使用用户 API Key（`Authorization: Bearer` / `x-api-key`），与网关现有提取方式一致。
- 拒绝 query 传 key。
- 无 Key / 错误 Key / 停用 Key / 停用用户：401。
- 只做身份校验，不做计费拦截：额度耗尽或 Key 过期仍允许查询（与 `/v1/usage`、`/v1/sub2api/billing` 同类只读自省）。
- 当前 Key 所属分组停用/删除时，仍允许查询该用户其他可见分组（本接口看的是用户可见范围，不是这把 Key 能不能发对话）。
- 仍执行 IP 黑白名单。

### R2 接口形态

- 新增脚本专用只读接口：`GET /v1/sub2api/channel-status`。不改造现有面板 JWT 接口。
- 与 Codex 共用同一 `base_url` 和 API Key。
- 响应跟随 `/v1/sub2api/billing`：无面板 `{success,data}` 信封；错误用网关 OpenAI 风格 `{error:{type,message}}`。
- 覆盖当前监控模式（V1 或 V2），调用方只调这一条路径。
- 响应字段：
  - `object`：`sub2api.channel_status`
  - `schema_version`：`1`
  - `mode`：`v1` / `v2` / `off`
  - `connected`：全部 `items` 均可用且 `item_count>0` 时为 `true`（总览全绿）
  - `key_group_id` / `key_group_name`：当前 API Key 绑定分组；Key 未绑分组时省略
  - `key_group_connected`：该绑定分组是否可用；无绑定或 `items` 中找不到对应条目时为 `null`
  - `key_group_status`：绑定分组的 `status`；找不到时省略
  - `checked_at`：RFC3339 UTC
  - `item_count` / `connected_count`
  - `items[]`：该用户全部可见分组的精简条目
- 单条 item：`group_id`（V1 无稳定 ID 时可省略）、`group_name`、`name`、`provider`、`status`、`connected`、`is_key_group`、`error_category`（无可省略）。
- `is_key_group=true` 表示该条目属于当前 Key 绑定分组。同一分组多条监控时，匹配到的条目都标 true。
- 功能关闭或没有匹配监控：HTTP 200，`mode=off` 或当前模式，`connected=false`，`items=[]`。不把「没数据」伪装成已连上。绑定分组信息（若 Key 有分组）仍返回，`key_group_connected=null`。

### R3 可见范围与条目可用性

- 不按当前 API Key 绑定分组裁剪。
- V1 面板目前列出全部 enabled 监控；本接口仍按用户可见分组名过滤，避免把用户不能用的分组状态交给脚本。
- 单条 `items[].connected`：
  - V1：`primary_status=operational`
  - V2：`health.overall` 为 `healthy` 或 `warning`（`critical` / `unknown` 视为未连上）
  - `error_category=rate_or_capacity` 视为未连上
- V2 用最近 90 分钟、按 `platform_group` 聚合的健康度，不返回趋势桶和请求量。

### R4 限流

- 该接口独立计数，不占用网关对话 RPM，也不占用面板 `UserRPM` / `HeavyRPM`。
- 按用户 ID 固定窗口：**12 次/分钟**（同一用户多把 Key 共用额度）。
- 超限 429，`Retry-After`，网关错误 `type=rate_limit_error`。
- Redis 异常 fail-open。
- 管理员不豁免。MVP 硬编码阈值，不新增后台配置项。

### R5 文档

- 在接口注释和现有用户向文档中写明路径、鉴权头、字段含义。
- 允许一行 curl 作为契约示例。
- 不提供轮询循环、Codex resume 示例或官方脚本。

## Acceptance Criteria

- [ ] AC1 有效 API Key 可调用 `GET /v1/sub2api/channel-status`，返回 R2 所列字段；`items` 覆盖该用户全部可见分组。
- [ ] AC2 无 Key、错误 Key、query 传 key 均不可用；停用 Key 返回 401。
- [ ] AC3 额度耗尽或过期的 Key 仍可查询状态。
- [ ] AC4 当前 Key 所属分组停用时，仍返回该用户其他可见分组状态。
- [ ] AC5 响应不含 API Key、账号、配额、上游 endpoint、原始错误正文、请求量绝对值。
- [ ] AC6 同一用户超过 12 RPM 返回 429 + `Retry-After`；对话请求 RPM 不受影响。
- [ ] AC7 监控关闭或无匹配项时 200 且 `connected=false`、`items=[]`。
- [ ] AC8 V1/V2 任一模式下脚本只调这一条路径即可得到完整 `items`。
- [ ] AC9 用户不可见分组不会出现在 `items` 中；可见分组即使当前 Key 未绑定也会返回。
- [ ] AC10 顶层 `connected` 仅在全部条目可用且条数大于 0 时为 true；部分分组异常时为 false，但异常分组仍出现在 `items` 里且 `connected=false`。
- [ ] AC11 Key 已绑定分组时：`key_group_id` / `key_group_name` 有值；对应 `items` 的 `is_key_group=true`；`key_group_connected` 与这些条目的可用性一致。
- [ ] AC12 Key 未绑定分组时：不出现 `key_group_id` / `key_group_name`，所有 `is_key_group=false`，`key_group_connected` 为 `null`。
- [ ] AC13 有回归测试覆盖鉴权、分组停用仍可查、限流、脱敏、空数据、V1/V2 映射、可见范围过滤、Key 绑定分组标识。

## Out of Scope

- 轮询脚本、Codex / CLI 自动重试、webhook / SSE
- 匿名公开状态页
- 给现有面板 JWT 接口加 API Key
- 管理员配置限流阈值
- 返回可用率、延迟、排行榜、智力检测、V2 趋势桶
