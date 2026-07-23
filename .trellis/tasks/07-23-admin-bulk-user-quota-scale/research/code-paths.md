# 额度对象与现有代码链路

## 结论

“用户额度”在当前仓库不是唯一字段。最贴近中转站计费的是用户钱包余额 `users.balance`；Token 用量校准和 `user_platform_quota` 是另外两套机制。

## 钱包余额

- 数据模型：`backend/ent/schema/user.go` 的 `balance` 字段使用 PostgreSQL `decimal(20,8)`，默认值为 0。
- 管理员接口：`backend/internal/server/routes/admin.go` 注册 `POST /users/:id/balance`。
- Handler：`backend/internal/handler/admin/user_handler.go` 的 `UpdateBalance` 支持 `set`、`add`、`subtract`，并使用管理员幂等 JSON 执行包装。
- Service：`backend/internal/service/admin_user.go` 的 `UpdateUserBalance` 先读取用户再计算，拒绝负余额；余额变化时失效认证缓存和计费余额缓存，并写入 `admin_balance` 调整记录。
- 前端：`frontend/src/views/admin/UsersView.vue` 的余额列提供余额历史、充值和扣减入口；`frontend/src/api/admin/users.ts` 已封装 `updateBalance`。

## 其它额度

- `frontend/src/views/user/UsageView.vue` 的管理员校准同时处理 Token 用量和钱包余额，但它是单用户、带日期窗口的校准记录，不是全量倍率调整。
- `backend/ent/schema/user_platform_quota.go` / 相关管理员接口处理按平台的窗口额度，语义是限额而非钱包余额。

## 设计影响

如果目标是钱包余额，批量实现应复用或抽取现有余额更新的审计、缓存和幂等语义；直接执行裸 SQL 会绕过历史记录、缓存失效和并发扣款保护。因子重算还需要在数据库层以原子表达式处理，避免先读后写覆盖并发扣款。

## 事务与审计证据

- `backend/internal/service/admin_user.go` 的 `GrantUserBalances` 已提供多用户余额变更的事务范式：余额更新和每用户 `admin_balance` 调整记录同事务提交，失败时整体回滚，提交后统一失效认证与计费缓存。
- 该方法当前按输入用户逐个读取和更新，适合已知小批量奖励；“全部未软删除用户”不应先分页读出再逐个调用单用户接口，否则会产生长事务、部分失败或并发覆盖风险。
- 产品语义已确认是输入 `x > 1`，最小可靠实现应让数据库以 `balance = balance / x` 的表达式原子更新，并返回每个用户的修改前/后余额，用于同一事务写入逐用户审计记录。
- 管理员写接口已有 `executeAdminIdempotentJSON`，新接口应复用该机制，防止浏览器重试或重复点击再次缩减。
