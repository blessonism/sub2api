# 上游监控历史用量显示额度折算

## Goal

在上游监控的历史用量 tab 中，基于现有历史记录的 `actual_cost` 与 `total_tokens`，新增一列展示每 1 额度约等于多少 M token，帮助快速判断不同上游分组的额度折算效率。

## Requirements

- 历史用量表格新增“每额度 M Token”列。
- 折算公式为 `total_tokens / actual_cost / 1_000_000`。
- 当额度为空、为 0、非有限数，或 token 非有限时显示 `-`，不把未知值伪装成 0。
- 中英文 locale 均补齐新增列文案。

## Acceptance Criteria

- [ ] 历史用量 tab 的表头和每行记录都展示新增折算列。
- [ ] `actual_cost <= 0` 或异常数据时展示 `-`。
- [ ] 中英文 locale 中存在新增列 key。

## Definition of Done

- 针对性前端类型检查或 key 扫描通过。
- 不修改后端接口，不引入兼容性分支。
- 保持下游 fork 分支边界，任务记录包含下游工作流上下文。

## Technical Approach

复用现有 `UpstreamRelayGroupUsageHistory` 字段，在 `UpstreamRelayGroupMonitoringView.vue` 内新增展示函数；表格新增列，不变更 API DTO 和后端查询。

## Out of Scope

- 不调整历史用量接口字段。
- 不新增跨页聚合统计。
- 不改变当前 Actual Cost / Token 的展示格式。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 `.trellis/spec/frontend/type-safety.md`。
- 语义检索定位到 `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue` 的历史用量表格。
