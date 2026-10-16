# 验证记录

实现位于 `feature/channel-monitor-user-cards` 独立工作树，基线 `custom/main` / `46df93de6`。仅改 V1 用户卡片及必要的管理、数据支持。用户已明确要求合入本地 `custom/main`，本轮据此提交与合并；不推送或部署。

## 已通过

- Ent、Wire 正式生成；生成结果只包含本次 group_id 和服务接线。
- Go 定向单元：`go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin -run 'ChannelMonitor|BuildUserView|BuildTimelinePoints|EffectiveVisibleRate|GetUserGroupVisibility' -count=1`。新增 HTTP 用例修正 fixture 的 EffectiveStatus 后，单独重跑 `./internal/handler -run ChannelMonitorUser` 通过；service 和 admin 均通过。
- PostgreSQL 18.1 + Redis 8.4 真实容器集成：`go test -tags=integration ./internal/repository -run 'TestChannelMonitorLinkedGroupLifecycle|TestChannelMonitorDisplayMetricsAggregation|TestOpsOutputTPS' -count=1 -v`，5 个顶层测试通过，包含运维输出速度回归。
- `go test ./cmd/server -run '^$'`：正式 Wire 入口编译通过。
- 前端定向 Vitest：9 文件 / 38 项通过，包含新卡片、分组表单、原配额、刷新、详情、V1/V2 切换、API 文档入口及 i18n 完整性/键冲突。
- `pnpm typecheck`、变更 TS/Vue/i18n 文件的定向 ESLint、`git diff --check` 通过。

Go 命令使用 `GOPROXY=https://goproxy.cn,direct`，绕过本机默认 proxy 的连接超时。

## 视觉验收

使用真实 MonitorCard / MonitorCardGrid 和项目样式、zh/en i18n，在 Chrome 的独立 fixture 预览中验证（不是生产页面截图）。

- 360、768、1440px × light/dark × zh/en，共 12 组。
- 每组包含参考图正常指标、超长名称/模型、极大倍率/响应时间/TPS、缺失指标、降级、限流、失败、零值。
- 页面、卡片和三列指标均无横向溢出；浏览器未捕获 pageerror。极大倍率/响应时间省略显示，完整值保留在提示中。
- 按用户反馈将边框改为低饱和灰蓝渐变；手工对照参考图检查边框、平台图标、状态与时间、三列分隔、TPS 和 60 条历史。修正 scoped global selector 后，深色卡片背景确认为 #1f2937。
- 截图与尺寸检查 JSON 保存在 `/Users/suki/.codex/visualizations/2026/10/17/channel-monitor-user-cards/`，含 card-light.png / card-dark.png 和各尺寸截图。临时 fixture 源码、开发服务和 Playwright 临时依赖已清理。

## 限制与交付说明

- 额外尝试的全 service 包 `-race` 编译受本机磁盘空间不足影响，未得到 race 检测结果；已清理本轮失败构建缓存。普通并发单元测试（相同批次、重叠批次、调用者取消）通过。
- 存量监控 group_id 默认 NULL，需要管理员补选关联分组；未关联时新指标显示缺省值，原探测仍可用。
- 迁移必须在新版服务启动前应用。没有自动按 group_name 回填，也没有修改真实生产数据。
- 原工作区已有不属于本任务的未提交日志与文件；合并必须保留，不纳入本次提交。

## 边框配色复核

根据用户反馈，边框从粉紫改为灰蓝：浅色 #d7e1ed → #e2e8f0，深色 #3b4b61 → #334155，悬停采用同色系。已重新生成全部 12 组截图和 verification.json，确认边框变量生效、无浏览器错误和横向溢出；MonitorCard 定向 ESLint 通过。第二轮临时预览和依赖也已清理。

## 无边框阴影调整

按用户最新反馈，卡片与加载骨架去掉外边框，改用轻柔阴影；深浅主题和悬停分别调整阴影强度。保留内部指标分隔线及键盘焦点提示。前述灰蓝边框截图属于上一版验收，不能代表最新阴影样式。

最新阴影改动已通过 MonitorCard/MonitorCardGrid 定向 ESLint 与 `git diff --check`；此次只调整外观，没有重跑前述全部行为测试。
