# 技术设计

## 数据模型

- `account_collections`：`id`、`name`、`sort_order`、时间戳和软删除字段；使用活动记录部分唯一索引约束名称。
- `account_collection_members`：`account_id`、`account_collection_id`、`created_at`，联合唯一键；外键删除采用级联，仅删除关联。
- Account 与 AccountCollection 通过显式关联实体建立多对多关系，独立于现有 `groups/account_groups`。

## API 契约

- `/api/v1/admin/account-collections`：列表、创建；`/:id`：改名、删除；`/sort-order`：批量排序。
- `/api/v1/admin/account-collections/members/batch`：`account_ids`、`account_collection_ids`、`operation=add|remove`。
- 账号列表新增 `account_collection_id` 查询参数；账号创建/更新请求新增 `account_collection_ids`，账号 DTO 返回 `account_collections` 和 `account_collection_ids`。
- 所有 ID 在写入前整体校验；任一不存在则拒绝请求，避免静默部分执行。批量关系写入使用事务和冲突忽略保证幂等。

## 前端数据流

- AccountsView 同时加载账号编组和现有业务分组；快捷标签写入列表查询参数并重载第一页。
- 当前筛选编组 ID传入 CreateAccountModal 作为初始选择；EditAccountModal 使用账号 DTO 中的当前关系。
- 管理弹窗负责 CRUD/排序，成功后刷新编组列表并修正已删除的活动筛选。
- 批量归组弹窗只调用增量成员 API，不复用现有覆盖式业务分组更新。

## 风险与回滚

- 新表和新字段为向后兼容增量；旧客户端和现有调度路径不读取账号编组。
- 回滚应用版本时新表可保留，不影响旧版本；不在自动回滚中删除数据表。
