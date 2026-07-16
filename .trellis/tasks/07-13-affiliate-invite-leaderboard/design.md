# 技术设计

## 边界与契约

- 管理端新增 `GET /api/v1/admin/affiliates/leaderboard`，接受 `page`、`page_size`、`search`，返回标准分页结构。
- 榜单行字段为 `rank`、`user_id`、`email`、`username`、`aff_code`、`invite_count`、`all_credit_amount`、`payment_redeem_amount`。
- 服务层沿用 `AffiliateService`，仓储层在 `affiliate_repo.go` 内完成聚合和全局排名。

## 数据流

1. 从未删除邀请人的 `user_affiliates.aff_count` 取得累计邀请人数。
2. 按受邀关系汇总受邀用户 `users.total_recharged`。
3. 独立汇总受邀用户已使用、正值、余额类型的 `redeem_codes.value`，避免与关系聚合形成笛卡尔积。
4. 在搜索前通过窗口函数生成全局名次，再搜索、分页。
5. 前端通过独立 API 方法加载并使用现有后台表格与分页组件展示。

## 兼容与回滚

- 不修改既有接口、表结构和邀请绑定逻辑。
- 删除新增路由、页面、API 方法及仓储方法即可回滚。
- 统计依赖保留的兑换码记录；已硬删除记录无法回溯，本任务不补建流水。
