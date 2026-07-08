# 修复上游分组倍率监控历史用量分页

## Goal

修复管理员「上游分组倍率监控」历史用量列表无法继续翻页、导致连接器历史记录看不完整的问题，确保历史用量分页状态与后端总数一致。

## What I already know

- 用户反馈历史用量没有真正显示完整，且“下一页”不可点击。
- 前端历史用量每页固定请求 50 条记录。
- 后端历史用量接口按筛选条件分页返回 `items`、`total`、`page`、`page_size`、`pages` 和 `summary`。
- 历史用量记录只包含已写入 `upstream_relay_group_usage_history` 的连接器/分组，不等同于连接器清单。

## Assumptions

- 这是现有页面的分页状态或响应契约问题，不需要改变历史用量采集模型。
- 用户希望能看完当前筛选范围内的所有历史记录；是否一次性加载全部以实际实现风险为准。

## Requirements

- 当后端返回总数大于当前页数量时，前端必须能进入后续页。
- 当前筛选范围的总数、页码、每页数量和展示数量应保持一致，不误导用户。
- 修复不得破坏连接器列表、快照变化、推荐历史等同页其他分页功能。

## Acceptance Criteria

- [x] 历史用量在 `total > page_size` 时“下一页”可点击，并能加载下一页。
- [x] 历史用量页数计算能兼容后端 `pages` 缺失或异常时的兜底。
- [x] “仅异常”过滤不会让用户误以为已经加载了全量异常记录。
- [x] 相关前端测试或针对性检查覆盖分页状态。

## Definition of Done

- Tests added/updated where appropriate.
- Targeted lint/type/test checks pass or known failures are documented.
- Downstream fork workflow constraints are respected.

## Out of Scope

- 不调整历史用量采集周期、数据表结构或连接器刷新策略。
- 不改造为无限滚动，除非定位后发现当前分页无法满足需求。

## Technical Notes

- `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
- `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`
- `backend/internal/handler/admin/upstream_relay_group_monitoring_handler.go`
- `backend/internal/repository/upstream_relay_group_monitoring_repo.go`
- `.trellis/spec/guides/downstream-fork-workflow.md`

## Implementation Notes

- 前端历史用量分页页数改为基于 `pages` 与 `ceil(total / page_size)` 共同兜底，避免后端页数字段异常时把列表锁在第一页。
- 筛选待应用状态改为比较草稿条件与已应用条件，避免等值变更把翻页按钮误禁用。
- 新增视图测试覆盖 `total > page_size` 但 `pages` 异常时仍能进入下一页。
