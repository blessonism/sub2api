# 实施计划

1. 增加 Ent schema、SQL 迁移、领域/仓储/服务接口和账号 DTO 映射。
2. 注册管理员账号编组 CRUD、排序、批量成员操作 API，并把筛选和创建/更新关系接入账号 API。
3. 增加前端类型/API、快捷筛选、管理弹窗、批量归组弹窗和创建/编辑归属选择。
4. 补齐后端服务/接口测试和前端 API/组件测试。
5. 执行代码生成、格式化、后端目标测试、前端测试与类型检查，复核分支和 Trellis 上下文。

## 实施记录

- 使用迁移 `190_account_collections.sql` 建立独立数据表，未修改现有业务分组或调度模型。
- 账号列表筛选通过请求上下文把 `account_collection_id` 下传至仓储查询，避免扩大核心 AccountRepository 接口并破坏大量调度侧实现。
- 创建/编辑账号由前端账号 API 在账号保存成功后同步独立编组关系；批量归组使用独立增量接口。
- 已生成 `wire_gen.go`，完成目标 Go 测试、前端类型检查、API/编辑弹窗测试与 ESLint 检查。

## 验证命令

- `go test ./internal/service ./internal/handler/admin ./internal/server ./internal/repository`
- `pnpm exec vitest run <account collection related specs>`
- `pnpm typecheck`
- `git diff --check`
