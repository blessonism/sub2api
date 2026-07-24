# 无原始Token消费校准 + 错误透出

## Boundary

- 改动集中在消费校准事务路径与管理端校准弹窗错误展示。
- 不改 Token 校准无用量拒绝、正向余额不计消费、幂等与鉴权。
- 统计查询继续只认日分摊 `balance_delta`（有分摊）或主表 `created_at`（无分摊）；本任务保证无 Token 消费校准**始终写日分摊**，避免再落成「操作日记账」。

## Data Flow

### 有原始 Token（不变）

1. `queryOriginalDailyTokens` → `originalTokens > 0`
2. `before = originalCost + existingCalibrationSpend`
3. `calculateConsumptionCalibration` → `delta / after / walletAfter`
4. `allocateBalanceDelta(rows, originalTokens, walletDelta)` 按日分摊
5. INSERT 主表 + 日分摊；UPDATE `users.balance`

### 无原始 Token（新增）

1. `queryOriginalDailyTokens` → `originalTokens == 0` **不再 return NO_ORIGINAL_USAGE**
2. 仍算 `before`（通常 = 已有校准消费差额，原始 cost 多为 0）
3. `calculateConsumptionCalibration` 不变
4. 不调用按比例分摊；构造单行：

```text
allocation_date = consumption.start_date
original_tokens = 0
token_delta     = 0
balance_delta   = walletDelta   // = -consumption_delta
```

5. INSERT 主表 + 该单行；UPDATE 余额

### 前端错误

```text
createCalibration 失败
  → extractApiErrorMessage(error)  // 复用 utils/apiError.ts
  → 有 message 则 toast message
  → 否则 t('usage.adminCalibrationFailed')
```

## Contracts

| 项 | 约定 |
|----|------|
| 无 Token 归属日 | `consumption.start_date` |
| 单行分摊精度 | 与主表 `balance_delta` 完全一致（无舍入） |
| 有 Token | 现有最大余数法，6 位小数 |
| 错误码 | 消费无 Token **不再**发 `NO_ORIGINAL_USAGE`；Token 校准无 Token 仍发 |
| API 请求体 | 无变更 |
| 日分摊 `original_tokens >= 0` | 已有 CHECK 允许 0 |

## Compatibility

- 旧客户端可继续提交；仅服务端放宽无 Token 消费。
- 迁移：无新列；依赖已有 `197` 的 `balance_delta` / `consumption_*`。
- 回滚应用：恢复「无 Token 拒绝」即可；已写入的 `original_tokens=0` 分摊行仍可被 `SumBalanceSpent` 正确汇总。

## Risks

| 风险 | 缓解 |
|------|------|
| 管理员误把大额消费挂到 start_date | UI 轻提示 + 审计字段保留范围 |
| 同时勾选 Token 仍失败 | 错误透出明确；文档/提示引导取消 Token |
| `original_tokens=0` 被某些汇总忽略 | 核对 `SumBalanceSpent` 仅看 `balance_delta IS NOT NULL`，不依赖 tokens>0 |

## Rejected

- 归属到「操作日 / end_date」：与管理员所选范围起始不一致，难解释。
- 自动剥离 Token 字段：隐式改请求，审计难。
- 按历史校准单 ID 冲销：范围更大，Out of Scope。
