# 技术设计

## 数据模型

在 `admin_usage_calibrations` 增加可空的 `revoked_at TIMESTAMPTZ` 与 `revoked_by BIGINT REFERENCES users(id)`，并为未撤销查询增加索引。历史记录采用软撤销，保留原始校准字段和每日分摊，便于审计与回溯。

## 撤销流程

新增 `POST /api/v1/admin/usage/calibrations/:id/revoke`。服务层校验仓储可用性并委托事务；仓储事务锁定校准记录和目标用户，拒绝不存在或已撤销记录，按原 `balance_delta` 反向更新用户余额，写入撤销字段并返回完整记录。若恢复后余额为负则拒绝整个事务。余额变化后复用现有认证/计费缓存失效逻辑。

## 统计边界

- `UsageService` 与 `DashboardService` 的使用记录/仪表盘路径停止调用或加总 `SumBalanceSpent`。
- 仓储中仍用于余额消费、Token 分摊、排行榜和用户排序的校准 SQL 统一追加 `revoked_at IS NULL`。
- Token 校准记录和余额校准记录仍可在管理员校准历史中展示；撤销状态由 DTO 返回给前端。

## 前端交互

校准历史每条未撤销记录显示撤销按钮，点击后使用原生确认；成功后刷新历史、使用统计和用户详情，失败显示 API 错误。已撤销记录展示状态、撤销时间和撤销管理员信息（若后端返回），按钮禁用。

## 兼容与回滚

迁移使用 `ADD COLUMN IF NOT EXISTS` 和索引创建，旧数据默认未撤销。代码发布顺序为先迁移后服务；回滚代码时保留新增字段不影响旧查询，数据回滚通过恢复原分支或反向提交完成，不删除审计记录。
