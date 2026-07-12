# 技术设计：抽奖活动美元用量口径

## 1. 数据流与单位

### Token 模式

`usage_logs` 四类 Token 求和 -> `int64` 原始 Token -> 门槛/阶梯计算 -> `lottery_entries.tokens` 快照 -> 用户进度与开奖权重。

### 美元模式

`SUM(usage_logs.actual_cost)`（PostgreSQL `NUMERIC`）-> `FLOOR(sum * 1_000_000)::BIGINT` 微美元 -> 门槛/阶梯计算 -> `lottery_entries.cost_microusd` 快照 -> 用户进度与开奖权重。

管理界面以美元输入且最多 4 位小数，API 与后端统一传输整数微美元。所有资格比较只使用整数，避免 Go/JavaScript 浮点边界误差。聚合时向下取整到微美元，不能因四舍五入让未真正达到门槛的用户获得资格。

## 2. 数据库变更

新增迁移 `190_lottery_campaign_usd_usage.sql`：

- `lottery_campaigns.usage_mode VARCHAR(16) NOT NULL DEFAULT 'token'`，限定为 `token|usd`。
- `lottery_campaigns.threshold_cost_microusd BIGINT NOT NULL DEFAULT 0`。
- `lottery_campaigns.entry_step_cost_microusd BIGINT NOT NULL DEFAULT 0`。
- `lottery_entries.cost_microusd BIGINT NOT NULL DEFAULT 0`。
- 调整活动金额约束：当前口径的门槛必须大于 0；阶梯模式对应步长由服务层与数据库共同校验；非当前口径字段允许为 0。

默认值保证历史活动原样进入 `token` 模式，无需数据回填或重算历史资格。

## 3. 后端契约

### 活动配置

`LotteryCampaign`、`LotteryCampaignInput`、管理员请求增加：

- `usage_mode`
- `threshold_cost_microusd`
- `entry_step_cost_microusd`

历史客户端未传 `usage_mode` 时，后端归一化为 `token`，保留兼容性。Token 模式忽略并归零美元字段；美元模式忽略并归零 Token 字段。

新增业务错误 `LOTTERY_USAGE_MODE_LOCKED`。更新活动时若用量口径发生变化，仓储层在事务中确认不存在 `lottery_draw_batches`；存在任意批次则拒绝切换。这样可避免检查与更新之间出现并发开奖竞态。

### 用量查询

将仓储的 Token 专用查询提升为口径化用量结构，同时返回 `Tokens` 与 `CostMicrousd`。SQL 根据经过枚举校验的活动口径选择固定查询，不拼接外部输入：

- Token：四类 Token 求和，且仅统计 `actual_cost > 0`。
- 美元：对所有 `actual_cost > 0` 求和，包括图片和按次计费请求。

用户查询、资格同步、达标人数统计都复用同一口径选择函数和同一时间窗口。

### 资格与用户响应

`lottery_entries` 同时保留 `tokens` 与 `cost_microusd`，当前口径写入真实值，另一口径写入 0。已有 `entry_count` 仍是开奖权重的唯一来源，因此中奖选择算法无需分叉。

用户响应保留 `today_tokens`、`threshold_tokens`，并新增：

- `today_cost_microusd`
- `threshold_cost_microusd`

前端根据 `campaign.usage_mode` 选择展示字段。候选人管理响应同样新增 `cost_microusd`，保留 `tokens` 兼容旧客户端。

## 4. 前端交互

- 管理表单增加 Token/美元分段选择控件。
- Token 模式继续使用 M Token 输入；美元模式使用带 `$` 单位的数字输入，步长 `0.0001`。
- 前端把美元字符串严格转换为微美元整数，禁止超过 4 位小数、非有限值和非正门槛。
- 活动列表标签、详情指标、规则预览、用户进度和候选人用量按口径切换格式。
- 已有开奖批次时，后端负责最终锁定校验；前端收到 `LOTTERY_USAGE_MODE_LOCKED` 后展示明确错误，不依赖本地状态猜测。
- zh/en 文案同步放入下游 locale overlay。

## 5. 兼容、发布与回滚

- 迁移先于新应用启动执行，新增列均有非空默认值。
- 旧活动、旧资格及旧 API 字段继续有效。
- 回滚应用版本时旧代码会忽略新增列；不要回滚数据库迁移，避免破坏已创建的美元活动数据。
- 本功能不修改余额扣减、活动时间窗口、开奖随机算法或奖金发放。

## 6. 风险控制

- 精度：全链路整数微美元；聚合向下取整。
- 口径漂移：统一仓储用量结构与服务计算入口，不分别实现用户进度、同步和人数统计。
- 历史审计：产生开奖批次后锁定口径。
- 并发：口径切换检查与活动更新在同一数据库事务内完成。
- JavaScript 安全整数：管理员输入转换后必须通过 `Number.isSafeInteger`；后端同时验证上限与正数。
