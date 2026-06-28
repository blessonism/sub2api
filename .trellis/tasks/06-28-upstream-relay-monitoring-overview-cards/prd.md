# 上游倍率监控概览卡片精简

## Goal

精简管理员“上游倍率监控”页面顶部概览区，减少重复卡片，让待应用 Priority 数量直接体现在 tab 上。

## What I Already Know

- 本次只改前端展示，不改后端接口、统计口径或策略预览 tab 内部内容。
- 用户确认顶部保留三张卡：连接器状态、候选状态、最近同步。
- “待应用建议”独立卡片删除，待应用数量移动到 tab，显示为 `Priority  0` 这类纯数字徽章。
- tab 文案不出现“建议”两个字。

## Requirements

- 合并“可用连接器”和“需重新登录”为一张连接器状态卡。
- 合并“启用候选”和“探测失败”为一张候选状态卡。
- “最近同步”继续作为独立卡片。
- 删除“待应用建议 / 应用前需确认风险”概览卡。
- 将“Priority 建议”tab 改为 “Priority”，并在标题旁展示待应用数量徽章。
- 不调整策略预览 tab 里的预览统计卡和明细表。

## Acceptance Criteria

- [ ] 顶部概览只展示连接器状态、候选状态、最近同步三张卡。
- [ ] 点击连接器状态和最近同步仍进入连接器 tab；点击候选状态仍进入候选映射 tab。
- [ ] Priority tab 显示纯数字徽章，不显示“建议”字样。
- [ ] 中英文 i18n key 同步更新。
- [ ] 针对性前端测试通过。

## Out of Scope

- 不改策略预览 tab 内的候选数、Priority 数、排除数三张卡。
- 不改候选、连接器、建议生成或应用逻辑。
- 不新增依赖或抽象组件。

## Technical Notes

- 当前页面为 `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`。
- 本仓库是下游二开仓库，任务 base branch 记录为 `custom/main`，当前工作分支为 `feature/upstream-relay-policy-preview`。
