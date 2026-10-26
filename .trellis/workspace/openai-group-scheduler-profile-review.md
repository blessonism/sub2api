# 分组级 OpenAI/Codex 调度画像评审

任务：评估「方案 B：分组覆盖 `lb_top_k` + TTFT 权重」能否让 pro 分组用户尽量调度到实测最快（TTFT）的 Codex/OpenAI 账号。

结论先行：**方案方向正确，但是半成品。** 它能改善「新会话第一次选号」的 TTFT 偏好，**不能**单独兑现「pro 用户尽量用实测最快账号」。多轮 Codex 会话会被 `previous_response_id` / session sticky 在打分之前锁死；top-K 轮盘赌会把分差压平；`top_k=1` 副作用大于收益。

---

## a) 结论

| 问题 | 判断 |
| --- | --- |
| 任务对代码的描述是否属实？ | **大体属实**，见下一节核对表。有几处需要修正。 |
| 方案 B 是否可行？ | **可行，作为第一阶段**。改动面小，接在现有 `openAIWS*ForRequest` 上。 |
| 能否达成「pro 尽量用实测最快号」？ | **不能单靠拉 TTFT 权重 + top_k=2。** 真实效果主要发生在「无 previous_response_id、且 sticky 未命中或已逃逸」的负载均衡层。Codex `/v1/responses` 多轮几乎整段锁在首轮选中的账号上。 |
| 是否建议做？ | **建议做，但按本文推荐设计扩范围**：JSON 覆盖而不是两列；pro 配 `top_k=2`（禁止 1）；TTFT 权重拉高同时保留 Load/ErrorRate；第二阶段再做分组 sticky 逃逸阈值和相对 TTFT 淘汰。 |
| 前置条件 | 全局 `openai_advanced_scheduler_enabled` 必须为 true。默认是 **false**（`setting_parse.go:253`）。开关关闭时根本不会走 TTFT 打分路径。 |

### 任务描述核对

| 描述 | 代码事实 |
| --- | --- |
| `Select` 入口约 :375，请求已带 `GroupID` | 属实。`Select` 在 `openai_account_scheduler.go:375`；`selectAccountWithSchedulerOnce:2390` 把 handler 传入的 `groupID` 写入 `req.GroupID`。 |
| 分层：previous_response → Guardian → session sticky → LB | 属实。`:389` / `:431` / `:448` / `:465`。 |
| sticky 逃逸默认 TTFT>15s 或错误率>50% | 属实。`openAIStickyEscapeConfig:2532-2560`，yaml 默认见 `deploy/config.example.yaml:409-416`。 |
| 打分公式含 Priority/Load/Queue/ErrorRate/TTFT/Reset/QuotaHeadroom/UpstreamCost + 粘性加成 | 属实。`:1011-1026`。 |
| TTFT 来自进程内 EWMA，`ReportResult:1813` | 属实。stats 是 `sync.Map`（`:184-289`），**不进 Redis，进程重启清零，多副本各算各的**。 |
| top-K 后加权轮盘赌 | 属实。`:792-844`，权重 `(score-minScore)+1` 是刻意压平分差。 |
| 权重/topK 有全局运行时覆盖，不区分分组 | 属实。`openAIWSSchedulerWeightsForRequest:2592`、`openAIWSLBTopKForRequest:2518` 都只读 `openAIAdvancedSchedulerRuntimeSettings`，**函数签名没有 GroupID**。 |
| Group 已有 `model_routing`、`rpm_limit`；账号-分组多对多 | 属实。`ent/schema/group.go:225-286`，`account_group.go` 另有 `priority` 默认 50。 |
| Claude `filterByMinPriority` 是硬分层，Codex 不是 | 属实。Claude：`gateway_scheduling.go:743-749` 先 `filterByMinPriority` 再 `filterByMinLoadRate`。Codex 高级调度把 priority **归一化进分数**（`:981-983`），不是硬过滤。 |

需要修正的描述：

1. **`filterByMinLoadRate` 不会在 Codex 路径上先淘汰快账号。** 它只在 Claude `GatewayService.selectAccountWithLoadAwareness`（`gateway_scheduling.go:749`）。OpenAI 路径用分数里的 `loadFactor`。
2. **probe 预算默认多数请求不生效。** `openAIAccountSelectionProbeLimit=64`（`:35`）只在 `openAICostOverflowExpanded`（`:1619`，依赖 `UseUpstreamTokenCost && weights.UpstreamCost>0` 且候选数 > topK）时 `enableLimit`。默认 `upstream_cost: 0`，pro 方案若不打开成本权重，这不是主风险。
3. **打分用的是 `accounts.priority`，不是 `account_groups.priority`。** `openAIAccountSchedulingPriority:623-628` 直接读 `account.Priority`。`queryAccountsByGroup` 虽按 `account_groups.priority` 排序（`account_repo.go:3097-3099`），高级调度随后重打分，中间表优先级被丢掉。运维如果只改「账号在某分组的优先级」，对 Codex 高级调度无效。
4. **HTTP `/v1/responses` 把 `previousResponseCanMove` 固定传 `false`**（`openai_gateway_handler.go:640`）。因此有 `previous_response_id` 时走硬亲和，**不会**因为 sticky weighted 而改成加权偏好。

---

## b) 风险与坑清单（带行号）

### 1. 方案 B 的真实效果：权重够不够「尽量最快」？

**不够，只是「略偏向更快」。**

默认权重（`deploy/config.example.yaml:396-401`，代码回落 `openai_account_scheduler.go:2578-2588`）：

`Priority=1, Load=1, Queue=0.7, ErrorRate=0.8, TTFT=0.5`

TTFT 只占总基础权重 4.0 里的 0.5。TTFT 因子还是 **候选池内 min-max 归一化**（`:987-990`）：

- 只有 0/1 个账号有 TTFT 样本，或所有样本相等：`ttftFactor` 恒为 **0.5**，TTFT 权重再大也 **零区分度**。
- 无样本账号同样是 0.5（冷启动/重启后全员中性）。

轮盘赌再压一次（`:808-814`）：

```
weight = (score - minScore) + 1.0
```

数值例子（两号其它因子都 0.7，只差 TTFT）：

| TTFT 权重 | 最快/最慢分 | 轮盘权重 | 第一枪打中最快号的概率（top_k=2） |
| --- | --- | --- | --- |
| 默认 0.5 | 2.95 / 2.45 | 1.5 / 1.0 | **60%** |
| 拉到 5.0 | 7.45 / 2.45 | 6 / 1 | **86%** |

所以「拉满 TTFT + top_k=2」能把新会话首枪从接近五五开拉到八成打中最快号，**不是 100%**。有 session_hash 时种子还是确定性的（`:763-789`），同一会话反复抽到同一排列；这和 sticky 叠加后，首轮一旦抽到次快号就会钉死。

`top_k=7`（默认，`:2511-2515`）时，第三、第四快的号仍在轮盘里，pro 体感会更散。

### 2. `top_k=1` 的副作用：不要用

`selectTopKOpenAICandidates` 在 `topK<=0` 时也会收成 1（`:706-708`）。`top_k=1` 时 `selectionOrder` 通常只有一个号：

1. **满载就排队，不改走次快号。** `tryAcquireOpenAISelectionOrderWithBudget:1176-1178` 发现 `CurrentConcurrency >= Concurrency` 会 `continue`；没有下一个候选后，`finishLoadBalanceSelectionFallback:1709-1717` 对 **同一个号** 建 `WaitPlan`。用户感知延迟 = 排队，可能比用稍慢的空闲号更差。这和「最快」目标相反。
2. **新会话羊群效应。** 确定性种子 + 单点 top-1 → 同一时刻所有新会话打同一号。Load 权重若远小于 TTFT（例如 TTFT=5、Load=1），负载从 0%→100% 只能扣 1 分，抵不过 TTFT 的 5 分，**几乎不会自我平衡**。
3. **429 单点。** 该号被 `BlockAccountScheduling(..., "429")`（`openai_account_runtime_block_fastpath.go:239`）后，候选过滤 `runtime_blocked`（`:1444-1446`）会把它整号摘掉，流量瞬间砸到「新的最快号」，形成 429 接力。top_k=2 时至少还能立刻打第二名。
4. **错误率/TTFT 反馈变慢。** 全集中在一个号上，其它号没有新样本，重启后更难重新发现谁快。

**pro 分组不要配 top_k=1。推荐 2，账号很多时最多 3。**

### 3. 粘性优先于负载均衡：老会话不换号（最大坑）

分层顺序（`Select:389-465`）：

1. **`previous_response_id` 硬亲和**（`:389-428`）  
   条件：`PreviousResponseID != ""` 且 OpenAI，且 `(!StickyWeighted || !PreviousResponseCanMove)`。  
   HTTP `/v1/responses` **永远** `previousResponseCanMove=false`（`openai_gateway_handler.go:640`）。Codex 多轮只要带 `previous_response_id`，**直接返回原账号，根本不打分、不看 TTFT 权重、不看 top_k、不走 sticky 逃逸。**
2. **Guardian 父号**（`:431-446`）同样硬粘。
3. **session_hash 硬粘**（`:448-458`），仅当 `StickyWeighted==false`（默认，`setting_parse.go:254`）。逃逸阈值 15s / 50%（`:555-563, 630-641`）。3s vs 400ms 的号 **都不会逃**。
4. 逃逸后把 `PreserveStickyBinding=true`（`:460-462`），负载均衡选中新号时 **不会改绑**（`:1225-1226`）。下一请求仍先打老号 → 再逃逸 → 再 LB。这是「临时逃逸」不是「迁到快号」。且有 `previous_response_id` 时连这步都走不到。
5. **StickyWeighted=true** 时跳过硬 sticky，但若粘性号已在 top-K 内，会被 **挪到 selectionOrder 队首**（`:1056-1071`），轮盘赌直接被旁路。粘性加分默认 `SessionSticky=3.0`、`Previous=5.0`（`:2587-2588`），比默认 TTFT=0.5 高一个数量级。

WS 还有第四层锁：建连选号后「连接内不重选号」（handler 注释，约 `:2510`）。整条 WS 会话钉死建连账号。

**含义：方案 B 只优化「无 previous_response_id 的首轮 / Messages / 粘性未命中」。对典型 Codex 多轮 HTTP 几乎无感。** 不要为了「换快号」轻易打开 `previous_response_id` 跨号迁移——工具续链会坏（WS 侧 `previousResponseCanMove` 已按 function_call_output 覆盖率计算，`:2314`）。

### 4. TTFT EWMA 冷启动 / 重启 / 多副本 / 混模型

- 存 `defaultOpenAIAccountScheduler.stats`（进程内 `sync.Map`，`:184-289, 2063-2067`）。
- `alpha=0.2`（`:235`）：约 10 次样本才把旧值降到 ~10%。慢号要靠自己被打到才能变「慢」。
- 无样本：`hasTTFT=false` → `ttftFactor=0.5`（`:987-990`）。重启后全员中性，首轮退回 priority/load/轮盘。
- **多副本各算各的「最快」**，调度结果在实例间不一致。
- **按 accountID 聚合，不按模型。** 快号跑慢模型会把 EWMA 拉脏。
- 失败路径 `ReportOpenAIAccountScheduleResult(..., false, nil)` 只更新错误率，不更新 TTFT（`:244` 要求 `firstTokenMs > 0`）。合理。
- Embeddings/alpha search 成功时经常 `firstTokenMs=nil`，这些流量不贡献 TTFT。

### 5. probe 预算

`openAIAccountSelectionProbeLimit=64`（`:35`）。`selectByLoadBalance` 里的 budget 默认不 limited，只有成本溢出扩 top-K 时才 `enableLimit`（`:1551-1552, 1619`）。`tryAcquireOpenAISelectionOrder` 自己会 `enableLimit`（`:1154-1156`），但主路径走的是 `WithBudget` 那个。

对方案 B：不是主风险。若将来同时打开 `upstream_cost` 且 top_k 很小、溢出列表很长，64 次 probe 可能打不到真正的快号。

### 6. 前置过滤会不会先淘汰快号？

| 过滤 | Codex 高级调度 | 会不会误杀快号 |
| --- | --- | --- |
| `filterByMinLoadRate` / `filterByMinPriority` | **不用**（Claude 专用，`gateway_scheduling.go:743-749`） | 否 |
| `listSchedulableAccounts` 按 GroupID | 只用该分组绑定账号（`openai_gateway_scheduling.go:1509-1514`） | 若快号没绑到 pro，永远选不到——这是运营问题，不是调度 bug |
| `RequirePrivacySet` | `:1451-1453` | 会。pro 若开隐私门，未 set 的快号被排除 |
| 利润门 `openAIProfitControlVetoReason` | `:1807-1808` | 会。高倍率快号可能被毛利率踢出 |
| 用户-分组-账号绑定 | `selectAccountWithScheduler:2156-2171` | 会。绑定集合更小；失败才 fallback 到整组 |
| `runtime_blocked`（429 冷却） | `:1444-1446` | 会暂时拿掉当前最快号，这是正确的安全阀 |
| `SubscriptionPriority` | `:1480-1517` | 会。先打 ChatGPT 订阅池，TTFT 只在该池内比；订阅号整体更慢时，pro 的 TTFT 权重帮不上 |
| `model_routing` | **OpenAI 调度不用**。`GetRoutingAccountIDs` 只在 Claude 路径且 `platform==anthropic`（`gateway_scheduling.go:277-279`） | 对 Codex 无交互。不要指望用模型路由给 pro 指定快号 |
| 渠道价限制 / 能力 / compact / 传输 | `:1797-1804, 1455-1461` | 缩小池子后再打分 |

### 7. 429 冷却如何与「快」交互

- 429 后 `BlockAccountScheduling`（`openai_account_runtime_block_fastpath.go:216-239`），调度侧整号 `runtime_blocked`。
- 冷却结束，**内存 TTFT EWMA 仍是「快」**（429 没写入 TTFT），ErrorRate 因 `success=false` 升高。若 ErrorRate 权重不够，刚解禁的快号会被立刻打满，再次 429。
- 因此 pro 配置必须 **保留甚至略提高 ErrorRate 权重**，不能只拉 TTFT、把 ErrorRate 降到 0。
- sticky 逃逸默认错误率 50%（`:2543-2548`），对 429 几乎无帮助——号已经不在池里，轮不到逃逸。

### 8. 其它实现陷阱

- **高级调度总开关默认关。** `getOpenAIAccountScheduler:2059-2060` 关则回落 `selectAccountWithLoadAwareness`（按 `account.Priority` + 负载排序，**无 TTFT**，`openai_gateway_scheduling.go:1334-1352`）。分组覆盖在关开关时等于没写。
- **分组覆盖函数当前不接收 GroupID。** 必须改 `openAIWSSchedulerWeightsForRequest` / `openAIWSLBTopKForRequest` 签名，并在 `buildOpenAIAccountLoadPlan:935, 1030` 传入 `req.GroupID`。
- **读分组请用 `GetByIDLite`。** 现有 privacy 路径用 `GetGroupByID`（`:1423-1424`），会额外打账号计数聚合（`group_repo.go:223-236`）。不要为覆盖配置再付一次。
- **字段要贯通：** `ent/schema/group.go`、`groupEntityToService`（`api_key_repo.go:957`）、`service.Group`、`admin_group` DTO/Update、`admin_group_duplicate.go`、`api_key_auth_cache.go`（若从 apiKey.Group 读）、GroupsView、ent 迁移。漏 duplicate/auth cache 是后续静默丢配置的典型坑。
- **全局运行时覆盖 vs 分组覆盖的优先级** 必须写死：建议 `分组覆盖 > 全局 DB 覆盖 > yaml 默认`，且仍受 `settings.enabled` 门控（与现注释 `:2521-2524, 2595` 一致）。
- 权重覆盖后必须 `IsValid()`（`:2600-2602`），全 0 会静默回落全局。

---

## c) 推荐最终设计

### 字段：用 JSON，不要两列

Group 上已有多块 JSON（`model_routing`、`messages_dispatch_model_config`、`models_list_config`）。两列 `lb_top_k_override` + `ttft_weight_override` 第一次迁移便宜，第二次（sticky 逃逸、相对淘汰、error_rate）还要再迁。

建议一列：

```text
groups.openai_scheduler_overrides JSONB  -- 空/NULL = 全部继承全局
```

对应 Go（指针 = 未设置则继承）：

```go
type GroupOpenAISchedulerOverrides struct {
    LBTopK                *int     `json:"lb_top_k,omitempty"`                 // 建议 2~3，禁止 1（或 1 时告警并当 2）
    WeightTTFT            *float64 `json:"weight_ttft,omitempty"`
    WeightErrorRate       *float64 `json:"weight_error_rate,omitempty"`
    WeightLoad            *float64 `json:"weight_load,omitempty"`              // 阶段 1 可先不暴露 UI
    StickyEscapeTTFTMs    *int     `json:"sticky_escape_ttft_ms,omitempty"`    // 阶段 2
    StickyEscapeErrorRate *float64 `json:"sticky_escape_error_rate,omitempty"` // 阶段 2
    TTFTMaxRatio          *float64 `json:"ttft_max_ratio,omitempty"`           // 阶段 2：> minTTFT*ratio 则移出 top-K（无样本不淘汰）
}
```

**不要**在阶段 1 做 Claude 式 `filterByMinPriority` 硬分层来模拟「快号」——那是人工优先级，不是实测 TTFT。

**不要**为了「换快号」默认打开 `previous_response_id` 跨账号迁移。

### 函数改动点

1. `openAIWSLBTopKForRequest(ctx, groupID *int64) int`  
   `openAIWSSchedulerWeightsForRequest(ctx, groupID *int64) ...`  
   以及 `openAIStickyEscapeConfig()` 在阶段 2 改为 `...(ctx, groupID)`。
2. `buildOpenAIAccountLoadPlan`（`:935, 1030`）把 `req.GroupID` 传进去。
3. 解析：`GetByIDLite` → 覆盖 merge → 仍走 `IsValid()`；非法覆盖回落全局并打 warn。
4. 阶段 2 在 `selectTopKOpenAICandidates` 之前加相对 TTFT 过滤：`hasTTFT && minTTFT>0 && ttft > minTTFT*ratio` 则移出主池（无样本保留，作为探索）。
5. `Select` 里 sticky 逃逸改读分组阈值（`:555`）。**不要**在阶段 1 改 `PreserveStickyBinding` 语义。
6. 管理端：`admin_group` API + `GroupsView.vue` 仅对 openai/composite 分组展示；校验 `lb_top_k>=2`（或 1 时强制升 2 并提示）。
7. 测试：  
   - 有分组覆盖 vs 无覆盖回落  
   - 总开关关闭时忽略分组覆盖  
   - 权重全 0 回落  
   - top_k=2 + 高 TTFT 时第一枪分布  
   - 有 `previous_response_id` 时覆盖不影响层 1  
   - duplicate group 拷贝 overrides  
   - 无 TTFT 样本时 top-K 不塌缩

### pro 分组推荐配置（阶段 1）

在 **高级调度已开启** 的前提下：

| 项 | 值 | 原因 |
| --- | --- | --- |
| `lb_top_k` | **2** | 86% 级偏好 + 满载/429 时有第二枪；避免 top_k=1 排队 |
| `weight_ttft` | **4.0 ~ 5.0** | 相对默认 0.5 拉开 min-max 分差 |
| `weight_error_rate` | **1.0 ~ 1.2** | 解禁后的 429 号不要立刻被 TTFT 重新打满 |
| `weight_load` | 保持 **1.0**（不要 0） | 唯一的反羊群力 |
| sticky weighted | **保持关** | 打开后粘性号会被塞进队首，抵消 TTFT |
| subscription priority | 看池子；若订阅号整体更慢，pro **不要开** | 先切池再打分 |

运营侧仍建议：pro 绑定的账号池尽量本身就是快号（多对多可以共享，但不要把明显慢号绑进来）。调度画像是池内排序，不是从全站捞快号。

### 与现有机制的优先级（写进注释/spec）

```
利润门 / PrivacySet / 用户-账号绑定 / 429 runtime block / 能力与渠道限制
    → 得到候选池
previous_response_id 硬亲和 / Guardian / session sticky（15s 逃逸）
    → 命中则返回
分组 openai_scheduler_overrides（topK + 权重 [+ 阶段2 相对淘汰]）
    → 仅作用于 LB 打分
全局 advanced scheduler DB 覆盖
    → yaml 默认
```

`model_routing` 对 Codex 无作用；不要把它当 pro 快号白名单。

---

## d) 分阶段落地路径

### 阶段 0 — 确认前提（不写代码，先看生产）

1. 生产是否已打开 `openai_advanced_scheduler_enabled`。没开则方案 B 零效果，应先开高级调度再谈分组画像。
2. pro 与其它分组是否共享账号池。共享则分组覆盖有意义；若 pro 已经只绑快号，收益主要是池内再排序。
3. 看调度指标：`StickyHitRatio`、`LoadBalanceSelectTotal`（`SnapshotMetrics:1827`）。若 sticky/previous 命中率很高，阶段 1 的用户体感会偏弱，必须做阶段 2。

### 阶段 1 — 最小可用（建议本次实现范围）

- Group JSON `openai_scheduler_overrides`：先暴露 `lb_top_k`、`weight_ttft`、`weight_error_rate`。
- 改两个 `ForRequest` + `buildOpenAIAccountLoadPlan` 按 GroupID merge。
- 迁移 + admin API + GroupsView（openai/composite）。
- 单测覆盖开关门控、回落、duplicate。
- pro 配 top_k=2、TTFT 4~5、ErrorRate ≥1。
- **验收口径：** 新 session（无 `previous_response_id`）的选号 TTFT 分布左移；**不要**用多轮会话 TTFT 当验收——那一层还没动。

### 阶段 2 — 让「尽量最快」覆盖已有会话的可迁移部分

- 分组 `sticky_escape_ttft_ms`（pro 建议 2000~3000，而不是 15000）和 `sticky_escape_error_rate`（建议 0.2~0.3）。
- `ttft_max_ratio`（建议 2.0）：比组内最优慢两倍的有样本号移出 top-K；无样本保留。
- 明确 **不** 改 HTTP `previousResponseCanMove=false`。多轮 Codex 仍锁号；阶段 2 只改善「无 previous_response_id 的粘性会话」（Messages、部分 chat、粘性查找失败）。
- 可选：逃逸成功后是否改绑。改绑能让后续请求留在快号，但和官方「临时逃逸、避免短突发迁到冷号」注释（`openai_gateway_scheduling.go:1198-1202`）冲突，需要产品拍板。

### 阶段 3 — 测量基础设施（否则「实测最快」长期不准）

- TTFT EWMA 按 `(accountID, canonicalModel)` 分桶，或至少按模型族。
- 多副本共享：Redis 或定期从 usage_log 回填，避免重启/多实例各执一词。
- 管理端按分组预览分数（已有全局 scheduler score 快照思路，补 GroupID）。
- 观测：按 group 打点 `layer`、选中账号 TTFT、top1 命中率、429 后切换。

### 明确不做

- 不为「快」把 `top_k` 设成 1。
- 不把 Claude `filterByMinPriority` 搬到 Codex。
- 不把 `account_groups.priority` 偷偷改成打分输入（若要做，应单开任务，且和 `accounts.priority` 的语义先对齐）。
- 不靠 `model_routing` 服务 Codex。
- 不在利润门/PrivacySet 之前插入 TTFT 过滤。

---

## 一句话给决策

**做分组 JSON 覆盖，pro 配 top_k=2 + 高 TTFT/错误率权重；这能让新会话更常打到快号。** 若目标包含「老 Codex 多轮也换到快号」，必须另做 sticky 逃逸（阶段 2），并且接受 `previous_response_id` 硬亲和在 HTTP 路径上仍然锁号——那是正确性约束，不是调度权重能绕开的。
