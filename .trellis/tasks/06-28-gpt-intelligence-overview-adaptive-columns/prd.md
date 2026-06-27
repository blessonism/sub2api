# brainstorm: GPT 智商测试概览图自适应列数

## Goal

让 GPT 智商测试概览区域的模型分数卡片根据实际数据数量自适应列数：4 条数据时一行 4 列，5 条数据时一行 5 列，同时保留窄屏换行能力。

## What I already know

- 用户确认希望概览图根据数据变动调整列数。
- 当前实现位于 `frontend/src/components/user/monitor/GptIntelligencePanel.vue`。
- 卡片数据来自 `comparisonSeries`，由当前探针和 `snapshot.comparisons` 拼接生成，数量会随接口数据变化。
- 现有模板固定使用 `xl:grid-cols-4`，5 条数据时会变成 4+1。

## Requirements

- 桌面宽屏下按 `comparisonSeries.length` 生成 1-5 列。
- 4 条数据时保持一行 4 列。
- 5 条数据时显示一行 5 列。
- 窄屏继续按 1 列/2 列响应式展示，避免卡片挤压。
- 不改变后端数据结构和图表计算逻辑。

## Acceptance Criteria

- [ ] `GptIntelligencePanel` 的概览卡片网格不再写死 4 列。
- [ ] 单元测试覆盖 4 条与 5 条 series 的布局 class。
- [ ] 目标测试通过。

## Definition of Done

- Tests added/updated where appropriate.
- Lint/type risk considered for the touched frontend code.
- Changes remain scoped to GPT intelligence overview layout.

## Out of Scope

- 不调整 GPT 智商测试 API。
- 不重设计图表或分数卡视觉样式。
- 不处理其它监控面板布局。

## Technical Notes

- Semble CLI 当前不可用，已按项目降级规则使用 `rg` 定位代码。
- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，任务 base branch 设置为 `custom/main`，目标分支记录为 `feature/gpt-intelligence-overview-adaptive-columns`。
