# 排查 OVH Sub2API 502 与用户体验配置

## Goal

通过只读 SSH 证据定位 `api.edom37.online/responses` 的 502 由哪一层产生，并审计当前生产配置中会直接影响请求成功率、首字延迟、流式稳定性和错误可理解性的项目。

## Background

- 用户反馈错误：`unexpected status 502 Bad Gateway: Upstream request failed`。
- 目标 URL：`https://api.edom37.online/responses`。
- 样例 request id：`ee150378-e40a-4b72-abcf-72aca7056a2d`。
- OVH SSH 别名为 `ovh`，生产部署目录预期为 `/opt/sub2api-deploy`，应用容器预期名为 `sub2api`；实际情况必须以只读盘点为准。
- 本次仅排查，不修改生产配置、不重启容器、不部署、不修改数据库，也不向 `/responses` 发送带凭据或用户数据的探测请求。

## Requirements

- 采集容器状态、健康检查、重启次数、资源限制和宿主机 CPU、内存、磁盘、负载等运行证据。
- 识别公网入口、源站反向代理、Sub2API 应用和外部上游之间的真实请求链路。
- 检查与用户体验直接相关的有效配置，包括连接、响应头和读取超时，流式/SSE 缓冲与空闲超时，keep-alive，连接池，并发与队列限制，重试，健康检查和错误包装。
- 优先按样例 request id 关联日志；若无命中，则在有限时间窗口内采样近期 502、`Upstream request failed`、超时、连接重置、限流、OOM 和容器重启证据。
- 读取配置和日志时不得输出 `.env`、访问令牌、API Key、Cookie、Authorization、数据库内容、证书私钥或用户请求正文。
- 对结论标注证据、影响范围、置信度和建议方向，区分根因、放大因素和可观测性缺口。

## Acceptance Criteria

- [ ] 确认或有证据地收窄 502 的实际生成层：CDN/公网入口、源站反代、Sub2API 或外部上游。
- [ ] 给出会影响用户体验的配置清单，至少覆盖超时、流式传输、并发/资源、重试和错误反馈。
- [ ] 对样例 request id 给出日志关联结果；无命中时说明日志保留或字段缺口，并用近期同类错误替代分析。
- [ ] 输出按严重度排序的根因判断和整改建议，明确哪些建议需要变更生产状态及其风险。
- [ ] 整个排查过程不改变生产服务、配置、容器、数据库、DNS、Cloudflare、防火墙或流量。

## Out of Scope

- 修改或部署修复。
- 读取用户请求正文、真实凭据或数据库业务数据。
- 对第三方上游执行压力测试或批量探测。
