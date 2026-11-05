# Langfuse 观测层（独立机器）

与业务服务器物理隔离的观测层，接收业务侧 `observability.langfuse` 异步上报的数据。

默认只有元数据（userId/sessionId 检索键、用量、实扣费用、状态）。业务侧把
`capture_prompt` / `capture_response` 打开后，提示词与响应 assistant 文本会额外进入
trace/generation 的 `input`/`output` 字段，并随事件对象落外部 S3。

trace / observation 落 ClickHouse，原始事件对象落外部 S3（S3 兼容存储），
本地不保留对象副本。

## 组件

| 服务 | 作用 | 暴露 |
|---|---|---|
| langfuse-web | UI + ingestion API | `127.0.0.1:3000` |
| langfuse-worker | 异步处理 ingestion 队列 | 内部 |
| postgres | 元数据 | 内部 |
| clickhouse | trace/observation 存储 | 内部 |
| redis | 队列 | 内部 |

`langfuse-web` 只绑 `${LANGFUSE_BIND_IP}:3000`（docker 网关），由宿主上的反向代理
按域名转发，不对公网直接监听。

## 启动

```bash
cp .env.example .env && vi .env   # 填入随机值
docker compose up -d
curl -fsS http://127.0.0.1:3000/api/public/health   # {"status":"OK","version":"..."}
```

首次启动用 `LANGFUSE_INIT_*` 自动建好 org / project / 登录用户，不需要手工在 UI 建。

## S3 前置

`S3_BUCKET` 指定的桶需已存在且密钥对其可读写；前缀由编排固定为：

```
langfuse/events/<projectId>/<trace|observation>/<id>/<event>.json
langfuse/media/
```

桶不需要公共读，Langfuse 全程用签名请求。

## 反向代理与证书

宿主 Caddy 增加独立站点块（不走 dsh 的 forward_auth，观测层自带账号体系）：

```caddyfile
infuse.dshvip.site {
	tls {
		issuer acme {
			disable_tlsalpn_challenge
		}
	}
	encode zstd gzip
	reverse_proxy 172.19.0.1:3000
}
```

`disable_tlsalpn_challenge` 是因为域名在 Cloudflare 代理后面，443 终止在 CF，
源站只能走 HTTP-01。签发结果落在 Caddy 的 `acme-v02.api.letsencrypt.org-directory/<域名>`。

## 业务侧对接

业务仓库 `deploy/config.example.yaml` 的 `observability.langfuse`：

```yaml
observability:
  langfuse:
    enabled: true
    host: https://infuse.dshvip.site
    public_key: pk-lf-...
    secret_key: sk-lf-...
```

关闭状态（`enabled: false` 或密钥缺失）下业务侧不建队列、不起 goroutine、零出网。

## 一轮对话一张卡（trace 粒度）

Langfuse 里 trace 是「一次工作单元」，不是「一次模型调用」。Codex / Claude Code 这类
agentic 客户端的一次提问会拆成几十次上游调用（读文件、跑命令、改代码），所以正确的
映射是：

| Langfuse 层 | 对应什么 | 承载什么 |
|---|---|---|
| Session | 一个客户端会话 | 聚合键，跨轮稳定 |
| **Trace（一轮）** | **用户问了一次** | `input` = 该轮用户消息；`output` = 该轮最后一条**非空**回答；`timestamp` = 该轮首步时间 |
| Generation（一步） | 一次上游调用 | 模型、用量、实扣费用、耗时、**这一步自己的输出** |

改造前后（最近 300 条真实 trace 实测）：

| 指标 | 每调用一张卡 | 一轮一张卡 |
|---|---|---|
| 300 次调用产生卡片 | 300 | **37** |
| `output` 为空 | 78% | 19%（确实是纯工具轮） |
| `input` 与前面某条完全相同 | 87% | 0%（每轮只有一条） |

### 前置条件

**`capture_prompt: true` 是「一轮一张卡」的必要条件。** 轮次身份取自用户消息条数，
它由请求体解析得出；关掉 `capture_prompt` 时解析不发生，`user_prompt_turns` 恒为 0，
卡片会静默退化成「一次上游调用一张」——也就是这次改造前的样子。进程启动时会为这种
组合打一条警告。

刻意不做「解析但不留正文」的半开状态：那要让每个请求白付一次完整 JSON 解析
（实测 2.9MB 请求体约 8ms、11MB 临时分配），换来的却是没有正文的空卡片。
宁可明确退化，也不假装聚合。

退化时的表现：卡片退到请求级键（同一步的重试仍会合并），metadata 带
`turn_signature_fallback: true`，一眼能看出这张卡没认出轮次。

### 轮次怎么识别

客户端不发「本轮结束」信号，所以轮次身份只能从请求里推：

- **线程**：`sha1(身份 + 首条用户消息首行)`，跳过 `<environment_context>` 这类注入块；
- **轮次**：**用户消息条数**。agentic 客户端在一轮里只让用户开口一次，后续步骤全是
  工具结果，条数恒定；用户再开口才 +1。实测同一线程内该值稳定（3,3,3 / 14,14,14）。
- **注意**：不能用「条数是否前进」判断轮次前进。同一线程里并发的子代理会让条数在
  3 和 14 之间来回跳，按「前进即关闭」会把卡片反复切碎。条数只做**键**，关闭只看空闲。
- **收尾**：该轮最后一次步骤之后空闲 `turn_idle_seconds`（默认 300s）落卡；进程退出时
  由 `langfuse.Shutdown` 全量落卡。
- **回答取值**：取该轮**最后一条非空**文本，不是「最后一步」—— 实测 38 个真实轮次里
  有 20 个以纯工具调用收尾，按最后一步取会让一半卡片没有输出。

### 会话内定位字段

| 字段 | 位置 | 含义 |
|---|---|---|
| `sessionId` | trace 顶层 | 会话聚合键。客户端给了 `session_id` / `X-Session-Id` / `X-Claude-Code-Session-Id` 就用它；没给时用 `auto-<hash>-<首次出现时间>` 推导 |
| `tags` | trace 顶层 | `thread:th-<hash>`，用于按线程过滤 |
| `session_source` | metadata | `client` / `auto` / absent（无法归组，与历史行为一致） |
| `session_title` | metadata | 会话标题，取首条真实用户消息首行 |
| `thread_id` | metadata | 会话内的逻辑线程：同一 sessionId 下起点不同的对话流各自一条 |
| `turn_index` | metadata | 线程内第几轮（= 用户消息条数） |
| `turn_steps` | metadata（卡片） | 这一轮一共几步上游调用 |
| `thread_step` | metadata（步骤） | **轮内**第几步上游调用，从 1 开始 |
| `step_attempt` | metadata（步骤） | 同一步的第几次尝试；`>1` 表示客户端重试/并行重发 |
| `turn_cost` | metadata（卡片） | 该轮各步实扣合计 |

归组在 worker 侧完成（单消费者串行），业务同步路径仍只有一次 channel send；状态表是
纯内存的有界表（线程 TTL 30 分钟、轮次空闲 5 分钟、各 4096 条），不产生 I/O、不写日志、不落盘。

### 顺序与代价

步骤的 generation 先发、卡片后发，是刻意的：步骤要立刻可见，卡片等这一轮结束。
Langfuse 对未知 trace 的 generation 会先暂存、trace 到达后自动挂上（observations 计数
与总费用都对得上，已用可删除探针验证）。

已知代价，说清楚：

1. **卡片延迟**：该轮最后一次步骤之后最多 `turn_idle_seconds` 才出现。Langfuse v4 的
   Events 视图本身还有约 15 分钟物化延迟，实际感知不到。
2. **进程崩溃 / 重启**：仍在进行中的那一轮，其 generation 已发出但卡片没了 → 这些
   generation 在 UI 上不可见。进程正常退出时会把它们补发（先发卡片、再清队列；
   退出预算 4s），但如果队列里还有没处理完的步骤，那一轮会被合理地裂成两张卡。
   观测是 best-effort，与「队列满即丢」一致。
3. **超长静默**：一轮中间静默超过阈值会被切成两张卡（300s 下实测 1/38）。
4. **并发子代理**：`(线程, 条数)` 相同才会合并；实测交错的两路子代理条数不同，不会撞。
5. **历史上限**：同时未收尾的轮次超过 2048 时，强制落最老的一轮（可能一分为二）。
6. **不做历史迁移**：旧卡是步骤级、新卡是轮级，切换后同一会话里会短暂混排。

## 正文采集范围（input / output 有多完整）

| 值 | 来源 | 缺省上限 | 说明 |
|---|---|---|---|
| trace `input` | **最新一条**用户消息（跳过 system/developer、工具结果、审批块） | 卡片侧 32KB | 超限保留尾部并加 `…（前文已省略）` |
| trace `output` | 该轮**最后一条非空** assistant 正文 | 卡片侧 64KB | 超限打 `turn_output_truncated` |
| generation `output` | 这一步自己的 assistant 正文 | `capture_max_bytes` = 256KB | 超限打 `output_truncated` |
| generation `input` | 不发 | — | 同一轮每步的用户消息完全相同，重复发几十份正是要修的冗余 |
| `metadata.request_body` | 原始请求体样本，**每轮只存首步一份** | 8KB | 取证抽样，不是全文 |

长会话的输入处理：每条上游调用都会重发完整历史，如果把整段历史塞进 `input`，
真正被回答的那句会被几十轮上下文淹没。所以 `input` 只放最新一条用户消息，
历史规模用 `metadata.user_prompt_history_bytes` 表达，轮次数用
`metadata.user_prompt_turns`。Codex 会把工具审批块（`>>> APPROVAL REQUEST`）写进
user 消息，这类块不算用户原话，会被跳过。

硬顶：`capture_max_bytes` / `capture_prompt_max_bytes` 的单侧上限都是 2MB；单个
generation 事件正文最多 1MB，单张卡片最多约 100KB。Langfuse ingestion 单请求上限是
4.5MB（实测 8MB 返回 `413 Body exceeded 4.5mb limit`），上报侧再按 3MB 预算切包。
开启采集后队列条数会按 128MB 正文字节预算自动收紧，避免大上限把内存吃穿。

## 输出完整性标记

正文采集有上限（`observability.langfuse.capture_max_bytes`，默认 256KB），超过就只留前 256KB。
为了事后能判断"这段回答是不是被切了"，metadata 里带这几个字段：

| 字段 | 位置 | 含义 |
|---|---|---|
| `output_chars` | generation | 响应正文字符数（rune），和客户端看到的字数直接对得上 |
| `finish_reason` | generation | 上游收尾原因：`stop` / `length` / `tool_calls` / `incomplete:max_output_tokens`（responses）/ `end_turn`（anthropic） |
| `output_truncated` | generation | **只在正文触顶时出现**，表示这一步的回答被截断 |
| `raw_stream_truncated` | generation | 原始 SSE 字节被截（流里 `response.created` 这类大 JSON 很容易超限），**不代表正文缺失** |
| `turn_output_truncated` | trace | 卡片上的回答被 64KB 上限截断 |
| `turn_input_truncated` | trace | 卡片上的用户消息被 32KB 上限截断 |
| `turn_input_missing` | trace | 这一轮没抽到用户文本（纯工具请求） |
| `turn_no_answer` 占位 | trace `output` | 该轮确实没有 assistant 正文时，`output` 写「〔本轮无文本回答，仅工具调用/未产出〕」，**不是上报失败** |

判读：`finish_reason=length` 是模型自己停的；`output_truncated=true` 才是观测层切掉的。
两者都没有而回答明显短，说明这一轮本来就没有 assistant 正文（纯工具调用步）。

## 读历史数据

切换前后的卡片粒度不同：旧卡是「每一步一张」，`name` 形如 `sub2api.model-call · 7d0b · #12`；
新卡是「每一轮一张」，`name` 形如 `sub2api.turn · 7d0b · t3`。按 `name` 前缀就能分开，
不做历史迁移（重写已有 trace 没有增量价值）。

## 验证

### 一、聚合语义探针（可删除，几秒钟）

验证「一步一条 generation、一轮一条 trace」在真实 Langfuse 上的最终渲染：先发 3 条
generation（此时 trace 还不存在），再补 1 条卡片，然后回读断言 observations 数、
input/output、总费用、时间戳。跑完自己 `DELETE` 掉。

```bash
LF_HOST=https://infuse.dshvip.site \
LF_PUBLIC_KEY=pk-lf-... LF_SECRET_KEY=sk-lf-... \
    python3 deploy/langfuse/probe-turn-card.py
```

判读：全 ✓ 才算链路成立，否则说明 Langfuse 侧对「generation 先于 trace」的暂存行为变了。

### 二、客户端实机联调

```bash
# 真实 Client → 真实 Langfuse → 检索回读
LF_HOST=http://127.0.0.1:3000 \
LF_PUBLIC_KEY=pk-lf-... LF_SECRET_KEY=sk-lf-... \
go test -tags langfuselive ./internal/observability/langfuse/ -run TestLiveIngestionReachesLangfuse -v
```

## 版本与升级（v3 → v4）

实测结论：v4 可以沿用现有客户端，但有两个硬前提。

| 项 | 要求 | 说明 |
|---|---|---|
| ClickHouse | ≥ 26.x | 迁移 `0039_create_events_full` 使用全文索引 `TYPE text(tokenizer = splitByNonAlpha)`，24.12/25.8 解析失败：`Code: 80 INCORRECT_QUERY`，随后 `Dirty database version 39` 让 web 反复重启 |
| 写模式 | `LANGFUSE_MIGRATION_V4_WRITE_MODE=dual` | 默认的 `events_only` 会停用 `POST /api/public/ingestion`，`/api/public/traces` 直接返回 "not available ... events_only mode"；只设 `legacy` 会让 worker 崩：`NATIVE_OTEL_BEHAVIOUR=direct requires WRITE_MODE in {dual, events_only}` |

`dual` 同时写 legacy 表与 `events_full`：legacy ingestion 继续可用，存储占用约翻倍。
v4 的方向是 OTel（`POST /api/public/otel/v1/traces`），切过去要改客户端，属独立改动。

### 升级步骤

```bash
# 1. 备份（v4 会迁移 ClickHouse schema，旧版本读不回新 schema）
docker exec langfuse-postgres-1 pg_dumpall -U postgres > backups/pg-$(date +%F).sql
docker run --rm -v langfuse_ch_data:/data:ro -v "$PWD/backups":/backup alpine \
  tar czf /backup/ch-$(date +%F).tgz -C /data .

# 2. 改编排：langfuse :3 → :4（web + worker）、clickhouse 24.12 → 26.10、
#    追加 LANGFUSE_MIGRATION_V4_WRITE_MODE=dual

# 3. 重建（数据只有探针时可丢弃 ClickHouse 卷，走已验证的全新迁移路径）
docker compose down && docker volume rm langfuse_ch_data langfuse_ch_logs
docker compose up -d
```

### 验收判据

```bash
curl -sS http://<网关IP>:3000/api/public/health        # 期望 4.x
LF_HOST=https://<域名> LF_PUBLIC_KEY=... LF_SECRET_KEY=... \
  go test -tags langfuselive ./internal/observability/langfuse/ \
  -run TestLiveIngestionReachesLangfuse -v              # trace/cost/正文全绿
```

v4 的 trace 级 `totalCost` 由聚合侧异步补齐，晚于 generation 落库；探针对它做了 45s 轮询。

### 回滚

镜像退回 `:3` 并**用备份恢复 ClickHouse 卷**——v4 迁移过 schema，旧版本无法直接读取。
