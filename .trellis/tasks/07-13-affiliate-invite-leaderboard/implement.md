# 实施计划

1. 扩展邀请返利服务契约、仓储聚合查询、管理员 Handler 和路由，补充聚合与排序测试。
2. 扩展前端管理员 API 类型与方法，新增排行榜页面、路由、侧栏入口和中英文文案。
3. 补充 API 与页面测试，运行相关 Go 测试、Vitest 和前端类型检查。
4. 检查下游分支边界、差异范围和任务验收条件。

## 验证命令

```bash
go test ./internal/repository ./internal/service ./internal/handler/admin ./internal/server/routes
pnpm exec vitest run <邀请排行榜相关测试>
pnpm typecheck
```

## 风险与回滚点

- 聚合 SQL 必须先分别聚合邀请关系与兑换码，避免重复放大金额。
- 搜索必须发生在排名之后，避免筛选时重排名。
- 不执行数据库迁移、生产操作、提交或推送。

## 验证结果

- 邀请排行榜仓储定向测试、相关服务测试和后端包编译通过。
- 前端榜单 API、页面、路由、侧栏及 locale 定向测试通过。
- 前端类型检查与完整 ESLint 检查通过。
- 后端 `internal/repository` 全包测试仍有与本任务无关的既有失败，涉及对话采集、上游中继和 usage log 测试契约漂移；本任务定向用例通过。
