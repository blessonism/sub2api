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
