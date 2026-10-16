# 全平台 System Prompt 注入验证

## 实现

- 前后端使用统一平台目录开放全部登记平台与 composite；无新增数据结构、缓存版本或迁移。
- 保持原 API Key 分组快照、管理员 DTO、关闭保留草稿与更新省略保持等既有行为。
- 新增 Gemini、System One 协议策略；补齐 Gemini/Antigravity/Grok 的独立出口，包括 Grok Messages/WS HTTP bridge、Antigravity 模型兜底及签名修复。
- TypeSafe 对每个问题的 instructions 应用模式，保留 state、criteria 与类型，并显示中英文专用说明。
- 独立图片、音视频接口继续沿用原行为；Gemini/Antigravity 本地计数仍为估算。

## 验证结果

- 最终定向后端：service 494、admin handler 2 个通过项（含父测试/子测试），0 失败；覆盖原有 HTTP/WS 与配置契约及本次全平台扩展。
- 相关独立网关回归：300 个通过项，0 失败，覆盖 GeminiForward、AntigravityForward/Compat、Grok、System One、计数与 WS HTTP bridge。
- 真实 PostgreSQL/Redis：OpenAI、DeepSeek、Gemini、TypeSafe、composite 保存、认证投影、缓存命中/失效、复制，以及迁移默认/durable outbox 通过；CI=true 保证 Docker 不可用时不会静默跳过。
- 前端 GroupsView 36、API 2、共享组件 6、i18n completeness 3、collision/ops 33，共 80 测试通过；创建/编辑/切平台/simple mode、正文保留与空白校验覆盖。
- 前端类型检查、变更文件 ESLint、后端全仓 unit 编译、gofmt、git diff --check、任务上下文 validate 通过。
- golangci-lint v2.14.0 全 unit 范围发现既有测试层依赖、静态建议等报告；本次新增的 switch 建议已修复，使用 --new-from-rev=HEAD 验证通过（0 issues）。未修改无关代码。

## 交付方式

延续本会话用户“合并到 custom main”“部署到 ssh ovh”授权，把修复合入本地 custom/main 后从固定提交 git archive 构建。无 Git push，既有工作树 journal/旧任务不纳入。生产仅切换 sub2api，保留当前镜像与 override 备份供回滚。部署结果另记录在 deployment/。
