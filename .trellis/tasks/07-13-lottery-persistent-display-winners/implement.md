# 实施计划：抽奖活动常驻展示与中奖名单公开

## 实施顺序

1. 调整 repository 活动选择查询和 service 可见性/常驻校验，保证已结束常驻活动可读但不可报名，并补充后端单元测试。
2. 注册管理员活动级中奖名单路由，复用现有 service/repository 查询能力，补充 handler/API 契约测试。
3. 扩展管理员抽奖 API 与管理面板：加载并展示选中活动中奖名单，更新常驻操作状态和中英文文案。
4. 更新用户侧测试，覆盖没有当前新活动时展示已结束常驻活动及公开中奖名单。

## 验证计划

- `go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/server/routes`
- `pnpm --dir frontend test:run src/api/__tests__/lotteryCampaigns.spec.ts src/components/admin/activities/__tests__/LotteryCampaignAdminPanel.spec.ts src/components/user/activities/__tests__/LotteryCampaignActivity.spec.ts src/views/user/__tests__/CampaignRewardsView.spec.ts`
- `pnpm --dir frontend typecheck`

所有单项测试命令设置 60 秒上限；若包级测试超时，缩小到相关测试函数并记录未覆盖范围。

## 风险与回滚点

- 可见性规则必须与报名规则分离，禁止因公开已结束活动而重新开放资格写入。
- 管理中奖 DTO 不得复用于公开 API；用户侧始终只接收脱敏 DTO。
- 管理面板切换活动时必须处理请求竞态，避免显示上一活动中奖名单。
- 本任务不修改表结构；回滚时恢复相关查询、路由和 UI 即可。
