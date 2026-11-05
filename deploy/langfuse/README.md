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

## 会话聚合（同一会话的多次请求）

一次对话在业务侧是「多条上游请求」，在 Langfuse 里默认也是「多条 trace」。业务侧在
ingestion 时把这批 trace 归到同一个 `sessionId`，并配上会话内的结构字段，让 session
视图能读成一条有序时间线。

| 字段 | 位置 | 含义 |
|---|---|---|
| `sessionId` | trace 顶层 | 会话聚合键。客户端给了 `session_id` / `X-Session-Id` / `X-Claude-Code-Session-Id` 就用它；没给时用 `auto-<hash>-<首次出现时间>` 推导 |
| `session_source` | metadata | `client` / `auto` / absent（无法归组，与历史行为一致） |
| `session_title` | metadata | 会话标题，取首条真实用户消息首行（跳过 `<environment_context>` 这类客户端注入块） |
| `thread_id` | metadata | 会话内的逻辑线程：同一 sessionId 下起点不同的对话流各自一条 |
| `thread_step` | metadata | 线程内第几次上游请求，从 1 开始 |
| `step_attempt` | metadata | 同一步的第几次尝试；`>1` 表示客户端重试/并行重发，真实对话只读一次 |

归组在 worker 侧完成（单消费者串行），业务同步路径仍只有一次 channel send；状态表是
纯内存的有界表（TTL 30 分钟、上限 4096 条），不产生 I/O、不写日志、不落盘。

读法：先按 `sessionId` 进 session，再按 `thread_id` 分成几条流，每条流按 `thread_step`
排序，`step_attempt=1` 的那条就是这个会话真正走过的一步。

已知边界：客户端不带会话头、且提示词采集关闭时无法归组；同一用户用完全相同的开场白
在 30 分钟内开两个会话会被并成一个（锚点相同即视为同一会话）。

## 正文采集范围（input / output 有多完整）

| 值 | 来源 | 缺省上限 | 说明 |
|---|---|---|---|
| `input` | **最新一条**用户消息（跳过 system/developer、工具结果、审批块） | `capture_prompt_max_bytes` = 512KB | 超限保留尾部并加 `…（前文已省略）` |
| `output` | assistant 正文 | `capture_max_bytes` = 256KB | 超限保留头部并打 `output_truncated` |
| `metadata.request_body` | 原始请求体样本 | 8KB | 只作取证抽样，不是全文 |

长会话的输入处理：每条上游调用都会重发完整历史，如果把整段历史塞进 `input`，
真正被回答的那句会被几十轮上下文淹没。所以 `input` 只放最新一条用户消息，
历史规模用 `metadata.user_prompt_history_bytes` 表达，轮次数用
`metadata.user_prompt_turns`。Codex 会把工具审批块（`>>> APPROVAL REQUEST`）写进
user 消息，这类块不算用户原话，会被跳过。

硬顶：任何一侧最多 2MB，单条调用正文合计最多 2MB —— 因为一条调用会展开成
trace + generation 两条事件、正文要发两遍，而 Langfuse ingestion 单请求上限是
4.5MB（实测 8MB 返回 `413 Body exceeded 4.5mb limit`）。开启采集后队列条数会按
128MB 正文字节预算自动收紧，避免大上限把内存吃穿。

## 输出完整性标记

正文采集有上限（`observability.langfuse.capture_max_bytes`，默认 64KB），超过就只留前 64KB。
为了事后能判断"这段回答是不是被切了"，trace/generation 的 metadata 里带四个字段：

| 字段 | 含义 |
|---|---|
| `output_chars` | 响应正文字符数（rune），和客户端看到的字数直接对得上 |
| `finish_reason` | 上游收尾原因：`stop` / `length` / `tool_calls` / `incomplete:max_output_tokens`（responses）/ `end_turn`（anthropic） |
| `output_truncated` | **只在正文触顶时出现**，表示回答被截断 |
| `raw_stream_truncated` | 原始 SSE 字节被截（流里 `response.created` 这类大 JSON 很容易超限），**不代表正文缺失** |

判读：`finish_reason=length` 是模型自己停的；`output_truncated=true` 才是观测层切掉的。
两者都没有而回答明显短，说明这一轮本来就没有 assistant 正文（纯工具调用步）。

## 验证

```bash
# 实机联调：真实 Client → 真实 Langfuse → 检索回读
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
