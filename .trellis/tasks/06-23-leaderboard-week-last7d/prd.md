# brainstorm: 排行榜周榜与最近7天口径

## Goal

让普通用户排行榜明确区分“本周”和“最近 7 天”，同时为管理员 Token 排行榜增加一个“周榜”快捷入口，降低切换本周时间范围的操作成本。

## Requirements

- 普通用户排行榜保留日榜，并新增三个明确周期中的后两项：
  - `week` 表示当前自然周，按用户时区从周一 00:00 到下周一 00:00 统计。
  - `last7d` 表示最近 7 个自然日，保留原来的滚动 7 天口径。
- 普通用户排行榜前端展示 `日榜 / 周榜 / 近 7 天`，范围文案、空状态文案与周期语义保持一致。
- 管理员 Token 排行榜不改后端查询模型，继续使用现有 `start_date/end_date` 时间范围。
- 管理员 Token 排行榜在时间范围控件右侧新增“周榜”快捷按钮，点击后把日期范围设置为当前自然周并刷新。

## Acceptance Criteria

- [ ] 普通用户请求 `period=week` 时，后端查询时间范围为当前自然周。
- [ ] 普通用户请求 `period=last7d` 时，后端查询时间范围为最近 7 个自然日。
- [ ] 普通用户页面能切换日榜、周榜、近 7 天，并显示正确范围/空状态文案。
- [ ] 管理员 Token 排行榜点击“周榜”后，使用当前自然周日期范围刷新数据。
- [ ] 管理员 Token 排行榜不引入新的后端 period API。

## Definition of Done

- 相关后端单测覆盖普通用户 `week` 与 `last7d` 周期。
- 相关前端 API 测试覆盖新周期参数。
- 运行与本次改动相关的 Go/Vitest 检查。

## Technical Approach

- 后端在 `userTokenLeaderboardRange` 中新增 `last7d` 周期，`week` 改为调用自然周范围函数。
- 前端普通用户 API 类型扩展为 `day | week | last7d`，页面选项和文案同步。
- 管理员页面新增本地日期工具函数，按钮只更新 `startDate/endDate` 并触发现有加载逻辑。

## Out of Scope

- 不调整管理员排行榜后端统计模型。
- 不改变管理员 DateRangePicker 组件的全局预设。
- 不修改排行榜统计 SQL 或排名算法。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，本仓库为 `Wei-Shaw/sub2api` 下游二开。
- 已读取 `.trellis/spec/backend/quality-guidelines.md`，普通用户可见排行榜需保持邮箱脱敏与用户侧 API 契约。
- 已读取 `.trellis/spec/frontend/type-safety.md`，用户仪表盘 API 类型需要与后端 JSON 字段保持一致。
