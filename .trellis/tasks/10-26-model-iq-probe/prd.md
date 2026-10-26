# 糖果问题探针与智商监控

## Goal

以"糖果问题"（标准答案 **21**，已由操作者确认）作为默认探针题，主动探测渠道上 GPT 系模型是否被降智：

- 用户端渠道状态页（`/monitor` V1）在"模型可用性"卡片网格下方新增"糖果探针"区块，按模型汇总展示最近探测结果。
- 管理端新增"智商监控"页面（侧边栏"渠道管理"分组内、与"渠道监控"同级），支持配置问题与期望答案、选择探测目标（账号×模型 / 仅按模型）、定时+手动触发、查看明细并人工改判。

## Background

- 糖果问题是社区常用的 GPT 降智检测题：降智模型常给出 20、23、29 等忽略"形状靠手感可分辨（可自选圆/星数量）"的错误答案；正常模型应答 21（摸 12 个五角星 + 9 个圆形）。
- 现有基础设施：`channel_monitor_challenge.go` 已有"固定题 + 期望答案 + 响应中找整数"的判分先例；上游中继探针已有"按账号×模型发固定 prompt"经 `AccountTestService.RunAccountTestBackground` 执行的先例；`gpt_intelligence_templates` 已有管理端可编辑题库存 settings 表的先例。

## Confirmed Facts

- 默认题面 = 操作者提供的糖果问题原文（苹果/桃子/西瓜 × 圆形 7/9/8、五角星 7/6/4），默认期望答案 `21`。
- 判定规则（操作者选定）：**出现即通过**——响应文本中出现期望答案（数字按独立整数 token 匹配，与 challenge 判分一致）即 pass；期望答案非数字时退化为大小写不敏感的包含匹配。
- 触发方式（操作者选定）：定时（间隔可配，默认 360 分钟）+ 管理端手动"立即探测"。
- 展示粒度（操作者选定）：用户端按模型汇总、不暴露账号信息；管理端展示模型 + 账号（邮箱脱敏）明细。
- 探测目标（操作者选定）：管理端可选 `account_model`（账号×模型）或 `model_only`（每模型任选一个可用账号）；模型列表为空时自动发现 OpenAI 账号上的 `gpt-*` 模型。

## Requirements

- R1 后端配置：新 setting key `model_iq_probe_config`（JSON），含 enabled/question/expected_answer/interval_minutes/target_mode/models/account_ids/max_tokens，默认值如上；管理端 `GET/PUT /api/v1/admin/model-iq-probe/config`，带校验。
- R2 后端存储：新表 `model_iq_probe_results`（ent schema + SQL 迁移），记录账号、模型、协议、判定状态、期望/实际答案（截断 ≤4KB）、耗时、token 用量、人工改判字段；保留 30 天自动清理。
- R3 后端执行器：`ModelIQProbeRunner` 定时按配置解析目标（账号×模型或每模型单账号），经 `AccountTestService` 发探针题，判分写入结果表；leader lock 防多实例并发；手动触发端点复用同一执行通道。
- R4 后端查询：用户端 `GET /api/v1/channel-monitors/iq-probe` 返回按模型汇总（最新判定 + 最近 10 次）；管理端分页查询结果（过滤 model/account/status，邮箱脱敏）；人工改判端点。
- R5 前端用户端：`CandyProbePanel` 区块插入 `ChannelStatusV1View.vue` 的 MonitorCardGrid 与 GptIntelligencePanel 之间；每模型一行：判定徽章 + 实际答案摘要 + 相对时间 + 最近 10 次圆点时间线；空数据不渲染；风格与现有面板一致。
- R6 前端管理端：`ModelIqMonitorView.vue`（路由 `/admin/channels/iq-monitor`，侧边栏"渠道监控"之后），双 Tab：探测配置（表单）+ 探测结果（筛选/表格/分页/立即探测/人工改判/自动刷新）。
- R7 i18n：zh/en 同步新增 `nav.modelIqMonitor`、`channelStatus.candyProbe.*`、`admin.modelIqMonitor.*`；`pnpm check:i18n` 通过。

## Acceptance Criteria

- [ ] 管理端保存配置后，`立即探测`能触发一轮探测并产生结果记录（含正确答案 21 → pass）。〔代码已实现；需要带真实 OpenAI 账号的运行环境做人工验收〕
- [ ] 定时器按配置间隔自动执行；enabled=false 时不执行，用户端接口返回空。〔代码已实现；轮次执行需运行环境人工验收，enabled=false → 空列表已由单测覆盖〕
- [x] 用户端渠道状态页模型可用性下方出现糖果探针区块，仅当有数据时渲染，不暴露账号信息。〔CandyProbePanel 组件测试覆盖空态/徽章/零账号信息断言〕
- [x] 管理端结果表展示模型+账号明细，支持过滤/分页/改判；账号信息不出现在任何用户端响应中。〔repo 集成测试 + userSummary 测试覆盖；实现用 account_name（管理端本就可见，无需脱敏）〕
- [x] `go build ./...`、`go test ./...`（相关包）、`pnpm typecheck`、`pnpm check:i18n`、相关 vitest 全部通过。〔service 包仅存量失败 TestOpenAICodexTicketGate_CompactRequestUsesForwardOutboundModel，clean custom/main 同样失败，与本任务无关〕
- [x] 全程在 `feature/model-iq-probe` 分支（基线 custom/main），不 push、不部署。

## Notes

- 范围外：V2 渠道状态页不加区块；不改动现有 V1/V2 监控与 challenge 逻辑；不部署 OVH。
- 高风险操作（commit/push/迁移执行）按 downstream-fork-workflow.md 需操作者确认。
