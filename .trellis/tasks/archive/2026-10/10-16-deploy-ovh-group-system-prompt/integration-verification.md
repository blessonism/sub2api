# 真实集成与生产构建验证

固定 SHA 干净源码快照中执行：

- `GOPROXY=https://goproxy.cn,direct GOMAXPROCS=4 go test -tags=integration -p 2 ./internal/repository -run GroupSystemPrompt -count=1 -v`：真实 postgres:18.1-alpine3.23 / redis:8.4-alpine，两个用例通过。
- `TestGroupSystemPromptPersistenceProjectionAndCache`：持久化、认证投影、缓存命中/失效和复制。
- `TestGroupSystemPromptMigrationDefaultAndDurableInvalidation`：数据库默认值、配置更新持久化失效。
- `pnpm run build`：i18n 完整性 3 测试、vue-tsc 和 Vite 生产构建通过。既有 chunk 大小/Browserslist 提示不阻断构建。

Docker Hub alpine manifest 请求超时；复用本机与当前生产镜像 ID 完全一致的已验证运行时，仅替换本次新编译的内嵌前端 Go 程序。运行时资源与 entrypoint 从当前生产提交到目标未变化；记录派生 Dockerfile 和镜像配置比较。
