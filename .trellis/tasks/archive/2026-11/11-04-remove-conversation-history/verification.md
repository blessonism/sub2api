# 本地验收结果

## 应用拆除

- 基线 custom/main 的 21b8b4ff478011ea91124b7ed8bca499ca35c797。
- 工作目录 /Users/suki/code/sub2api-remove-conversation-history，分支 fix/remove-conversation-history。
- 专属后台、接口、采集状态、响应缓冲、落库和导出 worker、清理任务、配置、Ent 实体与 Wire 注册已删除。
- 两个共享函数保持原语义，其他二开通过局部差异保留。
- 活跃代码专属引用扫描为零，历史 161–163 迁移逐字保持一致。

## 自动检查

- 八个受影响后端包的 unit 测试全部通过，覆盖 config、handler、handler/admin、server/routes、service、repository、cmd/server 及 server 根包接口契约。
- PostgreSQL repository integration 定向测试通过，覆盖新库、旧库升级、重复执行、依赖冲突回滚以及现有迁移并发与 schema 检查。
- Ent、Wire 二次生成逐文件 SHA256 一致。
- go build ./cmd/server、go vet 受影响包、Go 格式检查与 git diff --check 通过。
- 前端路由、侧栏、语言包共 17 个测试文件、143 项测试通过，包括旧页面 404 回归。
- vue-tsc、改动文件 eslint 与 Vite 生产构建通过。因本机 Corepack 入口异常，直接使用既有 node_modules 二进制执行。

## 真实应用入口

在专用本地 PostgreSQL、Redis 上从空库启动新构建应用，连接本地模拟 OpenAI 上游。

- /v1/chat/completions、/chat/completions、/v1/responses、/responses、/backend-api/codex/responses 各执行流式和非流式请求，10 次全部返回 200。
- 在三张对话表均不存在的情况下，写入 10 条 usage_logs，输入 120 tokens、输出 80 tokens，用户余额正常扣减。
- 旧管理员 config 和 sessions 接口均返回 404。

## 数据演练

- 独立 PostgreSQL 容器中构造旧表，每张一条记录，包含已启用采集设置与普通备份设置。
- 使用 pg_dump 生成备份，先恢复到另一数据库，再清理旧库并完成清理后恢复。三个序列恢复后 nextval 均为 2。
- 再次清理及重复清理通过，普通备份设置保持原值，最终专属表与序列归零。
- 对象存储使用本地 Moto 5.2.5 S3 兼容服务，构造登记对象、孤儿对象、4 个对象版本和 1 个删除标记。
- 逐版本备份并校验 SHA256，模拟一次 503 删除失败后重试，重复清理通过。恢复校验对象内容、当前状态和版本数量，普通备份对象保持原版本及内容。
- 恢复会由 S3 服务重新分配 VersionId，备份清单保留原 VersionId 供溯源。

## 两阶段产物

本地演练产物保存在 /Users/suki/.codex/artifacts/sub2api-remove-conversation-history-20261104，全部为人工测试数据。

- phase1.patch 保存应用拆除检查点。
- phase2.patch 单独保存清理迁移与迁移回归。
- phase1-verification.json、database-rehearsal.json、object-rehearsal.json、app-smoke.json 保存验证摘要。
- legacy-backup.dump、object-manifest.json 与 objects/ 保存测试备份。
- legacy-backup.dump 的 SHA256 为 cdb0f75146611e88de06693a4e06876631e526f54cb02507a901a82e0471dbc3。

## 发布边界

本次数据库与对象操作均限定隔离本地测试环境。254_remove_conversation_history.sql 属于第二阶段，会随应用启动自动执行。后续生产发布应先使用阶段一，完成真实备份和对象清点后再引入阶段二。

变更经 review 后按应用拆除与数据清理拆为两个 Git 提交，提交后归档任务。原工作目录仍为团队挂账分支及其原有 40 项未提交变更。

## 提交前 Review

- 已完成网关、计费回调、共享函数、管理入口、前端路由、生成代码及清理迁移的差异审查，未发现阻断项。
- 本轮复核 8 个后端包测试、17 个前端文件的 143 项测试及 git diff --check 全部通过。
- 数据库恢复和对象存储清理依据实施阶段隔离 PostgreSQL、Moto S3 的演练结果，本轮未重复执行数据演练。

## Git 交付

- 第一阶段提交 16f596878f36397ae4350145c062a935a600a1b2，完成应用拆除并保留旧表。
- 第二阶段提交 4d421688a9bc79b188b8332f8f935388de9c50cf，追加清理迁移与集成测试。
- 两个提交均位于 fix/remove-conversation-history，第二阶段的自动迁移行为与备份前置要求已在提交说明中标注。
