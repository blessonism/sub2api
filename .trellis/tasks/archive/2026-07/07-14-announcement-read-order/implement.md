# 实施计划

1. 扩展用户列表过滤条件，使公告阅读列表能把公告 ID 传入 Repository；在数据库查询中实现已读在前、未读在后且稳定的阅读时间排序。
2. 将管理员阅读情况 API 与弹窗的默认排序改为 `read_at desc`，并开放阅读时间列排序。
3. 增加 Handler 默认参数测试和 Repository SQL 排序测试，覆盖公告限定、空值置后和稳定次序。
4. 运行目标 Go 测试与前端组件测试，确认现有排序与弹窗交互无回归。

## 风险文件

- `backend/internal/repository/user_repo.go`：共享用户列表查询，只允许在公告 ID 有效且排序字段为 `read_at` 时进入新分支。
- `frontend/src/components/admin/announcements/AnnouncementReadStatusDialog.vue`：保持搜索、分页和取消请求逻辑不变。

## 验证命令

```bash
cd backend && go test ./internal/repository ./internal/handler/admin ./internal/service
cd frontend && pnpm exec vitest run src/components/admin/announcements/__tests__/AnnouncementReadStatusDialog.spec.ts
```
