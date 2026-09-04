# OpenAI 分组级调度画像(top_k/TTFT 权重覆盖 + previous_response 有条件逃逸)

## Goal

让 pro 分组的用户尽量调度到实测最快(TTFT)的 OpenAI/Codex 中转账号:

1. **分组级调度覆盖**:分组可覆盖高级调度器的 `lb_top_k`、TTFT/错误率/负载权重,并支持按分组把"明显慢于组内最优"的账号移出 top-K 池。
2. **apikey HTTP previous_response_id 有条件逃逸**:纯中转场景下多轮会话被 HTTP 硬亲和锁在首轮账号上;在工具续链可自包含重建(canMove)且绑定账号实测明显慢时,允许换号重选,转发前剥离 `previous_response_id`,成功后改绑新 resp_id。

设计依据:`.trellis/workspace/openai-group-scheduler-profile-review.md` 与 `openai-group-scheduler-profile-review-relay.md`(Orca/grok 两轮评审,协调者抽查核实)。

## 背景

本部署上游几乎 100% 是别人的 sub2api 中转(apikey 型账号),不同线路实测 TTFT 差 5~40 倍。现状问题:

- Codex 侧高级调度器(TTFT EWMA 打分 + top-K 轮盘赌)只有全局权重/top_k 配置,无法按分组差异化;
- HTTP `/v1/responses` 把 `previousResponseCanMove` 写死 `false`(`openai_gateway_handler.go`),带 `previous_response_id` 的多轮请求直接硬亲和返回,不看 TTFT、不走 LB——apikey 账号专属行为,纯中转场景下几乎每段多轮都锁死在首轮抽中的那把 key。

## Requirements

### 阶段 1:分组级调度覆盖

- `Group` 新增 JSON 字段 `openai_scheduler_overrides`(空/NULL = 全部继承全局),结构:
  - `lb_top_k *int`(禁止 1:配置为 1 时按 2 处理并打 warn)
  - `weight_ttft *float64`
  - `weight_error_rate *float64`
  - `weight_load *float64`
  - `ttft_max_ratio *float64`(相对淘汰:> minTTFT*ratio 且有样本的账号移出 top-K 前的主池;无样本保留)
- 优先级:`分组覆盖 > 全局 DB 覆盖 > yaml 默认`;仍受 `openai_advanced_scheduler_enabled` 总开关门控;非法覆盖(全 0 等)回落全局并 warn。
- 读取点改为按 `req.GroupID` 传入:`openAIWSSchedulerWeightsForRequest` / `openAIWSLBTopKForRequest`;`buildOpenAIAccountLoadPlan` 传入 GroupID。读分组用 `GetByIDLite`。
- 相对 TTFT 淘汰在 `selectTopKOpenAICandidates` 之前执行。
- 管理端:分组创建/更新 API 透传该字段;group duplicate 复制该字段;GroupsView.vue 对 openai/composite 平台分组展示配置表单。
- 字段贯通:`ent/schema/group.go`、service Group、group repo/entity 映射(含 Lite)、admin DTO。

### 阶段 2:apikey HTTP previous_response_id 有条件逃逸

- HTTP 转发链路计算 `canMove = !HasFunctionCallOutput || ContextCoversAllCallIDs`(复用 `AnalyzeToolCallOutputContextCoverageBytes`),替换写死的 `previousResponseCanMove=false`。
- `Select` 层 1(previous_response 亲和)新增 apikey 专用口子:账号为 `IsOpenAIApiKey()` 且 `canMove` 且绑定账号超出分组逃逸阈值(绑定号 `hasTTFT && ttft > minTTFT*ttft_max_ratio` 或 TTFT > `sticky_escape_ttft_ms` 或错误率 > `sticky_escape_error_rate`)时不硬返回,落入 LB;转发前 `RemovePreviousResponseIDFromBody`,成功后绑定新 resp_id(改绑,不是临时逃逸)。
- `canMove=false` 行为与现状完全一致(硬锁)。
- **不做**:不打开全局 `StickyWeighted`;不把 resp_id 原样跨 key 转发;不为快改 429 分类/health breaker;不给中转开 WSv2。

## Acceptance Criteria

- [ ] 分组无覆盖时,调度行为与现状完全一致(全局 DB 覆盖/yaml 默认路径不变)。
- [ ] 分组覆盖生效:pro 类分组配 `lb_top_k=2 + weight_ttft=4` 时,新会话(无 previous_response_id)打分与 top-K 明显偏向低 TTFT 账号;`ttft_max_ratio` 把超阈值账号移出主池(无样本不淘汰)。
- [ ] `lb_top_k=1` 被升为 2 并告警;总开关关闭时分组覆盖被忽略;权重全 0 回落全局。
- [ ] 带 `previous_response_id` 的 apikey 请求:canMove=false 时锁号不变;canMove=true 且绑定号不超阈值时仍硬亲和;超阈值时换号且请求体剥离 resp_id,成功后新 resp_id 绑定到新账号。
- [ ] OAuth 账号 HTTP + previous_response_id 行为不变(仍跳过 failover)。
- [ ] 分组 duplicate 正确复制 overrides;auth cache/DTO 贯通。
- [ ] `go build ./...`、`go test ./internal/service/... ./internal/handler/...` 相关包通过;前端 type-check/lint 通过。

## Notes

- 分支 `feature/openai-group-scheduler-overrides` 从 `custom/main` 拉出,PR 目标 `custom/main`(遵守 downstream-fork-workflow)。
- 工作在独立 worktree `/Users/suki/code/sub2api-scheduler-profile`,避免与主工作区进行中的 channel_monitor 改动(ent 生成文件)交叉。
