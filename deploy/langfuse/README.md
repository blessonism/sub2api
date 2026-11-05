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
