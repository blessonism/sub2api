# LLM API 网关 / 中转站：请求响应捕获与可观测性事实清单（核查日 2026-11-05）

核查方式：`web_search` 通道当日不可用（provider 402），全部事实来自官方文档 / 官方源码仓库 / PyPI / GitHub API 的直接抓取，仓库均以 `git clone --depth 1` 取 HEAD 后 grep 原文。每条事实后附 URL 或仓库内文件路径。

---

## 1. Langfuse

### 版本与部署
- 最新主版本 **v4**，最新发布 **v4.76.0**（2026-11-05T18:08Z）。3.x 仍在出补丁版：**v3.227.0**（2026-11-03）。仓库 tag 主版本只有 v1/v2/v3/v4（tag 数 165/328/366/118），**不存在 v5**。
  来源：https://github.com/langfuse/langfuse/releases ；`git ls-remote --tags`
- HEAD `2dd42e9`（2026-11-05）。部署形态：自托管（Docker Compose / Helm / K8s / Terraform AWS·Azure·GCP）+ Cloud（EU/US/JP/HIPAA 区域）。
  来源：https://langfuse.com/llms.txt ；https://langfuse.com/self-hosting
- 许可：`ee/`、`web/src/ee/`、`worker/src/ee/`、`packages/shared/src/server/ee/` 之外为 **MIT Expat**，`ee/` 单独许可。官方表述为 "MIT-licensed core features and paid Enterprise additions"。
  来源：仓库 `LICENSE`；https://langfuse.com/pricing-self-host

### legacy ingestion 单请求体积上限
- `POST /api/public/ingestion` 走 Next.js bodyParser，硬编码 **`sizeLimit: "4.5mb"`**（≈4.5 MB/请求）。
  来源：`web/src/pages/api/public/ingestion.ts`（`export const config.api.bodyParser.sizeLimit = "4.5mb"`）
- 同类 4.5mb 上限：`/api/public/dataset-items`、`/api/public/scores`、`/api/public/mcp`、`/api/public/skills`、`/api/trpc/[trpc]`；`/api/public/feedback` 为 16kb。
  来源：`web/src/pages/api/public/*/index.ts`、`web/src/pages/api/trpc/[trpc].ts`

### v4 的 OTel endpoint 成熟度
- OTel 入口：`POST /api/public/otel`（OTLP/HTTP，**支持 HTTP/JSON 与 HTTP/protobuf，gRPC 尚不支持**）；signal-specific 路径 `/api/public/otel/v1/traces`；另有 `/api/public/otel/v1/metrics`。
- 自托管本地端点从 **≥ v3.22.0** 起可用。
- 鉴权：Basic Auth（`pk-lf-…:sk-lf-…` base64）。
- **关键**：直接发 OTel spans 时必须带 header `x-langfuse-ingestion-version: 4`，才会实时进入 v4 数据模型与 v2 Observations/Metrics API；不带该 header 的数据**最多延迟 10 分钟**。`OTEL_EXPORTER_OTLP_TRACES_HEADERS` 也要单独配。
- 定位：OTLP **替代已弃用的 `POST /api/public/ingestion`**；Langfuse 自称 OTel backend，但明确说 "GenAI 语义约定仍在演进"，靠属性映射表兼容。
  来源：https://langfuse.com/integrations/native/opentelemetry ；https://langfuse.com/llms.txt
- 弃用时间表：Langfuse Cloud 于 **2026-11-16** 移除 legacy v3 公共 API；被弃用响应携带 `Deprecation`/`Sunset`/`Link` header 与 `_deprecation` JSON 字段。读路径迁到 `/api/public/v2/observations`、`/v2/metrics`、`/v3/scores`、`/experiments`；写路径迁到 `/api/public/otel/v1/traces`。
  来源：https://langfuse.com/faq/all/deprecated-api-migration.md

### OTel 入口的体积上限（源码实测）
- 该路由 `bodyParser: false`，自带 reader：`LANGFUSE_OTEL_INGESTION_MAX_BODY_BYTES` **默认 512 MiB（536870912）**，同时约束**编码后与解压后**体积；超过即走 `handleOtelRequestBodyTooLarge`（错误路径，非静默截断）。
- 超过 `OTEL_REQUEST_BODY_WARNING_BYTES = 16 MiB` 只记 warn 日志。
  来源：`web/src/pages/api/public/otel/v1/traces/index.ts`；`web/src/server/otel/processOtelIngestion.ts`；`web/src/env.mjs`

### `LANGFUSE_MIGRATION_V4_WRITE_MODE` 现状
- 取值枚举 **`legacy | dual | events_only`**，在 v4 中**默认 `events_only`**（v3 时代默认 `legacy`）；web 与 worker 两侧 env 必须保持一致。
- `events_only`：依赖 legacy traces/observations 表的公共 API 路由返回 404，legacy dataset-item 写入被拒；`dual` 是官方文档给的临时迁移桥（"set LANGFUSE_MIGRATION_V4_WRITE_MODE=dual on both the web and worker services and redeploy"）。
  来源：`packages/shared/src/env.ts`（`LANGFUSE_MIGRATION_V4_WRITE_MODE: z.enum(["legacy","dual","events_only"]).default("events_only")`，注释 "Defaults to events_only for the v4 target state (v3 shipped legacy)"）；`web/src/features/public-api/server/deprecations.ts`；`worker/src/env.ts`

### media / attachment：正文外置到 S3 后只存引用（已支持）
- 机制：SDK 客户端把 base64 data URI 抽出 → 用 presigned URL 直传对象存储 → 正文位置替换为标准 token：
  `@@@langfuseMedia:type={MIME_TYPE}|id={LANGFUSE_MEDIA_ID}|source={SOURCE_TYPE}@@@`，`SOURCE_TYPE ∈ {base64_data_uri, bytes, file}`。
- 内容寻址：`mediaId = url-safe(SHA-256).slice(0,22)`；数据库唯一约束为 `(projectId, sha256Hash)`；**相同内容重复上传直接返回同一 mediaId 且 `uploadUrl: null`**。官方文档："Deduplication: Files are simply replaced by their mediaId reference string if already uploaded"、"File uniqueness determined by project, content type, and content SHA256 hash"。
  来源：`packages/shared/src/server/media/mediaService.ts`（`getMediaId`）；`web/src/features/media/server/mediaService.ts`；https://langfuse.com/docs/observability/features/multi-modality.md
- 单附件上限：`LANGFUSE_S3_MEDIA_MAX_CONTENT_LENGTH` 默认 **1_000_000_000 B（1 GB）**；浏览器端 dataset item 上传额外限制 100 MB。
  来源：https://langfuse.com/self-hosting/deployment/infrastructure/blobstorage ；同上 multi-modality 文档
- 存储配置：`LANGFUSE_S3_MEDIA_UPLOAD_{BUCKET,PREFIX,REGION,ENDPOINT,INTERNAL_ENDPOINT,ACCESS_KEY_ID,SECRET_ACCESS_KEY,FORCE_PATH_STYLE,SSE,...}`；MinIO / SeaweedFS 需 `FORCE_PATH_STYLE`。
- **任意 S3 引用**（不搬数据）：在 `input`/`output`/`metadata` 里写 `s3://<bucket>/<key>`，由项目级 "External Media Storage" 集成解析（支持 S3 与 S3 兼容存储、prefix 限定、浏览器需放开 `GET`/`HEAD` + `Range` 的 CORS）。属 Feature Preview。
  来源：https://langfuse.com/docs/observability/features/multi-modality.md#external-s3-media
- **超长正文字段服务端外置**（真正的"正文外置只存引用"，非仅 media）：worker 的 `applyObservationFieldOverflow`
  - 开关 `LANGFUSE_OBSERVATION_FIELD_OVERFLOW_ENABLED`（**默认 false**）
  - 阈值 `LANGFUSE_OBSERVATION_FIELD_SIZE_LIMIT_BYTES` **默认 2 MiB（2097152 B）**
  - 对 `input`、`output`、`metadata` 各值按 UTF-8 字节数判超限 → 以 `text/plain` 上传到 media bucket → 行内替换为 `@@@langfuseMedia:type=text/plain|id=<mediaId>|source=field_size_limit@@@`
  - 结果分 `uploaded` / `reused` / `failed`；`reused` 即内容寻址去重命中；失败则**保留原字段**落库
  来源：`worker/src/features/observation-field-overflow/processObservationFieldOverflow.ts`；`packages/shared/src/domain/media.ts`（`OBSERVATION_FIELD_SIZE_LIMIT_MEDIA_SOURCE = "field_size_limit"`）；`worker/src/env.ts`

### 新版 gen_ai 语义约定支持
- OTel 处理器已解析 `gen_ai.input.messages`、`gen_ai.output.messages`、`gen_ai.system_instructions`、`gen_ai.tool.definitions`、`gen_ai.request.model`、`gen_ai.usage.input_tokens`；也兼容旧名 `gen_ai.prompt` / `gen_ai.completion`。
- input/output 属性优先级：`langfuse.observation.input` > `gen_ai.prompt` > `input.value` > `mlflow.spanInputs`（output 同理）。
  来源：`packages/shared/src/server/otel/OtelIngestionProcessor.ts`、`ObservationTypeMapper.ts`、`extractToolsBackend.ts`；https://langfuse.com/faq/all/empty-trace-input-and-output
- Langfuse 自家 JS SDK（AI SDK telemetry）本身按 `gen_ai.input.messages` / `gen_ai.output.messages` / `gen_ai.usage.input_tokens` 发射。
  来源：`packages/shared/src/server/llm/ai-sdk/telemetry.ts`

---

## 2. Helicone

### 许可与版本
- 许可：**Apache License 2.0**（仓库根 `LICENSE`）。不是 MIT。
  来源：https://github.com/Helicone/helicone/blob/main/LICENSE
- HEAD `7720d28`（2026-10-28），但**最后一个 release tag 是 `v2025.08.21-1`（2025-08-21）**，其后只有 `deploy-*` 类 tag，主干持续提交。自托管镜像版本因此停在 2025-08 构建。
  来源：https://github.com/Helicone/helicone/releases ；`git ls-remote --tags`

### 部署形态
- `helicone/helicone-all-in-one` 单容器；docker compose 全栈（postgres 17.4、clickhouse 24.3.13.40、minio `pgsty/minio:RELEASE.2026-08-04…`、redis 8.0.2、mailhog）；Kubernetes/Helm；manual；cloud（AWS/GCP/Azure）。
  来源：https://docs.helicone.ai/getting-started/self-host/overview.md ；`docker/docker-compose.yml`

### proxy 模式 vs async logger 模式
- **Proxy**：改 base URL 即接入（`gateway.helicone.ai` 或 provider 专属域如 `oai.helicone.ai`）。可获得网关能力：prompts、bucket cache、retries、custom rate limiting、custom properties、user metrics。在关键路径上。
- **Async**：`@helicone/async` = **Traceloop/OpenLLMetry 打点 + `OTLPTraceExporter` 指向 Helicone baseUrl**，不在关键路径、零传播延迟；但没有 cache/retry/rate limit。
  来源：https://docs.helicone.ai/references/proxy-vs-async.md ；`sdk/typescript/async/async_logger/HeliconeAsyncLogger.ts`

### 单请求体积限制
- **async 日志端点**：`MAX_PAYLOAD_SIZE = 10 * 1024 * 1024`，超出直接 `400 Payload too large`。
  来源：`worker/src/lib/managers/AsyncLogManager.ts`
- **proxy 请求体**：`MAX_INMEMORY_BYTES = 20 * 1024 * 1024`（20 MiB）——已知 Content-Length 且 ≤20 MiB 走内存 buffer；否则走 remote/container buffer（存在 `REQUEST_BODY_BUFFER` 绑定时）；无截断逻辑。
  来源：`worker/src/RequestBodyBuffer/RequestBodyBufferBuilder.ts`
- proxy 机型（Cloudflare Workers）平台侧上限未在官方文档给出明确数值：**未查到**。

### body 存储方式（S3/MinIO + ClickHouse）
- ClickHouse `request_response_rmt` 表**内联存正文**：`request_body String DEFAULT ''`、`response_body String DEFAULT ''`，均带 `TTL request_created_at + INTERVAL 3 MONTH`，并有 `ngrambf_v1(4,1024,1,0)` bloom 索引；引擎 `ReplacingMergeTree(updated_at)`，按月分区。
  来源：`clickhouse/migrations/schema_41_request_response_replacing_merge_tree.sql`
- S3/MinIO 落盘（`S3_ENABLED` 默认 true）：
  - `organizations/{orgId}/requests/{requestId}/raw_request_response_body`
  - `organizations/{orgId}/requests/{requestId}/request_response_body`
  - assets：`organizations/{orgId}/requests/{requestId}/assets/{assetId}`
  - prompt body：`organizations/{orgId}/prompts/{promptId}/versions/{promptVersionId}/prompt_body`
  来源：`valhalla/jawn/src/lib/shared/db/s3Client.ts`（`getRawRequestResponseKey`/`getRequestResponseKey`/`getRequestResponseImageKey`/`getPromptKey`）
- compose 里的 MinIO bucket：`request-response-storage`、`prompt-body-storage`、`hql-store`。
  来源：`docker/docker-compose.yml`

### 是否做 request body 去重
- **不做**。S3 key 以 `requestId` 为一级路径，ClickHouse 每请求一行；全仓 grep `dedup/contentHash/sha256`，唯一 sha256 命中是 worker 里 AWS SigV4 签名（`@aws-crypto/sha256-js`）。无内容寻址或跨请求复用。
  来源：`valhalla/jawn/src/lib/shared/db/s3Client.ts`；`worker/src/RequestBodyBuffer/RequestBodyBufferInMemory.ts`

### OTel 支持
- 不是通用 OTLP backend。Async 路径接受 OpenLLMetry 产生的 OTel spans（`OTLPTraceExporter` → Helicone），jawn 侧解析 `opentelemetry.proto.trace.v1.TracesData`；路由在 `/v1/trace/*`（`custom/log`、`custom/v1/log`、`custom/log/typed`）。
  来源：`valhalla/jawn/src/controllers/public/traceController.ts`；`valhalla/jawn/src/managers/traceManager.ts`

---

## 3. Arize Phoenix

- 版本：**arize-phoenix 21.0.0**（PyPI 最新；`arize-phoenix-v21.0.0` 发布 2026-10-30）；`arize-phoenix-client 3.7.0`（2026-11-05）；仓库 HEAD `7eb74a7`（2026-11-05）。
  来源：https://pypi.org/pypi/arize-phoenix/json ；https://github.com/Arize-ai/phoenix/releases
- 许可：**Elastic License 2.0（ELv2）**，不是 OSI 开源许可；客户端包（`arize-phoenix-client`）为 Apache-2.0。
  来源：仓库 `LICENSE`（"Elastic License 2.0 (ELv2)"）；https://pypi.org/pypi/arize-phoenix-client/json
- 自托管：支持。仓库内提供 `docker-compose.yml`、`helm/`、`kustomize/`、`azuredeploy.json`、`render.yaml`；文档有 local/container/self-hosted 三种起法。
  来源：https://arize.com/docs/phoenix/get-started ；仓库目录结构
- OpenInference 语义约定：依赖 `openinference-semantic-conventions>=0.1.40`；该包最新 **0.1.43**（PyPI，Apache-2.0）。OpenInference 是 OTel 的**扩展**约定（`openinference.span.kind`、`input.value`/`output.value`、`llm.*`），不是 OTel 官方 `gen_ai.*`。
  来源：`pyproject.toml`；https://pypi.org/pypi/openinference-semantic-conventions/json
- 摄取入口与 payload 上限：
  - HTTP：`POST /v1/traces` **只接受 `application/x-protobuf`**（其它 content-type 直接 415）；`content-encoding` 支持 gzip；`x-project-name` header 选项目。
  - gRPC：独立端口上的 `TraceService/Export`。
  - gRPC server 仅设置 `("grpc.so_reuseport", 0)`，**未覆盖 gRPC 默认单消息上限 4 MiB**（`grpc.max_receive_message_length` 缺省）。
  - 无 body-size 环境变量；摄取路径无属性值截断逻辑。
  - 背压不靠体积而靠队列：`PHOENIX_MAX_SPANS_QUEUE_SIZE` 默认 **20 000**，队列满返回 **503 "Server is at capacity"**（`is_not_at_capacity` 依赖注入）；另有磁盘不足时 gRPC `RESOURCE_EXHAUSTED`。
  来源：`src/phoenix/server/api/routers/v1/traces.py`、`grpc_server.py`、`api/routers/v1/utils.py`、`server/app.py`、`config.py`
- 文档索引（可程序化抓取）：https://arize.com/docs/phoenix/llms.txt

---

## 4. OpenTelemetry GenAI semantic conventions

### 归属与稳定度
- GenAI 约定已从 `open-telemetry/semantic-conventions` **迁出**到独立仓库 `open-telemetry/semantic-conventions-genai`；opentelemetry.io 上的 gen-ai 页只剩跳转说明。核心 semconv 版本 **1.45.0**。
  来源：https://opentelemetry.io/docs/specs/semconv/gen-ai/ ；https://github.com/open-telemetry/semantic-conventions-genai
- 稳定度：**Development**（旧称 Experimental）。`docs/gen-ai/gen-ai-spans.md` 顶部 `**Status**: [Development]`；`docs/registry/attributes/gen-ai.md` 里**全部 `gen_ai.*` 属性都是 Development badge，没有任何 Stable 的 gen_ai 属性**。
  来源：`docs/gen-ai/gen-ai-spans.md`、`docs/registry/attributes/gen-ai.md`

### gen_ai.* 属性清单（registry 实际条目，均为 Development）
- 请求：`gen_ai.operation.name`、`gen_ai.provider.name`、`gen_ai.request.model`、`gen_ai.request.max_tokens`、`gen_ai.request.temperature`、`gen_ai.request.top_p`、`gen_ai.request.top_k`、`gen_ai.request.stop_sequences`、`gen_ai.request.seed`、`gen_ai.request.stream`、`gen_ai.request.stream_cursor`、`gen_ai.request.choice.count`、`gen_ai.request.frequency_penalty`、`gen_ai.request.presence_penalty`、`gen_ai.request.encoding_formats`、`gen_ai.request.reasoning.level`、`gen_ai.request.previous_response.id`、`gen_ai.output.type`
- 响应：`gen_ai.response.id`、`gen_ai.response.model`、`gen_ai.response.finish_reasons`、`gen_ai.response.status`、`gen_ai.response.time_to_first_chunk`
- 内容（Opt-In）：`gen_ai.system_instructions`、`gen_ai.input.messages`、`gen_ai.output.messages`、`gen_ai.tool.definitions`；外置引用：`gen_ai.system_instructions_ref`、`gen_ai.input.messages_ref`、`gen_ai.output.messages_ref`、`gen_ai.tool.definitions_ref`
- 工具：`gen_ai.tool.name`、`gen_ai.tool.type`、`gen_ai.tool.description`、`gen_ai.tool.call.id`、`gen_ai.tool.call.arguments`、`gen_ai.tool.call.result`
- 用量：`gen_ai.usage.input_tokens`、`gen_ai.usage.output_tokens`、`gen_ai.usage.cache_read.input_tokens`、`gen_ai.usage.cache_write.input_tokens`，以及按模态细分 `gen_ai.usage.{text|image|audio}.{input|output|cache_read}.tokens`、`gen_ai.token.modality`
- 会话/记忆/检索：`gen_ai.conversation.id`、`gen_ai.conversation.compacted`、`gen_ai.memory.query.text`、`gen_ai.memory.records`、`gen_ai.memory.record.id`、`gen_ai.memory.record.count`、`gen_ai.memory.store.id`、`gen_ai.retrieval.query.text`、`gen_ai.retrieval.documents`、`gen_ai.retrieval.top_k`
- prompt：`gen_ai.prompt.name`、`gen_ai.prompt.version`、`gen_ai.prompt.variable.<key>`
- 注：`gen_ai.system` 已不在新 registry（LiteLLM 仍与 `gen_ai.provider.name` 并存发射）。
  来源：`docs/registry/attributes/gen-ai.md`

### 内容捕获开关（有，且是强制命名）
- 原则："OpenTelemetry instrumentations SHOULD NOT capture them by default, but SHOULD provide an option for users to opt in."
- 环境变量**必须**命名为 `OTEL_INSTRUMENTATION_GENAI_CAPTURE_CONTENT`，取值：
  `NO_CONTENT`（默认）/ `SPAN_ONLY` / `EVENT_ONLY` / `SPAN_AND_EVENT`；大小写不敏感；非法值回落 `NO_CONTENT`。
- 受该开关管辖的属性：`gen_ai.input.messages`、`gen_ai.output.messages`、`gen_ai.system_instructions`、`gen_ai.tool.call.arguments`、`gen_ai.tool.call.result`、`gen_ai.retrieval.query.text`、`gen_ai.prompt.variable.*`、`gen_ai.memory.query.text`、`gen_ai.memory.records`；**`gen_ai.tool.definitions` 不受管辖**（属于 schema 而非 payload）。
- 截断约定：无规范性阈值。原文 "Instrumentation MAY provide a configuration option allowing to truncate properties such as individual message contents, preserving JSON structure"。
  来源：`docs/gen-ai/gen-ai-spans.md` §"Recording content on attributes"、"Content capture configuration"

### 正文外置 + 只存引用（规范已定义）
- 三条路："[Default] 不记录" / "记录在 span 属性上" / "存到外部存储、在 span 上只记引用"（明确推荐用于生产）。
- 引用属性：`gen_ai.input.messages_ref`、`gen_ai.output.messages_ref`、`gen_ai.system_instructions_ref`、`gen_ai.tool.definitions_ref`，类型为 **absolute URI**；规范**不规定** URI scheme、对象布局、序列化格式、保留策略与访问方式。
- 上传 hook 的标准化配置：
  - `OTEL_INSTRUMENTATION_GENAI_CONTENT_HOOK`（如 `upload`）
  - `OTEL_INSTRUMENTATION_GENAI_UPLOAD_BASE_PATH`（本地目录或 `s3://…`、`gs://…`、`az://…`）
  - `OTEL_INSTRUMENTATION_GENAI_UPLOAD_FORMAT` = `json` | `jsonl`（默认 `json`）
  - `OTEL_INSTRUMENTATION_GENAI_UPLOAD_MAX_QUEUE_SIZE` 默认 `20`
  - hook 应在**采样决策之外**调用；hook 可改写内容，之后再写属性
- 去重：**"Implementations MAY use content-derived identifiers to deduplicate repeated content, such as system instructions or tool definitions."** registry 中的 `*_ref` 示例 URI 直接给出 64 位十六进制摘要命名的对象（如 `s3://example-bucket/gen-ai/e6856247…d169_system_instruction.json`）。
  来源：`docs/gen-ai/gen-ai-spans.md` §"Uploading content to external storage"、"Content upload configuration"；`docs/registry/attributes/gen-ai.md`

### OTLP 单请求体积限制
- 规范：服务端 **MUST** 强制消息大小上限；"server implementations typically enforce a default incoming message size limit of **4 MiB**. However, it is RECOMMENDED to use **64 MiB** as the default limit"；客户端同样 SHOULD 限制（含压缩前），推荐 64 MiB 默认；超限客户端 MUST NOT 发请求。响应侧推荐 4 MiB。
  来源：https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md
- Collector：`max_recv_msg_size_mib`（映射 `grpc.MaxRecvMsgSize`）可配；该值同时约束 zstd 解压窗口（下限 8 MiB、上限 512 MiB）。
  来源：https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configgrpc/README.md

---

## 5. 其余实现

### OpenLLMetry (Traceloop)
- `traceloop-sdk` **0.63.0**（PyPI，Apache-2.0）；仓库 HEAD 2026-11-05。
- 语义约定：用**自家**包 `opentelemetry-semantic-conventions-ai>=0.5.1,<0.6.0`（不是 OTel 官方 gen_ai registry），同时依赖 `opentelemetry-semantic-conventions>=0.63b1`。
- 正文捕获开关：`TRACELOOP_TRACE_CONTENT`（81 个文件引用；默认 `"true"`——与 OTel 规范默认不采集的方向相反）。
- 部分埋点已按 `gen_ai.input.messages` 发射（bedrock / watsonx）。
  来源：`packages/opentelemetry-instrumentation-{openai,bedrock}/pyproject.toml`、`…/bedrock/utils.py`；https://pypi.org/pypi/traceloop-sdk/json

### OpenLIT
- `openlit` **1.45.0**（PyPI，Apache-2.0）；仓库 HEAD 2026-11-03。
- 正文捕获：读 `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT`（`sdk/python/src/openlit/__init__.py:411`）。
- 语义约定：自带 `openlit/semcov`，`HAS_OFFICIAL_OTEL_SEMCONV` 标志位决定是否 import 官方 incubating semconv。
  来源：`sdk/python/src/openlit/__init__.py`、`sdk/python/src/openlit/semcov/__init__.py`

### LangSmith
- 自托管：**Enterprise 附加项的 Kubernetes/Helm 部署**，需要 license key；air-gap 需要 Helm chart ≥ 0.17.0；Insights/Chat 默认启用（chart ≥0.15.1），Engine 需额外 entitlement（chart ≥0.16.0），LLM Gateway 为 beta（chart ≥0.17.1）。闭源。
- `langchain-ai/langsmith-sdk` 仓库已于 **2026-10-16 归档**，Python/JS SDK 拆到 `langsmith-python` / `langsmith-javascript`。
  来源：https://docs.langchain.com/langsmith/self-hosted ；https://github.com/langchain-ai/langsmith-sdk

### Braintrust
- 自托管（数据面）已进入 legacy：**"Starting November 4, 2026, it isn't available to new customers"**，官方引导转向 BYOC（自带云）。自托管形态为"自管数据面 + Braintrust 托管控制面/UI/鉴权"。
- 正文捕获上限 / 去重机制的具体实现：**未查到**公开文档。
  来源：https://docs.braintrust.dev/admin/self-hosting

### Lunary
- PyPI `lunary` **1.4.42**；文档 https://docs.lunary.ai（index 在 `/llms.txt`）。
- `github.com/lunary-ai/lunary` 返回 **404**：当日无公开可查的自托管源码仓库。
  来源：https://pypi.org/pypi/lunary/json ；https://github.com/lunary-ai/lunary（404）

### Portkey（gateway）
- `Portkey-AI/gateway` **MIT**，最新 release **v1.15.2**（2026-01-12），仓库最后推送 2026-05-25。
- OTel：Portkey 自身可作为 OTel backend 端点（收 traces + logs）；反向"把 gateway 日志推到任意 OTel 兼容端点"标注为 **Experimental，[Enterprise/Self-Hosted]**。文档站标题已显示为 "Portkey (Prisma AIRS AI Gateway)"。
  来源：https://github.com/Portkey-AI/gateway/releases ；https://portkey.ai/docs/product/observability/opentelemetry

### Kong AI Gateway
- Kong Gateway 仓库 Apache-2.0，最新 release **3.9.4**（2026-10-12）。
- AI Gateway 并存两套模型：**v1 = Kong Gateway 插件模型**（`ai-proxy`、`ai-proxy-advanced` 等）、**2.x = Policies 模型**；提供 `configure-on-prem` 指南。
- 可观测文档：`ai-logs`（日志位置与级别）、`Gen AI OpenTelemetry metrics reference`（GenAI/MCP/A2A 的 OTel **metrics**）、`ai-audit-log-reference`（AI 策略标准化日志格式，用于用量分析聚合）。
- 是否支持请求/响应正文捕获（如 `log_payloads` 类开关）：**未查到**明确文档条目。
  来源：https://developer.konghq.com/ai-gateway/ ；https://developer.konghq.com/ai-gateway/ai-otel-metrics.md ；https://developer.konghq.com/ai-gateway/ai-logs.md

### Envoy AI Gateway
- 仓库已改名：`envoyproxy/ai-gateway` **301 → `theagentrouter/agent-router`**（描述："Manages Unified Access to Generative AI Services built on Envoy Gateway"），Apache-2.0，2358 stars，HEAD 2026-11-05；最新 release **v1.2.0**（2026-10-07）/ v1.2.1-rc2（2026-10-28）。
  来源：https://github.com/theagentrouter/agent-router

### Bifrost（maximhq/bifrost）
- Apache-2.0；核心模块 tag `core/v1.14.1`；另有 `plugins/telemetry/v1.9.4`、`transports/v2.4.0`、`helm-chart-v2.1.47`。
- 内置 telemetry 插件 = **Prometheus 指标**，暴露 `/metrics`，异步采集；另有独立 `docs/features/observability/otel.mdx`（OTel 导出）。连接器文档覆盖 BigQuery、Datadog、Kafka、PubSub、Splunk、Maxim、Prometheus、OTEL。
- 正文捕获：**按目的地独立剥离**，无单一总开关——日志存储（Postgres/SQLite/ClickHouse 行）用 `client.disable_content_logging`（可被虚拟 key 级、project 级覆盖）；对象存储侧另有 `client.retain_content_in_object_storage` 控制"日志 payload 外置到对象存储"。
  来源：https://github.com/maximhq/bifrost ；`docs/features/telemetry.mdx`、`docs/features/observability/content-logging.mdx`、`docs/features/observability/otel.mdx`

### LiteLLM（logging callback）
- `litellm` 最新稳定 **v1.108.0**（2026-11-04）；文档自述 "open-source (MIT)"（PyPI metadata 为 MIT）。
- 日志回调支持：Langfuse、**OpenTelemetry**、Datadog、S3、GCS、Azure Blob 等；OTel 以 `litellm_settings.callbacks: ["otel"]` 启用，Env：`OTEL_EXPORTER`(`otlp_http`/`console`)、`OTEL_ENDPOINT`、`OTEL_HEADERS`、`OTEL_SERVICE_NAME`、`OTEL_TRACER_NAME`。
- **semconv 模式**：`OTEL_SEMCONV_STABILITY_OPT_IN=gen_ai_latest_experimental` → LLM span 名变 `{operation} {model}`、kind=CLIENT、抑制非标准 `raw_gen_ai_request` 子 span、新增 `gen_ai.provider.name`（与 `gen_ai.system` 并存）、把逐条消息事件合并为单个 `gen_ai.client.inference.operation.details` 事件。
- 正文捕获：`OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT` = `NO_CONTENT|SPAN_ONLY|EVENT_ONLY|SPAN_AND_EVENT`，并支持 **per-handler** `OpenTelemetryConfig.capture_message_content` 做差异化策略。注意变量名与 OTel 规范现行名（`…_CAPTURE_CONTENT`）**不一致**，LiteLLM 用的是 `_MESSAGE_CONTENT` 旧名。
- 体积相关：`max_request_size_mb` 默认 `null`（不限）、`max_response_size_mb` 默认 `null`；`MAX_DECOMPRESSED_REQUEST_BODY_SIZE_MB` 默认 **50**（v1.108.0 起，压缩体解码前后都校验，超限 413）、`MAX_REQUEST_BODY_DECOMPRESSION_RATIO` 默认 **200**、`MAX_REQUEST_BODY_SIZE_TO_REPAIR_MB` 默认 **1**（超过则跳过 JSON 修复直接 400）、`MAX_REQUEST_HEADER_SIZE` 默认 16384 B。
- 正文是否落库：`store_prompts_in_spend_logs` **默认 false**（打开才把 prompts/responses 写入 spend log 表）。
  来源：https://docs.litellm.ai/docs/proxy/logging.md ；https://docs.litellm.ai/docs/observability/opentelemetry_integration.md ；https://docs.litellm.ai/docs/proxy/config_settings.md ；https://github.com/BerriAI/litellm/releases

---

## 6. 「同一会话多次请求只记增量 / 内容寻址去重」的成熟实现

**结论：截至 2026-11-05，没有查到任何一家把"会话增量（delta）记录"做成成熟特性。**内容寻址去重只有"附件/超长字段"层面的实现，且只有 Langfuse 一家是产品级落地的。

已核实存在的实现：

| 实现 | 粒度 | 机制 | 证据 |
| --- | --- | --- | --- |
| Langfuse media/attachment | 单个附件/媒体 | `mediaId = url-safe(SHA-256)[:22]`，唯一约束 `(projectId, sha256Hash)`，重复内容复用同一 mediaId、返回 `uploadUrl: null` | `packages/shared/src/server/media/mediaService.ts`、`web/src/features/media/server/mediaService.ts`；https://langfuse.com/docs/observability/features/multi-modality.md |
| Langfuse 超长字段外置 | 单条 observation 的 input/output/metadata | 超 `LANGFUSE_OBSERVATION_FIELD_SIZE_LIMIT_BYTES`（默认 2 MiB）→ 以 `text/plain` 存 S3 → 行内只留 `source=field_size_limit` 引用；结果含 `reused`（同内容复用） | `worker/src/features/observation-field-overflow/processObservationFieldOverflow.ts` |
| OTel GenAI `*_ref` | 每次调用的 instructions/input/output/tool definitions | 规范允许 hook 外置 + "MAY use content-derived identifiers to deduplicate"；registry 示例 URI 用 64 hex 摘要命名 | `docs/gen-ai/gen-ai-spans.md`、`docs/registry/attributes/gen-ai.md` |

明确"未查到"的部分：
- **会话增量**：无任何产品的文档描述"同一 session 第 N 次请求只存与第 N-1 次请求的 diff"。Langfuse 的 `gen_ai.conversation.compacted` 是"会话上下文是否被压缩"的布尔属性，不涉及增量存储。
- **prompt 级跨请求去重**：Helicone（requestId 路径、CH 每请求一行）、LiteLLM（`store_prompts_in_spend_logs` 直存）、Phoenix（每 span 直存）、Bifrost（行/对象存 payload）均无内容哈希去重路径可查。
- GitHub 仓库搜索 `llm trace dedup`、`content-addressed prompt logging`、`prompt deduplication observability`：命中 4 个个人玩具级项目（≤1 star），无成熟实现。

---

## 7. 关键事实汇总表

| 产品 | 开源自托管 | 正文捕获上限 | 去重机制 | OTel 支持 | 单请求体积上限 |
| --- | --- | --- | --- | --- | --- |
| **Langfuse v4.76.0** | 是（MIT core + EE 目录独立许可；Docker/Helm/K8s/Terraform） | 无固定正文上限；`input`/`output`/`metadata` 超 **2 MiB** 可自动外置到 S3（开关默认关） | **有**：媒体/附件 SHA-256 内容寻址 + 超长字段 `reused` 复用 | 有：OTLP/HTTP（JSON+protobuf），`/api/public/otel/v1/traces`，无 gRPC；需 `x-langfuse-ingestion-version: 4` 才实时 | legacy `/ingestion` **4.5 MB**（bodyParser 硬编码）；OTel 路由 **512 MiB**（`LANGFUSE_OTEL_INGESTION_MAX_BODY_BYTES`），警告线 16 MiB |
| **Helicone**（Apache-2.0） | 是（all-in-one / compose / Helm）；**最后 release 2025-08-21** | async 端点 **10 MB**；proxy 内存 buffer 阈值 20 MiB，无截断 | **无**（requestId 为键，逐请求直存） | 半：只接 OpenLLMetry 经 `@helicone/async` 的 OTLP 导出，非通用 OTLP backend | async **10 MB**（400 拒绝）；proxy 平台侧数值未查到 |
| **Arize Phoenix 21.0.0**（ELv2） | 是（pip/docker/helm/kustomize） | 无显式 body 上限；gRPC 默认 **4 MiB** 未被覆盖；无属性截断 | **无** | 原生 OTel（`/v1/traces` 仅 protobuf；gRPC TraceService）；语义用 OpenInference（semconv 0.1.43） | 队列上限 `PHOENIX_MAX_SPANS_QUEUE_SIZE` 默认 20 000 span，满即 503 |
| **OTel GenAI semconv**（Development） | 规范（Apache-2.0） | Opt-In；`OTEL_INSTRUMENTATION_GENAI_CAPTURE_CONTENT`=`NO_CONTENT`/`SPAN_ONLY`/`EVENT_ONLY`/`SPAN_AND_EVENT`；截断仅 MAY，无阈值 | 规范层：`*_ref` + "MAY use content-derived identifiers" | 本体即 OTel | OTLP：典型默认 4 MiB，**规范推荐 64 MiB**，可配置；响应 4 MiB |
| **OpenLLMetry 0.63.0** | 是（Apache-2.0） | `TRACELOOP_TRACE_CONTENT`（默认 true） | 无 | 有（OTLP 导出） | 取决于后端 |
| **OpenLIT 1.45.0** | 是（Apache-2.0） | `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT` | 无 | 有（OTel 原生 + 自带 semcov） | 取决于后端 |
| **LangSmith** | 仅 Enterprise 自托管（Helm，需 license key，支持 air-gap） | 未查到公开数值 | 未查到 | 未查到通用 OTLP backend | 未查到 |
| **Braintrust** | 自托管数据面已 legacy（2026-11-04 起不售新客），转 BYOC | 未查到 | 未查到 | 未查到 | 未查到 |
| **Lunary**（PyPI 1.4.42） | 源码仓库 **404**，当日无公开自托管 | 未查到 | 未查到 | 未查到 | 未查到 |
| **Portkey gateway v1.15.2**（MIT） | 是（gateway 开源；平台闭源） | 未查到 | 无 | 有：Portkey 作 OTel backend；反向推日志到 OTel 为 Experimental/Enterprise | 未查到 |
| **Kong AI Gateway**（Apache-2.0 3.9.4） | 是 | 正文级捕获开关未查到（有 `ai-logs`、GenAI OTel metrics、audit log 格式） | 无 | 有：GenAI/MCP/A2A 的 OTel **metrics** | 未查到 |
| **Envoy AI Gateway → agent-router v1.2.0** | 是（Apache-2.0，基于 Envoy Gateway） | 未查到 | 未查到 | 未查到（仓库改名后文档待查） | 未查到 |
| **Bifrost core/v1.14.1** | 是（Apache-2.0） | 无硬上限；按目的地独立剥离 `client.disable_content_logging`，payload 可外置对象存储 | 无 | 有（telemetry 插件=Prometheus；另有 OTel 导出文档） | 未查到 |
| **LiteLLM v1.108.0**（MIT） | 是 | `store_prompts_in_spend_logs` 默认 false；内容采集默认由 `…CAPTURE_MESSAGE_CONTENT` 控制 | 无 | 有：`callbacks:["otel"]` + `OTEL_SEMCONV_STABILITY_OPT_IN=gen_ai_latest_experimental` | `max_request_size_mb` 默认不限；解压后体 **50 MB** 上限、解压比 200、修复体 1 MB 上限 |
