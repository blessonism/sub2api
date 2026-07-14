# OVH mihomo 与 Sub2API 代理链路诊断结果

## 结论

当前 OVH 上没有可用的本机 mihomo 服务。宿主机未发现 mihomo/clash 的 systemd 服务、进程、容器、镜像、配置文件或常见代理端口监听；Sub2API 容器内 `mihomo` DNS 无法解析，`mihomo:7890` 端口探测失败。

Sub2API 的系统级出站代理当前被 `/opt/sub2api-deploy/docker-compose.override.yml` 明确关闭。主 compose 虽然保留 `http://mihomo:7890` 默认值和注释，但 override 把大小写形式的 `HTTP_PROXY`、`HTTPS_PROXY`、`ALL_PROXY` 及 `UPDATE_PROXY_URL` 全部设为空，实际容器环境也为空。

当前服务通过直连工作。OVH 宿主机和 Sub2API 容器访问 Cloudflare 探测端点均返回 `204`，访问 OpenAI `/v1/models` 均返回未携带凭据时预期的 `401`，证明 DNS、TLS 和到 OpenAI 的直连链路正常。

最近的 `/responses` 502 没有 mihomo 或代理连接错误证据。采样日志中显式代理错误为 `0`。其中 `303` 个请求明确因为 GPT-5.6 Sol 的思维等级被上游拒绝而全部返回 502；另有 `232` 个请求出现 `no available accounts`，其中 `199` 个最终返回 502。最近一条 `/responses` 502 为 `2026-07-11 01:15:53 +08:00`，随后两小时采样为 `905` 个 200、无 502。

## 分层证据

### mihomo 服务层

- 未发现 `mihomo.service` 或 `clash.service`。
- 未发现 mihomo/clash 进程、Docker 容器和 Docker 镜像。
- 未发现 `7890-7899`、`9090` 或 `1080` 相关监听。
- 在 `/etc`、`/opt`、`/usr/local/bin` 和 `/root` 的有限深度文件名扫描中未发现 mihomo/clash 文件。

判断：本机 mihomo 不是“运行异常”，而是当前没有部署或没有保留。

### mihomo 出站层

- Sub2API 容器内 `getent hosts mihomo` 返回 unresolved。
- Sub2API 容器到 `mihomo:7890` TCP 探测失败。
- 显式使用 `--proxy http://mihomo:7890` 的 HTTPS 探测在 DNS 阶段超时。

判断：如果重新启用主 compose 中的默认代理环境，当前部署会因 `mihomo` 名称和端口不可用而失败。

### Sub2API 应用网络层

- `sub2api` 容器健康，未 OOM，当前运行在 `sub2api-deploy_sub2api-network`。
- 容器中的系统代理环境变量全部为空。
- OVH 宿主机和容器均可直接访问公共 HTTPS 与 OpenAI。

判断：当前 Sub2API 不依赖本机 mihomo，系统级出站走直连且目前可用。

### Sub2API 业务代理层

- 代码中的账号请求代理来自 `account.Proxy.URL()`，再传给上游 HTTP 客户端；它与容器 `HTTP_PROXY` 是两条独立机制。
- 本次未读取生产数据库或真实代理凭据，因此不枚举具体账号代理记录。
- 最近 12 小时采样中没有包含 `proxy`、`mihomo` 或 `clash` 的明确错误字段。

判断：没有证据表明账号级代理当前发生系统性故障；若仅某个账号或代理失败，需要用代理名称、账号 ID 或失败 request ID 做最小范围关联。

## 502 证据

- `/responses` 502 关联信号主要为：
  - `openai.upstream_failover_switching`
  - `openai.account_select_failed: no available accounts`
  - `openai.forward_failed: 请求的思维等级不被允许`
- failover 的上游状态以 `502` 为主，同时有 `429`、`503` 和 `524`。
- Caddy 大量 `aborting with incomplete response` 为客户端取消或提前断开；这不是 mihomo 连接失败。

判断：这轮 502 的主因在上游账号池可用性、上游返回和请求参数兼容性，不在本机 mihomo。

## 最近社区 Issue 交叉验证

### 高度匹配

1. [`Wei-Shaw/sub2api#3782`](https://github.com/Wei-Shaw/sub2api/issues/3782)，创建于 `2026-07-07`，当前仍开放：`/v1/responses` 没有在本地拒绝非法或不受支持的 `reasoning.effort`，上游 400 容易被表现为 upstream error。
   - OVH 匹配证据：`303` 个 `gpt-5.6-sol` `/responses` 请求出现 `upstream error: 400 message=请求的思维等级不被允许`，最终全部为 502。
   - 当前代码对 GPT-5.6 的 `max` 会保留为 `max`；如果所选账号或上游能力只允许较低等级，就会触发该类错误。

2. [`Wei-Shaw/sub2api#3817`](https://github.com/Wei-Shaw/sub2api/issues/3817)，创建于 `2026-07-08`，当前仍开放：账号用完后不自动切换，Codex 直接 502，新建会话后恢复。
   - OVH 匹配证据：`232` 个请求记录 `no available accounts`，其中 `199` 个最终为 502；大量请求先发生上游 failover，再耗尽可选账号。

3. [`Wei-Shaw/sub2api#3981`](https://github.com/Wei-Shaw/sub2api/issues/3981)，创建于 `2026-07-10`，当前仍开放：Codex Desktop 多轮输入携带错误 `item_*` ID，被上游 400 拒绝，Sub2API 可能包装成 502，并在旧会话中重复失败。
   - OVH 匹配证据：`13` 个请求出现 `invalid_id_prefix`，上游要求 ID 以 `rs` 开头；这些请求在当前 failover 后最终返回 200，因此是同类兼容问题，但不是本次用户可见 502 的主因。

### 部分匹配

4. [`Wei-Shaw/sub2api#3934`](https://github.com/Wei-Shaw/sub2api/issues/3934)，创建于 `2026-07-10`，当前仍开放：GPT-5.6 压缩请求在 Sub2API 中转后出现 502，社区使用 compact model mapping 临时绕过。
   - OVH 有 `8` 个 `/responses` body-signal compact 请求，但均记录 `codex.remote_compact.succeeded`，因此本机当前 compact 路由不是主要故障点。

5. [`Wei-Shaw/sub2api#3857`](https://github.com/Wei-Shaw/sub2api/issues/3857)，创建并关闭于 `2026-07-09`：流式 `response.failed` 中的上下文超限被错误表现为 502，导致下游重试。
   - 该问题能解释长会话中的部分 502，但本次 OVH 采样没有命中 `context_length_exceeded`。

6. [`openai/codex#31864`](https://github.com/openai/codex/issues/31864)，创建于 `2026-07-09`，当前仍开放：GPT-5.6 Sol 的 `collaboration.spawn_agent` 保留命名空间冲突；第三方 Responses 代理可能把上游校验错误掩盖成 502。
   - OVH 最近 24 小时没有该错误文本，因此不是本次主因。

### 明确不匹配

7. [`Wei-Shaw/sub2api#3814`](https://github.com/Wei-Shaw/sub2api/issues/3814)，创建于 `2026-07-08`：SOCKS5 代理端口连接拒绝导致上游 502。
   - 该 issue 会出现明确的 `SOCKS5 connect`、代理地址和 `connection refused`；OVH 采样中此类错误为 `0`。

8. OpenAI 官方状态页在 `2026-07-09 19:44Z` 到 `20:42Z` 记录过模型选择容量错误，但已恢复。OVH 的主要失败从 `2026-07-10 22:21 +08:00` 开始，时间不重合，不能归因于该次官方事故。

## 建议

1. 第一优先级处理 GPT-5.6 Sol 的 reasoning effort：临时将该路由的 effort 从 `max` 降到已验证可用的等级，或在转发前按账号/模型能力归一化；同时把上游 400 透传为客户端 400，避免伪装成 502。
2. 第二优先级处理账号池耗尽：核对账号 98 的模型能力、额度、并发、临时不可调度状态和 failover 行为，避免确定性参数错误把全部账号逐个排除。
3. 当前保持 override 的直连模式，不要直接删除空代理覆盖；否则主 compose 会指向不存在的 `mihomo:7890`。
4. 整理部署配置的单一事实来源：要么删除主 compose 中失效的 mihomo 默认值并明确直连，要么正式部署 mihomo sidecar、加入同一 Docker 网络、增加健康检查后再启用代理。
5. 若要验证某个账号级代理，提供失败 request ID、账号 ID 或代理名称后执行只读定向关联；不要全量读取代理表或凭据。

## 生产变更边界

部署 mihomo、修改 compose、删除 override、切换代理节点、重启容器或修改账号代理都属于生产变更，必须先备份配置、说明影响和回滚方式，并再次获得明确确认。
