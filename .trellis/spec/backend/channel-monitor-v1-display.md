# V1 渠道监控展示契约

## 1. 范围与触发

适用于 V1 卡片、关联分组和用户列表展示指标。V2 页面不受本契约的界面部分影响。

## 2. 接口与数据库签名

- `GET /api/v1/channel-monitors`：当前认证用户的监控列表。
- `POST /api/v1/admin/channel-monitors`、`PUT /api/v1/admin/channel-monitors/:id`：新增/编辑关联分组。
- `POST /api/v1/admin/channel-monitors/:id/duplicate`：复制时保留关联分组。
- `channel_monitors.group_id BIGINT NULL REFERENCES groups(id) ON DELETE SET NULL`。
- `ChannelMonitorDisplayService.Enrich(ctx context.Context, userID int64, views []*UserMonitorView)`：原地补充可选展示字段，查询失败不影响基础探测响应。
- `ChannelMonitorDisplayRepository.BatchMetrics(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error)`：按分组/模型批量统计。

## 3. 数据与展示契约

### 关联分组

- `channel_monitors.group_id` 是可空的稳定 ID；`group_name` 仍是原展示标签，不能用于推断统计分组。
- 迁移 `246_channel_monitor_group_id.sql` 为存量监控保留 NULL；管理员补选。分组硬删除通过 FK `ON DELETE SET NULL` 解绑监控。
- 新增接受正数或 null/省略；更新正数绑定、0 解绑、null/省略保持原值。绑定前确认分组存在；复制保留分组并生成独立监控。
- 管理端选择本站 API Key 时带入其 `group_id`，管理员仍可改选。

### 用户接口和权限

`GET /api/v1/channel-monitors` 保留原字段，追加以下可空字段，未知值必须保留 JSON null：

| 字段 | 类型 | 口径 |
| --- | --- | --- |
| `group_id` | integer | 稳定关联 ID |
| `primary_checked_at` | RFC3339 timestamp | 主模型最近探测时间，不能用监控或附加模型时间替代 |
| `visible_rate_multiplier` | number | 当前用户最终可见倍率 |
| `cache_rate_24h` | number | 0–1 的缓存读取 Token 比例 |
| `output_tps_5m` | number | 最近 5 分钟逐请求输出速度平均值 |

身份来自认证 middleware，不能采信 query user_id。每次请求调用 `APIKeyService.GetAvailableGroups` 校验关联分组权限，并复用其专属可见倍率和分时策略结果；缓存不能复用其他用户的权限或倍率。

### 统计与降级

- 独立 `ChannelMonitorDisplayService` / repository 批量读取 `usage_logs`，范围为关联分组 + 主模型，包含该范围所有用户。
- 模型归一化复用 `channelMonitorV2ModelSQL`：非空 trimmed requested_model 优先，再回退 trimmed model 和 unknown。
- 缓存率：最近 `[now-24h, now)` 内 `SUM(cache_read_tokens) / SUM(input_tokens + cache_creation_tokens + cache_read_tokens)`；无有效分母为 null，真实零为 0。
- TPS：最近 `[now-5m, now)` 内 `AVG(output_tokens * 1000.0 / duration_ms)`，使用请求完整耗时；有效样本通过与运维共用的 `usageOutputTPSValidSQL` 过滤。正输出、正耗时，排除图像；仅 sync/stream/ws_v2/cyber_blocked 类型。无有效样本为 null。
- SQL 使用分组与时间条件配合现有 group/created_at 索引，不增加 usage log 索引或改变运维口径。
- 聚合结果按分组/模型缓存 30 秒。无样本结果也缓存；查询失败不缓存。相同及重叠批次按 key 共享在途查询，未在途 key 合为批量 SQL。
- 每条统计查询最多 2 秒；发起请求取消不取消其他等待者的共享查询。权限查询失败不提供新增倍率/统计，统计失败保留已解析倍率和基础探测结果。
- 未关联、无权限或分组不可用时，倍率和统计为 null。pure quota 不查询模型统计。
- TPS 属于请求体验，独立于 RPM/TPM 规模信息和配额展示开关。

### 卡片

- `MonitorCard.vue` 展示平台、名称、主模型、倍率、状态、主模型相对探测时间、三列指标、TPS 与最近 60 次探测。整卡 button 打开原详情。
- 7/15/30 日可用率继续使用原主动探测结果；状态条从左到右为旧到新，缺少历史时左侧补位，限流保留黄色语义。
- 未知值显示 `—`，真实零不能变成未知。响应时间自动切换 ms/s，Ping 留在提示中。
- 保留原自动刷新、配额展示和详情入口；新增文案在 zh/en custom overlay。
- 卡片与加载骨架不显示外边框；使用轻柔阴影区分层次，深浅主题及悬停分别调整强度。内部指标分隔线和键盘焦点提示保留。
- 容器内指标和超长倍率允许省略，完整数值保留在提示中；深色 scoped 样式使用完整 global selector，避免背景变量仅落到根节点。

## 4. 校验与错误矩阵

| 条件 | 结果 |
| --- | --- |
| 新增 group_id ≤ 0，或更新 group_id < 0 | HTTP 400，拒绝写入 |
| 绑定的分组不存在 | HTTP 404 / GROUP_NOT_FOUND |
| 更新 group_id = 0 | 解除关联；null/省略则保留原关联 |
| 用户未认证 | 路由认证中间件拒绝请求 |
| 分组未关联、不可用或当前用户无权限 | 新倍率/统计为 null；保留探测结果 |
| 分组权限查询失败 | 新倍率/统计为 null；列表仍成功返回 |
| 统计超时或失败 | 保留已解析倍率，缓存率/TPS 为 null；失败不缓存 |
| 缓存率无有效分母，或 TPS 无有效样本 | 对应指标为 null；有效的 0 必须返回 0 |
| 纯配额监控 | 不查询模型统计；沿用配额与历史展示 |

## 5. 正常、基线与异常用例

- 正常：主模型 A 关联分组 1；只汇总分组 1 / A 的所有用户用量，倍率仍按当前用户计算。
- 基线：存量监控未选分组；响应时间、可用率和状态条继续显示，新指标显示 `—`。
- 异常：用户切换或分组权限撤销；即使统计缓存命中，也必须重新校验权限，不能泄漏倍率或聚合结果。

## 6. 必需测试与断言

- `backend/internal/repository/channel_monitor_display_integration_test.go`：真实 PostgreSQL 分组关联生命周期、模型与时间过滤、缓存比例、逐请求 TPS 和有效样本过滤。
- `backend/internal/service/channel_monitor_display_test.go`：权限与倍率规则、缓存过期/无样本/失败、重叠并发合并和调用者取消。
- `backend/internal/handler/channel_monitor_user_display_test.go`：nullable JSON、认证身份、配额与 TPS 独立性。
- `frontend/src/components/user/__tests__/MonitorCard.display.spec.ts`：卡片指标、缺失/零值、中英文、相对时间、窗口切换、状态条及详情。
- `frontend/src/components/admin/monitor/__tests__/MonitorFormDialog.group.spec.ts`：新增带入、编辑绑定/解绑和分组查询失败。

## 7. 错误与正确实现示例

错误：按 `group_name` 聚合，或把个人倍率放入共享的分组/模型缓存。这会混淆同名分组并跨用户复用权限结果。

正确：统计键只包含稳定 ID 与归一化模型，权限与个人倍率逐请求解析。

```go
key := ChannelMonitorMetricKey{GroupID: *view.GroupID, Model: strings.TrimSpace(view.PrimaryModel)}
// 仅在 GetAvailableGroups 确认当前用户可访问后读取 key 的统计。
```
