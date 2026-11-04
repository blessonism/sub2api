# 实施记录

## 阶段一

- [x] 迁移共享函数，拆除专属模块及入口，更新现行规范。
- [x] 重新生成 Ent、Wire，并验证二次生成一致性。
- [x] 完成后端定向回归及前端检查，保存仓库外差异检查点。

## 阶段二

- [x] 新增向前清理迁移与必要迁移回归。
- [x] 隔离本地 PostgreSQL 完成备份、升级、失败回滚、恢复、序列和重复执行验证。
- [x] 隔离本地 S3 完成对象清点、备份、版本清理及失败重试验证。
- [x] 清表后验证正常入口，核查残留，记录质量检查结果。
- [x] 清理本次一次性文件与容器。

## 验证命令

- backend 中 go generate ./ent 与 go generate ./cmd/server，二次运行结果一致。
- backend 中 go test -tags unit ./internal/config ./internal/handler ./internal/handler/admin ./internal/server/routes ./internal/service ./internal/repository ./cmd/server。
- backend 中 go test -tags integration ./internal/repository -run 'TestMigrationsRunner|TestConversationHistoryRemoval'。
- frontend 用现有 node_modules 执行 vue-tsc、eslint、受影响 Vitest 和 Vite 构建。
- git diff --check 与专属模块残留扫描。

## 结果

两阶段代码与验证已完成。详细结果见 verification.md。Review 通过，交付按应用拆除与数据清理拆分为两个提交，随后归档任务及会话记录。
