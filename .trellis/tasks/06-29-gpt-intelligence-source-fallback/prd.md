# brainstorm: 修复 GPT 智力检测加载失败

## Goal

渠道状态页中的 GPT 智力检测不应因为外部公开 JSON 结构变化而不可见。需要由本项目后端采集公开页面中已经渲染的模型 IQ 数据，缓存并归一化为前端现有结构，让前端不再直接依赖 `current.json`。

## What I already know

* 用户反馈本地可以直接打开 `https://codexradar.com/current.json`，但渠道状态页看不到 GPT 智力检测。
* 当前前端直接请求 `https://codexradar.com/current.json`，并要求响应里存在 `model_iq`。
* 当前公开 JSON 返回 `schema_version: "2.0"`、`type: "public_summary"` 和 `api_access.full_api_status: "authorization_required"`，不再包含 `model_iq`。
* 公开首页 HTML 中仍渲染了当前 IQ 曲线数据，`<title>` 内包含日期、模型、推理强度、IQ 指数、通过题数、费用、耗时、cache 命中率等字段。
* 渠道状态基础卡片走系统后端接口，和 GPT 智力检测外部源加载失败相互独立。
* 本仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，改动需要遵守下游 fork 工作流。

## Assumptions (temporary)

* 本任务不直接内置任何第三方授权 Key。
* 本轮只采集公开 HTML 已渲染的数据，不调用需要授权的完整 API。
* 公开 HTML 里没有 token 细分和 quota radar 字段时，这些字段可以返回 `null`，前端按现有兜底展示。

## Open Questions

* 无。

## Requirements (evolving)

* 前端通过本项目后端接口加载 GPT 智力检测快照，不再直接请求 `https://codexradar.com/current.json`。
* 后端只采集公开首页 HTML 中已渲染的模型 IQ 数据，并缓存结果，避免高频请求第三方站点。
* 后端将公开 HTML 数据归一化为前端已有 `GptIntelligenceSnapshot` 结构。
* 若公开页面结构变化导致解析失败，返回明确错误，渠道状态基础视图仍正常展示。
* 后续如接入授权 API，应继续由后端持有 Key，前端不得直接暴露 Key。

## Acceptance Criteria (evolving)

* [x] 当前公开 JSON 缺少 `model_iq` 时，渠道状态页仍可通过后端采集接口展示 IQ 趋势。
* [x] 现有 `model_iq` 结构仍可正常解析和展示。
* [x] 用户能区分“渠道状态正常”和“GPT 智力检测外部指标不可用”。
* [x] 针对解析/降级行为有测试覆盖。

## Definition of Done

* Tests added/updated where appropriate.
* Lint/typecheck impact considered for touched packages.
* Rollout/rollback risk considered.
* 下游 fork 工作流约束已纳入任务上下文。

## Out of Scope

* 获取或提交 Codex Radar 授权 Key。
* 绕过第三方授权限制抓取完整 API。
* 搭建本项目自己的 DeepSWE 评测调度器、题库和评分器。
* 改动渠道状态核心监控逻辑。

## Technical Notes

* 前端 API：`frontend/src/api/gptIntelligence.ts`
* 页面入口：`frontend/src/views/user/ChannelStatusView.vue`
* 展示组件：`frontend/src/components/user/monitor/GptIntelligencePanel.vue`
* i18n：`frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`
* 测试：`frontend/src/components/user/__tests__/GptIntelligencePanel.spec.ts`
* 下游 fork 规则：`.trellis/spec/guides/downstream-fork-workflow.md`

## Technical Approach

* 新增后端 GPT 智力检测采集服务：使用短超时 HTTP 请求拉取公开首页 HTML，解析 `model-iq-score` 区域中 SVG `<title>` 数据。
* 服务层做 1 小时内存 TTL 缓存和 singleflight，减少第三方请求频率。
* 新增认证用户接口：`GET /api/v1/channel-monitors/gpt-intelligence`。
* 前端 `fetchGptIntelligenceSnapshot` 改为请求本机接口，保留旧 `parseGptIntelligenceSnapshot` 用于兼容测试和后续来源适配。

## Decision (ADR-lite)

**Context**: `current.json` 已改为公开摘要，不再提供页面当前需要的 `model_iq` 结构；但公开首页仍渲染了模型 IQ 趋势。

**Decision**: 本轮优先实现后端公开 HTML 采集 + 缓存 + 旧 DTO 归一化，恢复现有页面展示。

**Consequences**: 该方案比正式 JSON API 更脆弱，需通过缓存、解析失败兜底和测试降低风险；真正完全独立的评测系统留到后续任务。
