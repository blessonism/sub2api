# 实现清单

1. 扩展后端统计 DTO、仓储聚合 SQL、服务转换和现有测试。
2. 扩展前端 `WindowStats` 类型、账号今日统计单元格和中英文文案。
3. 补充或更新前后端定向测试，验证缓存命中字段和命中率边界。
4. 运行 Go 格式化/测试、前端定向 Vitest 与类型检查，修复回归。

## 验证命令

- `gofmt -w` 修改的 Go 文件。
- `go test ./internal/repository ./internal/service ./internal/handler/admin`
- `pnpm exec vitest run src/components/account/__tests__/AccountTodayStatsCell.spec.ts`
- `pnpm typecheck`

## 风险与回滚点

- SQL 聚合字段必须与 Scan 参数顺序一致；单元/集成测试失败时先检查顺序。
- 前端旧接口数据缺失字段时按零值处理，确保已有账号列表不崩溃。
- 变更不新增迁移；需要回滚时撤销 DTO、查询和组件字段即可。
