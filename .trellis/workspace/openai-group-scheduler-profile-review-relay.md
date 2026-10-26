# 分组调度画像补充评审：纯 apikey 中转场景

前置：上一轮 `.trellis/workspace/openai-group-scheduler-profile-review.md` 已核实。本文只写**场景更新后的增量**。抽查确认协调者 7 条事实成立（`ReportResult` 无账号类型过滤；HTTP `previous_response_id` 对非 apikey 主动跳过；apikey WSv2 账号级默认关 → HTTP SSE；`SubscriptionPriority` 只认 ChatGPT OAuth `plan_type`；billing probe 仅 `AccountTypeAPIKey`；health breaker 仅 pool-mode apikey；`require_oauth_only` 拒 apikey）。

**修订后的一句话：** 阶段 1（分组 top_k/TTFT 权重）在纯中转里**打分区分度变大、用户体感覆盖变小**；「pro 尽量用最快号」的主杠杆要从「加重 TTFT」改成「在工具续链可重建时，让 HTTP `previous_response_id` 也能逃逸」。不要打开全局 `StickyWeighted`，也不要把 `previous_response_id` 原样转到另一把 key。

---

## a) 哪些不变、哪些修订

### 不变（直接沿用上一轮）

- JSON 一列 `groups.openai_scheduler_overrides`，不要两列。
- 总开关 `openai_advanced_scheduler_enabled` 必须开。
- **禁止 `top_k=1`**（满载排队、羊群、429 接力）。纯中转 429 更常见，这条更硬。
- 不搬 Claude `filterByMinPriority`；不靠 `model_routing`；不改 `account_groups.priority` 当打分输入。
- 分组覆盖优先级：`分组 > 全局 DB > yaml`；`GetByIDLite`；字段贯通 duplicate/auth cache。
- 全局 `StickyWeighted` **保持关**（粘性号塞队首，抵消 TTFT）。
- 利润门 / PrivacySet / 用户-账号绑定仍在打分之前。

### 必须修订

| 上一轮 | 纯中转后 |
| --- | --- |
| 阶段 1 是「最小可用、先做」；阶段 2 sticky 逃逸是「若 sticky 命中率高再做」 | 阶段 1 仍先落地（LB 地基），但 **Codex HTTP 多轮几乎走不到 LB**；用户体感主路径改到「apikey + 可重建 input 时的 previous_response 逃逸」 |
| `SubscriptionPriority`「看池子」 | **无意义，强制关**。开了也是空订阅池 → 整池打分，零收益、多一次分区 |
| 阶段 2 明确 **不** 改 HTTP `previousResponseCanMove=false` | **要改，但是有条件的**（见 b）：只在工具续链可自重建时剥离 `previous_response_id` 再选号，不是把 resp_id 转到另一把 key |
| `ttft_max_ratio=2.0` 放阶段 2 | **提前进阶段 1**。跨线路 TTFT 差一个数量级时，min-max 会把快号簇压扁，需要先踢慢线路 |
| `upstream_cost`「pro 若不打开则不是主风险」 | 因子现在**会**有数（billing probe），但 **pro 速度画像必须保持 0**——自报低价会吸流量，和「最快」可能反相关 |
| 运营：「绑快号」 | 额外硬约束：pro **不能** `require_oauth_only`（否则 apikey 进不来） |

### 阶段 1 预期收益：打分变大，体感变小

**打分层（LB 真正跑起来时）收益变大：**

1. 不同中转线路 TTFT 常差 5–40×，min-max 会把 8s 线路打到因子 0；混合 OAuth 池里同质 ChatGPT 差得没这么大。
2. 没有订阅池先切一刀，TTFT 在全候选里比。
3. 传输面统一 HTTP SSE，EWMA 口径比 OAuth-WSv2 / apikey-HTTP 混跑干净。
4. 无样本仍是 0.5——冷启动/重启中性，这条不变。

**用户体感覆盖变小：** Codex `/v1/responses` 多轮带 `previous_response_id` 时，HTTP 把 `previousResponseCanMove` 写死 `false`（`openai_gateway_handler.go:640`），层 1 硬亲和直接返回，**不看 top_k、不看 TTFT 权重、不走 sticky 逃逸**。纯中转没有「OAuth 被跳过、再找一把 apikey」的混合缓冲，**几乎每一段多轮都锁在首轮抽中的那把 key**。阶段 1 只优化：无 `previous_response_id` 的首轮、Messages、粘性未命中。

验收口径仍是：**新 session 选号 TTFT 分布左移**；不要用多轮会话当阶段 1 验收。

### min-max 在「线路 TTFT 差大」时的区分度

数值（权重 TTFT=4，其它因子相同）：min=200ms，max=8000ms。

| 账号 TTFT | 因子 | 贡献 |
| --- | --- | --- |
| 200ms | 1.00 | 4.00 |
| 400ms | 0.97 | 3.90 |
| 800ms | 0.92 | 3.69 |
| 2000ms | 0.77 | 3.08 |
| 8000ms | 0.00 | 0.00 |

- **慢线路会被碾死**（这是阶段 1 的真实收益）。
- **快号簇内部几乎分不开**：200 vs 800ms 只差 0.3 分，Load=1.0 的满载惩罚就能翻盘。有一根 8s outlier 时，min-max 把 200–800ms 全挤在 0.92–1.00。
- 所以跨线路差异大 **不是**「权重再拉高就完事」。要先用 `ttft_max_ratio` 把 2s+ 踢出主池，剩下 200–600ms 再 min-max，快号簇才重新拉开。
- 轮盘 `(score-min)+1` 仍会把 top_k=2 压到约 8 成打中当前最快——上一轮的 86% 数量级仍成立，但「当前最快」在有 outlier 时其实是「快号簇里几乎随机」。

---

## b) 锁号常态 → 阶段 2 要提前，但换做法

**是，优先级提前。** 但提前的不是「把全局 sticky 逃逸阈值从 15s 改成 2s」，而是 **HTTP apikey 的 previous_response 有条件逃逸**。session_hash 逃逸（15s→2–3s）对 Codex HTTP 多轮**几乎走不到**（层 1 已经 return）。

### 不要做的两件事

1. **不要** `StickyWeighted=true`。它虽然能让 `canMove` 绕过硬亲和，但会把粘性号塞进 `selectionOrder` 队首（`:1056-1071`），TTFT 权重白加。
2. **不要** 把 `previous_response_id` 原样发到另一把本地 key。官方 API key 的 resp_id 按 key/project 隔离；中转场景更严——resp_id 活在**上游实例的 store + 上游当时选中的号**上。换本地账号（尤其不同 `base_url`）会 400 / previous_response_not_found / 「鉴权失败」。同 `base_url` 不同 key：上游按自己的 groupID 查绑，大概率 miss；即便命中，也只是继续打上游原来那把号，**本地「换快号」是假换**。

### 适配中转的做法：有条件剥离，而不是迁移 resp_id

与 WS 首包同一套覆盖率（`AnalyzeToolCallOutputContextCoverageBytes`）：

```
canMove = !HasFunctionCallOutput || ContextCoversAllCallIDs
```

| 条件 | 行为 |
| --- | --- |
| `canMove=false`（工具输出的 call_id 不在本包 input 里） | **仍硬锁**。剥离会导致上游 `No tool call found for function call output` |
| `canMove=true` 且绑定号 TTFT/错误率未超分组阈值 | 继续硬亲和（续链便宜，别为了 50ms 拆会话） |
| `canMove=true` 且绑定号 `hasTTFT && ttft > minTTFT*ratio`（或 > `sticky_escape_ttft_ms`） | **跳过层 1**，走 LB；转发前 `RemovePreviousResponseIDFromBody`；成功后 `bindHTTPResponseAccount` 绑**新** resp_id |

实现要点：

- HTTP 路径不要再把 `previousResponseCanMove` 写死 `false`；用上面的 coverage。handler 里已有 `validateFunctionCallOutputRequest`，差的是「可重建时允许换号」而不是「缺上下文就 400」。
- **不要**开全局 `StickyWeighted`。在 `Select` 层 1 增加 apikey 专用口子：`account.IsOpenAIApiKey() && canMove && shouldEscapeBoundAccount`。OAuth HTTP 本来就会被 `:695-719` 跳过，不影响。
- 逃逸后 **改绑新 resp_id**（不是 `PreserveStickyBinding` 那种临时逃逸）。会话后续应留在快号上。这和官方「session_hash 临时逃逸避免迁到冷号」注释不冲突——这里迁的是可重建的 HTTP 续链，不是 session_hash。
- WS 入站若将来给某把 apikey 开了 WSv2，沿用现有 coverage；不要另搞一套。

### 工具续链风险在纯中转是否同样成立？

**成立，而且更硬，不是更松。** 风险来自 **payload 是否自包含**，与 OAuth/apikey 无关：

- Codex 多轮常见「`previous_response_id` + 增量 input」，工具轮常见「只有 `function_call_output`、没有对应 `function_call`」→ `canMove=false`，必须锁号。
- 中转还多一层：即便 input 自包含，**带着 resp_id 换上游**也会坏；所以逃逸时必须剥离，不能「放宽亲和但仍携带 resp_id」。
- 结论：对 apikey **放宽的是「可重建时允许拆链换号」**，不是「incomplete 工具链也可以换」。上一轮「不要为了快默认打开跨号迁移」仍然对——改的是 HTTP 从「永远 false」变成「与 WS 同一 coverage，再加慢号门槛」。

---

## c) 纯中转新增风险 × 本设计

### 1. 中转 429 语义（上游限流透传）

三种 429 不是一回事：

| 来源 | 本地应不应该摘号 | 现状 |
| --- | --- | --- |
| 这把 key 在上游的 RPM/并发（如 `Too many pending requests`） | 应该短冷却 | 无 reset 头 → `apply429FallbackRateLimit` 秒级回避。合理 |
| 上游当前选中的 ChatGPT 号 5h/7d 打满，**透传 `x-codex-*`** | **不应该**按小时封这把中转 key（下一请求上游可能换号） | `handle429` 对 `Platform==OpenAI` **不区分 apikey/OAuth**，会 `SetRateLimited` 到 5h/7d reset。**过封** |
| 上游整站过载 | 应该摘 | 短冷却或 health breaker。合理 |

交互：

- **pool-mode**：`HandleUpstreamError` 默认不因 429 停调，但 `ObserveOpenAIAPIKeyHealthFailure` 把 429/5xx 计入 breaker，窗口内多次仍会 `TempUnsched`。快号若偶发 429 可能被 breaker 拿掉——这是安全阀，pro 不要关 breaker 来保「快」。
- **非 pool**：透传的 x-codex 长冷却会把当前最快中转钥匙直接打出池；阶段 1 的高 TTFT 权重会立刻吸到「新最快」，形成 429 接力。所以 **ErrorRate 权重要略高于上一轮**，且 `top_k=2` 不能少。
- 本设计 **不** 在阶段 1 改 429 分类（那是独立正确性 bug）。调度侧用 ErrorRate + 禁止 top_k=1 兜住。若后续改 429：apikey 忽略 `x-codex-*` 窗口、只走秒级 fallback。

### 2. TTFT 按模型分桶

**必要性比混合场景更高。** 一把中转 key 通常挂 gpt-5.4 / gpt-5 / gpt-4.1，慢模型会把 account 级 EWMA 拉脏；`ReportOpenAIAccountScheduleResult` 虽收 `model`，`ReportResult` 只按 `accountID` 写 EWMA（`:1813-1817`）。pro 用户若主要打慢模型，会被「被快模型刷亮的号」误导。

与本设计：阶段 1 仍按账号 EWMA（改动面小，跨线路 8s vs 200ms 仍分得开）。**阶段 3 提前到 2 之后立刻做**，至少 `(accountID, canonicalModel)`；没有分桶时不要把 TTFT 权重加到 5 以上（慢模型噪声会被放大）。

### 3. 多中转同线路重复

同 host（或同 host+同 key）的多把号：TTFT 几乎相同 → min-max≈0.5 → 占掉 top_k=2 的两个名额，真正的快线路进不了轮盘。上游粘性也相同，换来换去是假选择。

与本设计：阶段 1 **不做** host 折叠（范围）。运营先去重。配方上 `top_k=2` 在「两条真线路」时正确，在「两把重复 key + 一把慢的」时会废掉。阶段 3 可选：top-K 按归一化 `base_url` 多样性约束。

### 4. 上游 sub2api 自己的粘性

本地 TTFT 量的是 **整条路径**（本机 → 上游网关 → 上游当时选中的号）。

- 带着 `previous_response_id` 换一把 **同一上游** 的本地 key：上游仍锁原号，本地「快号」是假的。这再次证明 b) 必须 **剥离 resp_id** 才算真换。
- 剥离后打到同一上游：上游当新会话做自己的 LB，有机会换到上游更快的号——这是中转场景下 canMove 逃逸的额外收益。
- 剥离后打到 **另一条** 上游：换的是真线路，阶段 1 的 TTFT 权重这才兑现。
- 上游若自己 `top_k=7` 轮盘，本地 EWMA 是混合分布，方差大、收敛慢。`alpha=0.2` 在这种噪声下更钝。不要指望 10 次样本就稳定；`ttft_max_ratio` 用「明显慢」而不是「细排序」。

### 5. 其它短交互

- **health breaker**：只打 pool-mode apikey；与「快」同向（摘病号）。pro 保留。
- **billing `upstream_cost`**：现在有信号。速度画像保持 0。以后要「便宜且快」另做配方，不要塞进 pro。
- **Reset / QuotaHeadroom**：ChatGPT 窗口语义，apikey 中转上基本是 0。保持默认 0。
- **WS 连接内不重选号**：apikey 默认 HTTP，这条比混合场景弱。仍不要为了换号去给中转 key 开 WSv2。

---

## d) 修订后的落地路径与 pro 配方

### 阶段 0 — 前提（不写代码）

1. `openai_advanced_scheduler_enabled=true`。
2. pro **未** 开 `require_oauth_only`；**未** 开 `SubscriptionPriority`。
3. 看 `StickyHitRatio` / `LoadBalanceSelectTotal` / 有 `previous_response_id` 的占比。纯中转预期 previous 命中率很高——这是做阶段 2 的依据，不是做阶段 1 的否决。
4. 运营扫一遍同 `base_url` 重复号。

### 阶段 1 — 分组覆盖 + 相对 TTFT 淘汰（建议本次就做）

相对上一轮：JSON 覆盖范围扩大，**把 `ttft_max_ratio` 从阶段 2 拉进来**。

- 字段：`lb_top_k`、`weight_ttft`、`weight_error_rate`、`weight_load`、`ttft_max_ratio`。
- `selectTopKOpenAICandidates` 之前：`hasTTFT && minTTFT>0 && ttft > minTTFT*ratio` 移出主池；无样本保留。
- 函数签名改 `ForRequest(ctx, groupID)`；`GetByIDLite` merge；`IsValid()`。
- 验收：新 session（无 previous）选号分布离开慢线路；有 previous 的请求行为与现在完全一致。

### 阶段 2 — apikey HTTP previous 有条件逃逸（紧接着做，不要等）

这是纯中转场景下「多轮也换快号」的**唯一正确杠杆**。

- HTTP 计算 coverage → `canMove`。
- `Select` 层 1：apikey + canMove + 绑定号超分组阈值 → 不硬返回，落入 LB；转发前剥离 resp_id；成功则绑新 resp_id。
- `canMove=false` 行为与现在完全相同。
- 分组阈值复用 `sticky_escape_ttft_ms` / `ttft_max_ratio`（不必新旋钮）。
- **不开** `StickyWeighted`。session_hash 15s→2.5s 可作为 2b（只改善无 previous 的粘性会话），不是主路径。
- 验收：无工具续链的多轮，慢号上的第二条请求能换到快线路；带未覆盖 `function_call_output` 的请求仍锁原号。

### 阶段 3 — 测量（中转比混合更急）

- TTFT EWMA 按 `(accountID, canonicalModel)`。
- 多副本共享（Redis 或 usage_log 回填）。
- 可选：top-K 的 `base_url` 去重。
- 可选：apikey 429 忽略透传的 x-codex 5h/7d 窗口（独立 bug，不堵阶段 1/2）。
- 按 group 打点：layer、是否 strip previous、选中账号 TTFT、同 host 重复命中。

### 明确不做（相对上一轮新增）

- 不为中转打开全局 `StickyWeighted`。
- 不把 `previous_response_id` 原样跨 key 转发（包括同 `base_url`）。
- 不在 `canMove=false` 时逃逸。
- 不为「快」给中转号开 WSv2。
- 不把 `upstream_cost` / `SubscriptionPriority` / `Reset` 写进 pro 速度配方。
- 不在阶段 1 改 429 分类或 health breaker 阈值。

### pro 分组最终配方

前提：高级调度已开；`require_oauth_only=false`；`subscription_priority=false`；`sticky_weighted=false`。

```json
{
  "lb_top_k": 2,
  "weight_ttft": 4.0,
  "weight_error_rate": 1.2,
  "weight_load": 1.0,
  "weight_upstream_cost": 0,
  "ttft_max_ratio": 3.0,
  "sticky_escape_ttft_ms": 2500,
  "sticky_escape_error_rate": 0.25
}
```

| 项 | 值 | 相对上一轮 | 原因 |
| --- | --- | --- | --- |
| `lb_top_k` | **2** | 不变 | 86% 级偏好 + 429 第二枪 |
| `weight_ttft` | **4.0** | 从 4–5 收成 4 | 跨线路 min-max 已经很陡；再加到 5 会放大未分桶的模型噪声 |
| `weight_error_rate` | **1.2** | 从 1.0–1.2 取上沿 | 中转 429 透传、解禁回打、breaker 竞态 |
| `weight_load` | **1.0** | 不变 | 唯一反羊群力；快号簇被 min-max 压扁后全靠它 |
| `ttft_max_ratio` | **3.0** | 从 2.0 放宽，并提前到阶段 1 | 200ms 最快时，2.0 会只剩一根线；3.0 踢 2s+、保留 200–600ms 簇 |
| `sticky_escape_ttft_ms` | **2500** | 取 2–3s 中值 | 阶段 2 绑定号逃逸门槛；session_hash 2b 共用 |
| `sticky_escape_error_rate` | **0.25** | 取 0.2–0.3 中值 | 默认 0.5 对 429 几乎无感 |
| `upstream_cost` | **0** | 显式钉死 | 有 probe 信号也不进速度配方 |

`top_k=1` 仍禁止（1 当 2 并告警）。账号很多且已按线路去重时，最多把 `lb_top_k` 升到 **3**，不要更高。
