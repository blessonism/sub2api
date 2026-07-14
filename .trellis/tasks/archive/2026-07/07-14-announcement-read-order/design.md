# 公告已读情况排序设计

## 边界与数据流

`AnnouncementReadStatusDialog` 发送 `sort_by=read_at&sort_order=desc`，管理员公告 Handler 将参数交给 `AnnouncementService.ListUserReadStatus`。Service 在用户查询过滤条件中携带公告 ID，User Repository 通过 `users LEFT JOIN announcement_reads` 在数据库分页前完成排序；当前页用户再沿用现有逻辑补充阅读时间与公告可见性。

## 排序契约

- 仅当 `sort_by=read_at` 且公告 ID 有效时连接 `announcement_reads`。
- 连接条件同时限定 `user_id` 和当前 `announcement_id`。
- 排序键依次为：`read_at IS NULL` 升序、`read_at` 按请求方向、`users.id` 升序。
- `announcement_id + user_id` 的唯一约束保证连接不会产生重复用户。

## 兼容性

- API 响应结构不变，无需迁移。
- 邮箱、用户名、余额等现有排序路径不变。
- 默认排序从邮箱升序改为阅读时间倒序；用户主动切换其他列后仍按现有行为工作。

## 回滚

回滚 Handler 与弹窗默认参数，并删除 User Repository 的公告阅读排序分支即可；不涉及数据回滚。
