# 上游倍率监控 UI 决策台调整

## Goal

将管理员“上游倍率监控”页面从资源 CRUD 页面调整为运维决策台。首屏应优先呈现运行概览和候选映射主表，低频配置动作收进弹层，Priority 应用前展示风险摘要。

## What I Already Know

- 本次只改前端 UI，不新增后端接口，不改变现有 API wire shape。
- 当前主要页面是 `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`。
- 现有 API 类型位于 `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`，本次不修改请求/响应结构。
- 当前分支是 `feature/group-visible-rate-multiplier`，任务 base branch 为 `custom/main`。

## Requirements

- 顶部加入标题、最近刷新/同步信息、刷新和生成建议操作。
- 新增运行概览卡：可用连接器、需重新登录、启用候选、探测失败、待应用建议、最近同步时间。
- 默认主区域改为候选映射，强化倍率来源、健康状态、Priority 建议差异。
- 连接器和候选表单从常驻卡片改为弹层/抽屉。
- 连接器列表新增查看快照入口，快照在弹层中展示。
- Priority 应用确认弹窗增加风险摘要和 run 创建时间。

## Acceptance Criteria

- [ ] 页面加载后首屏能看到概览、候选主表和主要操作。
- [ ] 新建/编辑连接器、新建/编辑候选在弹层中完成，保存失败时输入不丢。
- [ ] 候选探测只让当前行进入 loading，不锁死整页。
- [ ] 连接器同步后可从连接器行查看快照。
- [ ] 生成建议后，候选表能体现命中的 Priority 建议变化。
- [ ] 应用建议弹窗显示风险摘要和明细，确认后刷新 run 状态。
- [ ] 前端类型检查和构建通过。

## Out of Scope

- 不做图表、趋势曲线、独立路由或可配置 dashboard。
- 不新增后端统计接口。
- 不修改现有 API 请求/响应类型。

## Technical Notes

- 概览统计基于当前已加载数据，文案避免暗示绝对全量。
- 如果没有现成抽屉组件，使用固定定位弹层实现，不引入新 UI 依赖。
- 删除连接器和候选继续使用确认弹窗，但文案需要说明影响范围。
