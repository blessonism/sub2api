# 技术设计

## 数据模型

- `announcement_email_broadcasts`：公告唯一外键、标题与 HTML 快照、状态、总数/成功数/失败数、操作人与时间。
- `announcement_email_deliveries`：任务、可空用户外键、邮箱快照、状态、尝试次数、租约、错误与投递时间；任务和用户唯一。
- 公告外键使用 `ON DELETE RESTRICT`，用户外键使用 `ON DELETE SET NULL`。

## 服务与数据流

1. 查询详情时校验公告并返回已有任务；未创建时分页读取正常用户并计算当前可发送人数。
2. 创建时先校验 SMTP 和公告有效期，再计算收件人并在一个事务中写入任务及投递快照。
3. 三个 Worker 轮询数据库，以 `FOR UPDATE SKIP LOCKED` 原子领取 pending 或租约过期的 processing 投递。
4. Worker 调用现有 `EmailService.SendEmail`，随后在事务中更新投递结果与任务计数；终态为 completed 或 partial_failed。
5. 重试接口只把 failed 投递恢复为 pending，并相应重置任务失败计数与状态。

## API

- `GET /api/v1/admin/announcements/:id/email-broadcast`
- `POST /api/v1/admin/announcements/:id/email-broadcast`
- `GET /api/v1/admin/announcements/:id/email-broadcast/deliveries`
- `POST /api/v1/admin/announcements/:id/email-broadcast/retry-failed`

详情响应在未创建时返回 `broadcast: null` 和 `eligible_count`；创建后返回任务汇总。投递列表支持 `status`、`search`、`page` 和 `page_size`。

## 安全与恢复

- 使用 goldmark 默认安全渲染，不启用原始 HTML；站点名、标题和模板变量均转义。
- 邮件地址使用标准库校验，并复用服务层保留邮箱判断排除合成地址。
- 错误信息截断后保存，不记录 SMTP 密码；完整邮箱只通过管理员接口展示。
- Worker 使用固定租约和有界查询；重启后自动领取租约过期或 pending 的投递。
