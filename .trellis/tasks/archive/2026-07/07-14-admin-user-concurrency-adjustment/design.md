# 管理员一键调整用户并发 - 技术设计

## 边界

- 前端入口位于管理员用户列表工具栏，复用现有 `ConfirmDialog`，在插槽中放置原生数字输入框。
- 前端 API 继续归属 `frontend/src/api/admin/users.ts`。
- 后端复用 `POST /api/v1/admin/users/batch-concurrency`，为 `mode` 增加 `floor`，不新增路由或数据库字段。
- 用户范围沿用该接口 `all=true` 的现有逻辑：分页枚举全部未软删除用户，不附加角色或状态过滤。

## API 合约

请求：

```json
{
  "all": true,
  "concurrency": 5,
  "mode": "floor"
}
```

响应沿用现有结构：`{"affected": <实际提升数量>}`。

- `floor` 模式要求 `concurrency >= 1`。
- `set` 和 `add` 模式保持原语义。
- 无目标用户或所有用户均已达到下限时返回 `affected: 0`。

## 数据流

1. 管理员打开“统一并发下限”确认弹窗并输入整数。
2. 前端完成即时校验后调用批量并发接口，固定发送 `all=true`、`mode=floor`。
3. Handler 沿用现有分页逻辑收集未删除用户 ID。
4. Service 清理非法 ID，并调用 Repository 的并发下限批量更新方法。
5. Repository 用单条原子 SQL 执行 `concurrency = GREATEST(concurrency, target)`，并仅匹配低于目标值的行。
6. Service 沿用现有认证缓存失效流程；前端成功后刷新当前用户列表。

## 正确性与并发

- 只升不降由数据库 `GREATEST` 原子表达，不采用“先读后写”，因此与同时发生的单用户提高操作并发时不会把更高值覆盖回目标下限。
- 软删除过滤继续由用户枚举和更新 SQL 双重约束。
- 前后端均校验正整数，后端为权威边界。

## 取舍

- 不新增独立弹窗组件，现有 `ConfirmDialog` 已能承载说明和输入框。
- 不改变单用户编辑、`set`、`add` 或调整记录逻辑；本功能只增加 `floor` 模式。
- 不增加数据库迁移、后台任务或进度轮询；现有批量接口同步返回即可满足本次范围。

## 回滚

- 前端可移除工具栏入口和 API 包装。
- 后端可移除 `floor` 分支及 Repository 方法；无数据结构变更需要回滚。
- 已提高的用户并发不会自动降低，符合本功能“只升不降”的业务语义。
