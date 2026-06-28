# 上游中继监控与推荐策略控制台设计

## 1. 核心理解

本任务不是单纯做一个“推荐按钮”，而是要把上游中继监控做成一个可控的决策控制台：

- 系统负责持续采集数据、发现变化、计算建议。
- 策略负责决定哪些候选可用、如何排序、建议怎样调整 priority。
- 管理员负责最终判断和应用，任何会修改 priority 的动作都必须有明确确认和审计记录。

一句话原则：

> 自动化负责发现和计算，最终控制权留给管理员。

## 2. 概念分层

### 2.1 数据采集层

负责把上游数据“养新”，不直接生成正式建议，也不修改 priority。

数据来源包括：

- 上游分组倍率：连接器同步时从上游拉取 group 倍率，形成倍率快照。
- 候选探测结果：对候选绑定账号发起轻量请求，记录成功/失败、延迟、错误类型。
- 上游账号余额：连接器同步时读取上游账号余额。
- 上游今日用量：按连接器 + 上游 group 维度聚合今日实际消耗和 token。
- 用量推导倍率：当登录倍率不可用时，用成本差值推导倍率。

### 2.2 数据有效性层

负责判断已有数据还能不能用于推荐。

关键设计调整：

- 页面不要把“倍率快照新鲜度 / 探测新鲜度”作为主要配置项暴露给管理员。
- 管理员真正关心的是“多久自动同步一次 / 多久自动探测一次”。
- 新鲜度应由系统根据自动间隔派生，作为内部判断或高级说明。

建议默认派生规则：

- 倍率数据过期阈值 = 自动同步上游倍率间隔 × 3。
- 探测数据过期阈值 = 自动探测候选间隔 × 3。
- 用量推导倍率过期阈值 = 自动同步/采样间隔 × 3。

示例：

- 自动同步倍率间隔为 60 分钟，则超过 180 分钟未成功同步才视为倍率过期。
- 自动探测候选间隔为 30 分钟，则超过 90 分钟未成功探测才视为探测过期。

### 2.3 推荐策略层

负责根据当前数据判断候选是否进入建议，以及如何排序。

推荐策略只消费数据，不主动刷新数据。

策略项包括：

- 最小成功率。
- 最小样本数。
- 是否排除连续失败候选。
- priority 起点。
- priority 步长。
- 排序顺序：倍率低优先、成功率高优先、p95 延迟低优先。

### 2.4 人工决策层

负责保留管理员最终控制权。

手动操作包括：

- 立即同步某个连接器的上游倍率、余额、用量。
- 立即同步全部连接器。
- 立即探测单个候选。
- 立即探测全部启用候选。
- 启用/禁用候选。
- 修改候选绑定关系、探测模型、目标分组。
- 查看即时预览。
- 生成正式建议记录。
- 手动应用建议。

核心规则：

- 手动 priority 永远优先于自动建议。
- 系统可以提示“当前 priority 与建议不一致”，但不能静默覆盖。
- 自动监控可以自动跑，推荐策略可以自动算，但应用变更必须由管理员确认。

## 3. 页面信息架构

建议把现有“策略预览”拆成三个区域。

### 3.1 自动监控

面向“系统多久主动刷新数据”。

字段建议：

- 启用自动同步上游倍率。
- 上游倍率同步间隔，单位分钟。
- 启用自动探测候选。
- 候选探测间隔，单位分钟。
- 失败重试间隔，单位分钟。
- 最大同步并发。
- 最大探测并发。
- 立即同步全部连接器。
- 立即探测全部启用候选。

展示建议：

- 最近一次自动同步时间。
- 最近一次自动探测时间。
- 下一次计划同步时间。
- 下一次计划探测时间。
- 最近失败原因。
- 正在运行的任务状态。

### 3.2 推荐策略

面向“系统怎么判断和排序”。

字段建议：

- 最小成功率。
- 最小样本数。
- 排除连续失败候选。
- priority 起点。
- priority 步长。
- 排序顺序。

高级说明：

- 倍率数据过期阈值由同步间隔派生。
- 探测数据过期阈值由探测间隔派生。
- 这些阈值用于排除过期数据，避免基于旧状态生成建议。

### 3.3 即时预览

面向“当前配置会产生什么结果”。

要求：

- 基于当前页面配置即时试算。
- 不保存 recommendation run。
- 不写 recommendation suggestions。
- 不修改 account group priority。
- 结果区必须有明显反馈：计算中、更新时间、建议数量、排除数量。
- 预览成功后自动定位到结果区，避免用户以为点击无效。

结果分组：

- 建议项：候选、目标组、当前 priority、建议 priority、倍率、成功率/延迟摘要、原因。
- 排除项：候选、排除原因、缺失或过期的数据、下一步建议。

### 3.4 建议历史

面向“正式留痕和应用”。

流程：

1. 管理员点击“生成正式建议”。
2. 系统按保存后的推荐策略创建 recommendation run。
3. 管理员查看 run 详情。
4. 管理员手动确认应用。
5. 系统批量更新 priority，并写入应用审计。

要求：

- 即时预览与正式建议历史必须分离。
- 只有正式建议 run 可以应用。
- 已应用 run 不允许重复应用。
- 应用时必须校验旧 priority 是否仍匹配，避免覆盖其他管理员刚刚做的手动调整。

## 4. 后端设计建议

### 4.1 配置模型

建议拆成两张单例配置表或一个配置表的两个 JSON/字段组。

#### 自动监控配置

建议表名：

- `upstream_relay_monitoring_policy`

字段建议：

- `id = 1`
- `auto_sync_enabled`
- `sync_interval_minutes`
- `auto_probe_enabled`
- `probe_interval_minutes`
- `failure_retry_interval_minutes`
- `sync_concurrency`
- `probe_concurrency`
- `updated_by`
- `created_at`
- `updated_at`

派生值不一定入库：

- `snapshot_stale_after_minutes = sync_interval_minutes * 3`
- `probe_stale_after_minutes = probe_interval_minutes * 3`

#### 推荐策略配置

现有策略表可以继续承载推荐策略：

- `upstream_relay_recommendation_policy`

字段保留：

- `min_success_rate`
- `min_sample_size`
- `exclude_consecutive_failures`
- `priority_start`
- `priority_step`
- `sort_fields`

建议调整：

- 不再把 `snapshot_freshness_minutes`、`usage_delta_freshness_minutes`、`probe_freshness_minutes` 作为主配置项展示。
- 后端可以暂时保留字段用于内部兼容，但实际值应由自动监控配置派生，或作为高级配置隐藏。

### 4.2 API 设计

自动监控配置：

- `GET /admin/upstream-relay-group-monitors/monitoring-policy`
- `PUT /admin/upstream-relay-group-monitors/monitoring-policy`

手动刷新：

- `POST /admin/upstream-relay-group-monitors/connectors/:id/sync`
- `POST /admin/upstream-relay-group-monitors/connectors/sync-all`
- `POST /admin/upstream-relay-group-monitors/candidates/:id/probe`
- `POST /admin/upstream-relay-group-monitors/candidates/probe-all`

推荐策略：

- `GET /admin/upstream-relay-group-monitors/recommendation-policy`
- `PUT /admin/upstream-relay-group-monitors/recommendation-policy`
- `POST /admin/upstream-relay-group-monitors/recommendations/preview`
- `POST /admin/upstream-relay-group-monitors/recommendations`
- `GET /admin/upstream-relay-group-monitors/recommendations`
- `GET /admin/upstream-relay-group-monitors/recommendations/:id`
- `POST /admin/upstream-relay-group-monitors/recommendations/:id/apply`

### 4.3 调度设计

自动监控需要后台调度，但必须保守实现。

建议能力：

- 每个任务类型独立调度：同步倍率、探测候选。
- 每个连接器/候选有独立的最近运行时间。
- 支持失败重试间隔。
- 支持全局并发上限。
- 支持运行中锁，避免重复执行。

调度触发方式可以分阶段：

1. 先实现配置和手动触发。
2. 再实现后台自动调度。
3. 最后补运行历史和失败重试可视化。

### 4.4 审计和安全

必须记录：

- 谁更新了自动监控配置。
- 谁更新了推荐策略配置。
- 谁生成了正式建议 run。
- 谁应用了建议 run。
- 每条建议应用前后的 priority。

禁止行为：

- 自动监控任务直接修改 priority。
- 即时预览创建 recommendation run。
- 即时预览写 suggestion 表。
- 预览或自动任务静默覆盖手动 priority。

## 5. 前端设计建议

### 5.1 控制台布局

建议 tabs：

- 候选映射
- 连接器
- 自动监控
- 推荐策略
- 即时预览
- 建议历史
- 快照变化

如果 tabs 太多，可以把“推荐策略”和“即时预览”合并为一个页签，但视觉上必须分区。

### 5.2 交互反馈

所有可能“点了没感觉”的操作都必须有反馈：

- 按钮 loading 文案。
- 禁用态。
- 成功更新时间。
- 失败错误提示。
- 操作后自动定位到结果区域。

尤其是即时预览：

- 点击后显示“正在按当前策略计算建议预览”。
- 成功后显示“预览已更新：时间”。
- 自动滚动到结果摘要。

### 5.3 手动优先提示

建议在建议项中标记：

- 当前 priority 是否与建议一致。
- 当前 priority 是否疑似刚被手动调整。
- 应用时是否可能冲突。

当应用发生冲突：

- 不覆盖。
- 展示冲突候选。
- 提示重新生成建议或手动处理。

## 6. 预览排除原因规范

排除项必须可解释，至少覆盖：

- 候选未启用。
- 连接器不是 active。
- 缺少有效倍率。
- 倍率数据已过期。
- 最近探测失败。
- 探测数据已过期。
- 连续失败。
- 成功率低于阈值。
- 当前 priority 已等于建议值。

排除原因不仅要有 `reason_code`，还要有人能看懂的 `reason`。

## 7. 分阶段落地计划

### Phase 1：当前任务收敛

目标：

- 保留现有全局推荐策略和即时预览。
- 改善文案，把“倍率快照新鲜度”从主概念中降级。
- 明确文档：真正的目标是自动监控配置 + 推荐策略配置。

不做：

- 不实现后台自动调度。
- 不实现自动应用。

### Phase 2：自动监控配置与手动批量操作

目标：

- 新增自动监控配置模型和 UI。
- 新增批量同步全部连接器。
- 新增批量探测全部启用候选。
- 推荐策略的新鲜度判断改为从自动监控间隔派生。

验收：

- 管理员能配置同步/探测间隔。
- 手动同步/探测有明确反馈。
- 即时预览使用派生的新鲜度判断。

### Phase 3：后台自动调度

目标：

- 按配置自动同步倍率。
- 按配置自动探测候选。
- 支持失败重试、并发限制、运行锁。

验收：

- 自动任务不会重复运行。
- 自动任务失败不会阻塞手动操作。
- 页面能看到最近运行和下一次运行时间。

### Phase 4：冲突保护与审计增强

目标：

- 应用建议时校验旧 priority。
- 手动改动与建议应用冲突时阻止覆盖。
- 完善审计展示。

验收：

- 手动 priority 不会被旧建议覆盖。
- 冲突可见、可重试、可重新生成。

## 8. 当前任务实现边界

当前任务可以继续完成：

- 全局推荐策略配置。
- 即时预览。
- 正式建议 run 与手动应用。
- 预览交互反馈。

下一轮任务再拆：

- 自动监控配置。
- 批量手动同步/探测。
- 后台自动调度。
- 新鲜度由自动间隔派生。

## 9. 开发注意事项

- 不要把“自动监控”和“推荐策略”继续混成一张表单。
- 不要让管理员直接面对“新鲜度”这种内部判断作为主配置。
- 不要让即时预览产生审计历史。
- 不要让自动任务修改 priority。
- 不要让应用建议绕过旧值校验。
- 不要覆盖现有并行任务的改动。
