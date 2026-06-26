# brainstorm: GPT 智力检测与渠道监控集成

## Goal

在现有网站中增加一个 GPT 智力检测/模型状态观察能力，优先考虑与现有渠道监控页面结合，让管理员或用户能直接看到外部信息源提供的 GPT 当前能力状态，并辅助判断渠道可用性或质量波动。

## What I already know

* 用户希望新增一个“GPT 的智力检测页面”。
* 用户认为体验可以类似现有“渠道监测”。
* 用户提供了直接信息源：`https://codexradar.com/current.json`。
* 用户倾向于将该能力直接结合在渠道监控页面中，而不是先做完全独立入口。
* 本仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，代码改动需遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
* 现有用户侧渠道监控路由为 `/monitor`，页面文件为 `frontend/src/views/user/ChannelStatusView.vue`。
* 现有渠道监控页面由 `MonitorHero`、`MonitorCardGrid`、`MonitorDetailDialog` 等组件组合而成，适合在 Hero 和渠道卡片网格之间插入模型 IQ 概览模块。
* `https://codexradar.com/current.json` 返回 `Access-Control-Allow-Origin: *`，MVP 可由前端直接 `fetch`。
* 信息源中与“智力检测”最相关的是 `model_iq` 字段，包括 `latest`、`recent_days`、`comparisons`、`quota_radar` 等数据。
* 信息源同时包含 Codex reset、社区压力、环境信号等内容，前端应筛选展示，不应把完整 JSON 原样暴露给用户。

## Assumptions (temporary)

* 该功能主要是前端展示外部公开 JSON 信息，不一定需要新增数据库表。
* 如果未来需要审计、缓存、内网部署或隐藏外部依赖，可能需要补后端代理接口。
* “智力检测”更接近模型能力/表现状态看板，而不是本系统主动发起测试请求。

## Open Questions

* 该能力在 MVP 中应只展示外部信息源当前快照，还是也要结合本系统渠道做逐渠道对比？

## Requirements (evolving)

* 能展示外部信息源中的 GPT 当前状态信息。
* 与现有渠道监控页面的导航和视觉结构保持一致。
* 外部信息源不可用时需要有清晰的降级展示。
* 优先展示 `model_iq.latest` 的分数、状态、通过题数、测试模型、推理强度、耗时和成本。
* 展示 `model_iq.recent_days` 的近期趋势，用于快速判断模型质量是否波动。
* 可选展示 `model_iq.comparisons` 中不同模型/推理强度的对比摘要。
* 手动刷新渠道监控时，应同步刷新模型 IQ 数据；自动刷新可复用现有页面节奏。

## Acceptance Criteria (evolving)

* [ ] 用户可以在渠道监控相关页面看到 GPT 智力/状态检测信息。
* [ ] 页面展示字段来自 `https://codexradar.com/current.json`，并避免把原始 JSON 直接暴露成难读内容。
* [ ] 数据加载失败、格式异常、空数据时都有可理解的页面状态。
* [ ] 新增逻辑不影响现有渠道监控页面的核心功能。
* [ ] 页面在桌面和移动端都不会出现文字溢出或卡片布局错位。

## Definition of Done (team quality bar)

* Tests added/updated where appropriate
* Lint / typecheck / CI green
* Docs/notes updated if behavior changes
* Rollout/rollback considered if risky

## Out of Scope (explicit)

* 暂不默认实现完整历史趋势存储。
* 暂不默认实现主动调用 OpenAI/GPT 进行真实题目测试。
* 暂不默认改变渠道路由、计费或优先级策略。
* 暂不默认把 Codex reset 预测、Tibo presence 或社区帖子流全部纳入渠道监控页面。
* 暂不默认新增后端数据库表或定时任务。

## Technical Notes

* 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
* 任务 base branch：`custom/main`。
* 任务建议工作分支：`feature/gpt-intelligence-monitor`。
* 当前工作区实际分支为 `feature/upstream-cost-calibration`，且存在较多未提交改动；实现前需避免混入该任务。
* 候选实现方式：
  * 前端直连外部 JSON：改动小、上线快，依赖外部源 CORS 和浏览器可访问性。
  * 后端代理外部 JSON：可做缓存、超时控制和统一错误格式，但改动范围更大。
* 推荐 MVP：先做前端直连的 `ModelIqPanel`/`GptIntelligencePanel`，挂载到 `/monitor` 页面；后续再按稳定性需求补后端代理。
