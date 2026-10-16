# 实施顺序

- [x] 新增迁移、Ent group_id 及生成代码；贯通管理 DTO/service/repository/duplicate。
- [x] 独立展示 service/repository 批量聚合、缓存、权限、降级；Wire 与用户 DTO。
- [x] 管理表单分组选择/Key 带入；卡片与状态条、格式化、zh/en overlay。
- [x] 服务/handler/真实 PostgreSQL/前端行为测试。
- [x] pnpm typecheck、i18n、定向 lint/Vitest、Go unit/integration；360/768/1440 深浅主题视觉验收。
- [x] 复核跨层读写、缺失字段、纯配额与存量兼容，更新契约与验证记录。

使用 feature/channel-monitor-user-cards 独立工作树，base custom/main。不得混入原工作树改动。实现与检查均主会话直接完成。
