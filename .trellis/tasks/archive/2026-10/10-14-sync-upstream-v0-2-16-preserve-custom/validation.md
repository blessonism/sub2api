# 同步验收记录

## 合并与交付

- 同步前 `custom/main`：`b16284c40d5bf8cd99dc5c77ff683ce3c450295a`。
- 官方 v0.2.16：`4d9ac3e482106d2da3a39fe3e546d8d6801f1cc1`。
- 共同祖先：`b8dece9000c68815a5b867ca5a1e6f236e173905`。
- 在 `sync/upstream-0.2.16` 合入固定官方 SHA，包含 233 个官方提交。24 个文本冲突已逐块解决；`backend/cmd/server/VERSION` 为 `0.2.16`，Ent/Wire 再生成结果稳定。
- 用户已确认“已有失败修一下，然后提交更新，不推送”。交付方式为本地 merge commit，验证通过后快进 `custom/main`；起始 7 项未跟踪路径不纳入提交。

## 二开保留与适配

1. 分组保留可见倍率、分时倍率、OAuth 过滤和 OpenAI 调度覆盖；接入官方分组 I18n。用户 API Key、订阅、可用渠道和支付套餐继续按二开可见倍率输出，并按 `Accept-Language` 本地化名称。
2. Gemini 模型列表/详情切换到官方新选号入口时，同步迁移二开的严格账号绑定、绑定优先及允许回退策略；旧入口不再保留无调用的副本。
3. 官方 Gemini 显式缓存存储费接入二开计费链路。报价与落库使用相同的请求 `PricingAt`，同时保留实际/可见分时倍率、响应模型计费等扩展。
4. 渠道状态保留智力检测及 API 文档入口；自动和手动刷新同时更新基础列表、检测面板及已展开的详情。
5. Wire cleanup 同时保留二开公告、活动、抽奖、对话采集和上游中继等 worker，以及官方缓存服务清理。
6. 补齐测试构造参数；对话采集过滤 ID 列表注册空切片默认值并验证实际环境变量加载。仓储兼容旧分组调用方未填 `TimeRatePriority` 的情况，使用 `schedule_first`。
7. 复核并恢复官方 HTTP/WS 失效密文 lineage、Kimi 原生 Responses 地址、Ollama 输出限制、代理错误记录、映射后模型提示模板以及分组绑定并发锁；保留二开提示模板开关与 Codex ticket 注入。


## 已有失败修复

- 前端：补齐账号到期预设、请求 ID 响应头和 OpenAI 图片转换开关；配置共用写入路径覆盖普通、批量及 Codex 导入。轻量账号列表打开弹窗前取完整详情，按目录展示分组，部分失败保留选择；请求参数使用快照，菜单内部滚动保持打开。简单模式隐藏高级分组操作并停止相关请求。
- 前端测试：修复不完整 mock、restore 后失效的列表默认实现、倍率和列配置断言、菜单默认模式及消息参数，消除未处理异常；新增两项 Codex 导入响应头回归。
- 后端：简单模式在服务层拒绝复合分组和高级操作；支付履约使用可信兑换入口，恢复前校验已使用充值码的用户归属。未装配倍率解析器时使用无缓存回退，修复每请求创建 janitor 缓存并被全局表持有的内存泄漏，保留原内存上限断言。
- 后端测试：设置用例按实际完整 PUT 契约先读再合并；更新账号 extra SQL fixture、usage log 列位、JWT 活跃触碰与 API 白名单断言，监控校验不依赖本机 DNS。
- 集成测试：补齐统计夹具的缓存 Token；检查用户夹具清理错误，在隔离数据库中级联清理审计外键关联数据，确保列表及订阅测试互不污染。
- lint：修复资源释放、类型断言、无效赋值、格式与错误文案，删除无调用的私有辅助函数；未新增 lint 豁免。最终取消重复问题输出限制，报告为 `0 issues`。

## 最终本地验证

| 检查 | 结果 |
| --- | --- |
| 前端完整 Vitest | 405 个文件通过；3,144 个测试通过、4 个原有跳过；0 未处理异常 |
| 前端 lint / typecheck / build | 全部通过 |
| 后端完整 unit 套件 | 57 个测试包通过；0 失败 |
| 后端完整 integration 套件 | 51 个测试包通过；真实隔离 PostgreSQL 18.1 / Redis 8.4 |
| 后端 golangci-lint v2.14.0 | `0 issues`，重复问题输出限制已取消 |
| 后端普通 / embed 构建 | 全部通过，`CGO_ENABLED=0`，输出到 `/dev/null` |
| CI 部署脚本、Compose 环境与 Caddy 缓存策略检查 | 全部通过 |
| 发布矩阵与 shell 语法 | 10 个 Python 测试通过，shell 语法通过 |
| Ent / Wire 生成与二开定向行为回归 | 通过；生成稳定性记录保留 |
| 冲突与 diff 检查 | 无未解决冲突；工作区和暂存区检查通过 |

上述为本地验证，未推送、未运行远端 CI、未部署。lint 修复后的最后一次完整后端测试及两种构建已全部完成并通过。

## 证据与复现

`validation-evidence.json` 保留固定引用、定向检查、历史基线复现及最终命令/结果。原 32 项前端失败、39 项未处理异常、18 个后端失败组和 53 项已报告 lint 问题已修复；完整检查额外发现的集成夹具及重复 lint 问题也已处理。历史记录用于定位问题，最终结果以 `final_validation` 为准。

本地 Go 为 1.27.2；前端使用现有 Node 25.7.0 / pnpm 10.33.2。前端限制 2 个 worker，后端 `GOMAXPROCS=2`、`-p 1`；后端测试、lint 和构建按顺序执行。此前磁盘不足、编译中断及失败检查不计为通过，最终通过结果均有完整结束记录。仓储保留原有 concurrency TODO 跳过项。

主要复现命令：

```bash
# frontend
pnpm exec vitest run --maxWorkers=2 --minWorkers=2
pnpm lint:check
pnpm typecheck
pnpm build

# backend
GOMAXPROCS=2 go test -p 1 -tags=unit ./... -timeout=600s
GOMAXPROCS=2 go test -p 1 -tags=integration ./... -timeout=600s
golangci-lint run --concurrency=2 --max-same-issues=0 --max-issues-per-linter=0 --timeout=10m ./...
```

## 提交与回滚

工作提交：`chore(sync): 同步官方 0.2.16 并保留二开`。提交并快进本地 `custom/main` 后，仅归档当前 Trellis 任务并记录本次 journal。

合并提交可用 `git revert -m 1 <merge-commit>` 创建反向提交；不改写共享历史。生产环境及原有未跟踪文件保持原状。

## 本地交付结果

- 同步 merge commit：`8983b05ff58c71a73beb405f9034158b8f7e9039`，父提交为原二开基线和固定官方 v0.2.16 SHA。
- 本地 `custom/main` 已快进到该同步结果；当前任务随后归档并记录 journal。
- 未推送、未部署；起始 7 项未跟踪路径保持原状。
