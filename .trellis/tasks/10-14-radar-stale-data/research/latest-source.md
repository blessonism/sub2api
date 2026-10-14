# CodexRadar 最新数据入口复核（2026-10-14）

旧 efficiency JSON 的 source_updated_at=2026-10-01，只能证明该静态快照的时间，不能作为整站最新评测日期。此前验收中的“公开源最新评测为 10 月 1 日”结论不完整。

原站首页智力区现已接入 RadarBench，通过 `/assets/radar-bench-display.mjs?v=bench-read-budget-v2` 读取 `/data/radar-bench-binding.json`，再按模型/档位 GET `/api/radar-bench-score?model=...&effort=...&view=summary`。公开 summary 链接要求 `selection=user-selected-v1`；原站用户选择配置时使用等价的 `X-Radar-Bench-Selection: user-selected-v1` 请求头。没有使用第三方私有凭据。

## 已验证的新数据

| 模型/档位 | 分数（满分 150） | 覆盖数 | 判分日（UTC） | 状态 |
| --- | --- | --- | --- | --- |
| GPT-6.1 Sol max | 106.7214965328173 | 53/64 | 2026-10-13 | provisional |
| GPT-6.1 Sol ultra | 111.52281652695919 | 46/64 | 2026-10-13 | provisional |
| GPT-6 Astra max | 113.67307692307693 | 63/64 | 2026-10-13 | provisional |
| GPT-5.6 Sol max | 75.85923493457054 | 49/64 | 2026-10-13 | provisional |

这些是实际公开判分日桶中的成绩，不能表述为 10 月 14 日重新测得，也不能声称已穷尽所有配置。部分请求存在超时/EOF；未成功读取不等于没有数据。

## 新契约

- schema: `radar-bench-public-summary-v5-150-daily-graded-count`。
- score_version: `radar-bench-v2-150`，score_max: 150。
- benchmark: `radar-bench-final64-20261004`，来源为 deepswe、pompeii、tb4、science 四类任务。
- scoring_policy_version: `radar-bench-v2-last3-per-task-equal-weight-150`；展示原站总分，不自行换算为旧 IQ。
- `coverage` / `required_tasks` / `score_status` 必须随分数展示，provisional 表示任务覆盖尚未完整。
- summary 的 `history.updated_bucket` 和 `history.points[].at` 是 UTC 日桶，不是采集时间或精确判分时刻。
- 本任务已在本地接入 binding 与 RadarBench summary，停止旧静态源 runtime 采集；生产仍运行原镜像，待提交和发布确认。
- 仅提高旧 JSON 上限不足以完成最新来源升级。本地实现与 UI 已使用新评分口径，不拼接旧曲线，也不降级旧 IQ。

## 检查限制

`/api/intelligence-efficiency-metrics` 从本地与 OVH 均返回 502；旧非 summary 的 `/api/radar-bench-score` 返回 410。正式公开 summary 可用。原站 `/data/radar-bench-last-known.json` entries 为空，不能拿它冒充最新结果。

原始公开响应的摘要、URL 与 SHA256 在 `radar-bench-observations.json`。本站发布状态没有变化。

## 最终动态采集与回放

2026-10-14 从 OVH 只读抓取目录及公开 summary，34 个配置中 29 个成功有成绩、5 个超时，耗时 17.38 秒；覆盖 6 个模型，包含 6.1 Sol、6 Astra、6 Sol/Luna 和 5.6 Sol/Luna。没有固定模型名单，旧型号只要来源有成绩仍展示。

真实捕获通过当前服务完整采集/归一化路径回放：29 个系列，最新判分日分布为 10 月 11 日 1 个、10 月 13 日 22 个、10 月 14 日 6 个，部分失败缓存 60 秒。超时不是没有成绩，未声称穷尽所有配置。必要字段与原始捕获 hash 见 `radar-bench-live-source.json`，回放输出见 `../live-radar-bench-replay.json`。
