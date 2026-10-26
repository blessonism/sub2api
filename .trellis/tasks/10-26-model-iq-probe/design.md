# 技术设计：糖果问题探针与智商监控

## 边界

- 只新增独立子系统（service/repo/handler/表/端点/页面），不改动现有 V1/V2 渠道监控、challenge 逻辑、中继探针。
- 判分"出现即通过"与 `channel_monitor_challenge.go` 的 `validateChallenge` 语义一致。
- 探测执行复用 `AccountTestService`（账号级测试通道），因此 **token 用量不可得**：表的 token 三列保留为 nillable（NULL=无，对齐 `channel_monitor_histories` 约定），当前恒为 NULL，管理端展示"—"。

## 实现偏离记录（实现后回填）

- 用户端区块二次迭代（操作者反馈"模型通常只有 1~2 个、行式列表不够美观、信息量不足"）：从"每模型一行 + 10 个小圆点"升级为**模型卡片**——回答引用框 + 近 20 次条带时间线（对齐监控卡片视觉语言）+ 正常率/连续正常统计；单模型时卡片占满整行。后端 `modelIQProbeRecentPerModel` 10 → 20。
- 用户端区块三次迭代（操作者反馈"仪表右侧文本不对齐偏空、红绿条太土"）：回答区改为与环形仪表等高（70px）的浅色盒并垂直居中；条带改为 **MonitorTimeline 同款**固定 20 槽 + 状态定级高度（正常 100% / 疑似降智·失败 40% / 幽灵占位 16%，不足左补齐），替代等宽砖块条。对齐经 DOM 几何测量验证（行高 70/70/70、条带高度三档 4/11/28px、内容居中 16/16px）。
- 用户端区块四次迭代（操作者反馈"仪表右侧上方空、下面挤；条带仍偏砖块，渠道状态的原版更好"）：布局重排为**回答框居左占满宽度、环形仪表做右侧栏**（等高对齐，阅读动线更顺）；条带完全对齐渠道状态卡片规格——h-4 矮条 + rounded-sm + MonitorTimeline 同款分级（100/35/15），条带上方加"最近 20 次探测 · 由旧到新"节奏行。
- 用户端区块五次迭代（操作者反馈"次数太少，不密集不好看"）：条带深度与后端 `modelIQProbeRecentPerModel` 对齐渠道监控卡片时间线的 **60 点**规格（默认 6h 间隔下约 15 天历史，30 天保留期内），幽灵占位仅在探测次数不足 60 时出现。
- `max_tokens` 配置项已移除：`AccountTestService` 通道不支持限制生成 token 数（`AccountTestOptions` 仅多媒体字段），保留即死配置。
- 账号标识用 `account_name`（账号名）而非邮箱：账号名在管理端本就可见，无需脱敏；用户端汇总不输出任何账号信息。
- `POST /admin/model-iq-probe/run` 不带过滤参数：手动触发固定跑全量目标，行级重试后续按需补充。
- 结果 DTO 增加 `effective_status` 字段（override ?? status 的冗余输出），避免前端自行合并逻辑。
- token 三列当前恒为 NULL（执行通道无 usage），表结构保留，未来切换执行通道时启用。
- 开发辅助（不入库/不提交）：`frontend/preview.html` + `frontend/src/preview/*` + `frontend/vite.preview.config.ts`，构成无后端容器的内存 Mock 预览环境（axios adapter 层拦截）。

## 数据模型

### 迁移 `backend/migrations/249_model_iq_probe_results.sql`

```sql
CREATE TABLE IF NOT EXISTS model_iq_probe_results (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT,
    account_email TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    protocol TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,              -- pass / wrong_answer / error
    error_category TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    expected_answer TEXT NOT NULL,
    actual_answer TEXT NOT NULL DEFAULT '',   -- 截断 <= 4KB
    latency_ms BIGINT NOT NULL DEFAULT 0,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cache_read_tokens INTEGER,
    override_status TEXT,              -- NULL = 未人工改判
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_model_iq_probe_results_model_created
    ON model_iq_probe_results (model, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_model_iq_probe_results_account_created
    ON model_iq_probe_results (account_id, created_at DESC);
```

### ent schema `backend/ent/schema/model_iq_probe_result.go`

与迁移字段一一对应；`account_id`/token 三列/`override_status` Optional+Nillable。表结构以 SQL 迁移为准，ent 仅代码生成（`go generate ./ent`）。

### 生效判定

`effectiveStatus = override_status ?? status`；用户端汇总与管理端列表均用生效值。

## 配置（settings 表）

- key：`model_iq_probe_config`（`SettingKeyModelIQProbeConfig`，加入 `domain_constants.go`）。
- JSON 结构与默认值：

```json
{
  "enabled": false,
  "question": "<糖果问题原文，含表格>",
  "expected_answer": "21",
  "interval_minutes": 360,
  "target_mode": "account_model",
  "models": [],
  "account_ids": [],
  "max_tokens": 2048
}
```

- 校验：expected_answer 非空；interval_minutes ∈ [5, 10080]；target_mode ∈ {account_model, model_only}；max_tokens ∈ [64, 8192]；models/account_ids 为非负字符串/ID 列表。非法配置返回 400 + `MODEL_IQ_INVALID_CONFIG`。
- 读写走 `SettingService.GetRawSettingValue/SetRawSettingValue`（JSON 编解码 + 缺省值合并）；PUT 成功后调用 runner `Reload()`。

## 探测执行

### `backend/internal/service/model_iq_probe_service.go`

- `ModelIQProbeService`：配置读写（Load/Save/Defaults）、判分 `validateModelIQAnswer(text, expected) string`：
  - expected 可解析为整数（含负号）→ 用 `-?\d+` 提取全部整数 token，出现即 pass（复用 challenge 正则语义）；
  - 否则大小写不敏感包含匹配。
  - 实际答案截断 4KB 存档。
- 目标解析 `resolveTargets(cfg)`：
  - 账号集合：`account_ids` 非空 → 指定账号（须为 active OpenAI 账号）；为空 → 全部 active OpenAI 账号（`Account.IsOpenAI()`）。
  - 模型集合：`models` 非空 → 指定列表；为空 → 各账号模型字段中 `gpt-` 前缀（大小写不敏感）去重联合。
  - `account_model`：账号 × 模型（账号支持该模型才探测，`Account.IsModelSupported`）；`model_only`：每模型选支持它的第一个账号（按 id 升序，确定性）。
- 结果写入 + 查询走 `ModelIQProbeRepository`。

### `backend/internal/service/model_iq_probe_runner.go`

- `ModelIQProbeRunner`：goroutine + `time.Timer` 循环（非全局 ticker，仿 `channel_monitor_runner.go`）；间隔 = 配置 interval ± 10% jitter；每轮 fire 前 `pong` leader lock（`tryAcquireModelIQProbeLeaderLock`，仿 upstream relay runner），非 leader 不执行。
- `Reload()`：配置保存后重置 timer（下一次按新配置执行）。
- `RunOnce(ctx)`：手动触发入口；进程内 atomic 防重入（进行中则返回进行中错误）；执行目标并发 limit 3（errgroup）。
- 单目标执行：仿 `RunAccountTestBackground`（同包复用 `TestAccountConnectionWithAccount` + `httptest.NewRecorder` + `parseTestSSEOutput`），取 responseText/errMsg/latency → 判分 → 写入结果行；账户不存在/测试异常记 `error` 状态。

### 存储/清理

- `backend/internal/repository/model_iq_probe_repo.go`：`Insert`、`ListPage`（过滤 model/account_id/effective status，按 created_at desc 分页）、`ListRecentForSummary`（近 30 天按 created_at desc LIMIT 500，service 层按 model 分组取最新 + 最近 10 次）、`OverrideStatus`、`DeleteBefore`（30 天）。
- 清理挂钩：`ops_cleanup_service.go` 的 `runCleanupOnce` 末尾（channel monitor 维护旁）调用 `modelIQProbeService.CleanupOldResults(ctx)`。

## API

- 用户端（`routes/user.go`，紧邻 `gpt-intelligence`；`channel_monitor_user_handler.go` 或新 handler 文件）：
  - `GET /api/v1/channel-monitors/iq-probe` → `{items: [{model, latest: {status, answer_excerpt(<=120字), checked_at}, recent: [{status, checked_at}]}]}`；禁用或无数据 → `{items: []}`；不含任何账号信息。
- 管理端（`routes/admin.go`，紧跟 channel-monitors 组；新 `handler/admin/model_iq_probe_handler.go`）：
  - `GET /api/v1/admin/model-iq-probe/config`
  - `PUT /api/v1/admin/model-iq-probe/config`（保存 + runner.Reload）
  - `GET /api/v1/admin/model-iq-probe/results?model=&account_id=&status=&page=&page_size=`（邮箱 `maskEmail` 脱敏输出）
  - `POST /api/v1/admin/model-iq-probe/run`（可选 body 过滤 account_id/model；调用 RunOnce）
  - `POST /api/v1/admin/model-iq-probe/results/:id/override`（body: status ∈ pass/wrong_answer/error/null）
- handler 错误统一 `response.ErrorFrom`；错误码前缀 `MODEL_IQ_*`。

## wire 注册

- `service/wire.go` 新 Provider（构造 service + repo + runner，`runner.Start()`），`cmd/server/wire_gen.go` 重新生成或手工同步；`cmd/server/wire.go` cleanup 列表挂 `runner.Stop()`。

## 前端

### 用户端

- `frontend/src/api/modelIqProbe.ts`：类型 `ModelIqProbeSummaryItem`/`ModelIqProbeStatusPoint` + `fetchModelIqProbeSummary()`。
- `frontend/src/components/user/monitor/CandyProbePanel.vue`：
  - 外壳对齐 `GptIntelligencePanel` 的 section 卡（`mb-5 overflow-hidden rounded-2xl border ...`）。
  - 头部：标题"糖果探针 · 降智检测"+ 汇总徽章（如 `3/4 正常`）。
  - 主体：每模型一行 —— 模型名（mono）、生效判定徽章（✓ 正常 / ✗ 疑似降智 / ⚠ 探测失败 / ○ 未探测）、实际答案摘要（截断）、相对时间、最近 10 次圆点时间线（绿 pass / 红 wrong_answer / 灰 error，样式对齐 MonitorCard 时间线）。
  - props：`items/loading/error`；`items` 为空且非 loading 时不渲染（由父视图 v-if 控制）。
- `ChannelStatusV1View.vue`：新增 `candyProbeItems` 状态 + `reloadCandyProbe`（AbortController 模式并入 `reloadAll`）；模板插在 MonitorCardGrid 与 GptIntelligencePanel 之间。

### 管理端

- 路由 `router/index.ts`：`/admin/channels/iq-monitor`（name `AdminModelIqMonitor`，titleKey `nav.modelIqMonitor`），紧挨 AdminChannelMonitor。
- 侧边栏 `AppSidebar.vue`：渠道管理分组内、渠道监控之后新增条目（heroicon 选用现有 import 风格）。
- `frontend/src/api/admin/modelIqProbe.ts` + 注册 `adminAPI`；类型就近放模块内。
- `frontend/src/views/admin/ModelIqMonitorView.vue` 双 Tab：
  - 探测配置：启用 Toggle、目标模式 Select（账号×模型 / 仅按模型）、模型列表输入（逗号分隔，空=自动发现）、账号多选（仅 account_model 显示，数据源 adminAPI 账号列表）、间隔（分钟）、问题 textarea、期望答案 input、保存。
  - 探测结果：筛选（模型/状态/账号）+ DataTable（时间、账号[脱敏]、模型、生效判定徽章、实际回答摘要→弹窗全文、耗时、tokens[NULL→—]、改判操作）+ 分页 + "立即探测" + `useAutoRefresh` 自动刷新。
- i18n：`nav.modelIqMonitor`（zh/en common.ts nav 块）、`channelStatus.candyProbe.*`（zh/en custom.ts overlay）、`admin.modelIqMonitor.*`（zh/en admin 模块，跟随 admin.channelMonitor 所在文件组织）。

## 测试

- 后端单测：`validateModelIQAnswer`（"……所以最少取出 21 个"→pass；"答案是 20"→wrong；"不是 21 而是 20"→pass[选定语义，记录为已知宽边]；期望"abc"包含匹配；空输入）、目标解析（过滤/去重/确定性选择/自动发现）、配置校验。
- 后端集成（repo，testcontainers 风格随现有 repo 测试）：insert/select 列数与顺序一致断言、ListPage 过滤、Override、DeleteBefore。
- 前端：CandyProbePanel 渲染/空态/徽章映射 spec；API 模块 parse spec（仿 gptIntelligence.spec.ts）。
