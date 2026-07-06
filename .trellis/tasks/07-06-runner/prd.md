# 修复上游倍率监控 Runner 状态与开关回滚

## Goal

修复上游倍率监控页 Phase 1/4 review 中发现的状态契约与交互一致性问题，确保自动调度器状态准确暴露，顶部自动调度开关失败可回滚，并且不会误提交策略页未保存草稿。

## What I already know

- 当前分支已有上游倍率监控页 5 阶段重设计的未提交实现。
- Review 发现 runner status 缺少 last_error，非 leader 实例可能被标记为 failed。
- 顶部 Auto 开关当前复用完整策略保存函数，失败不会回滚，还会提交策略页未保存草稿。
- 项目要求修改仓库内容前遵守下游 fork workflow，并在实现/检查上下文中纳入该指南。

## Requirements

- 后端 runner status 必须包含每个 job 的 last_error，并在失败时记录错误原因、成功时清空。
- 非 leader lock skip 必须是中性状态，不得污染 last_succeeded / last_error。
- 顶部状态条必须区分调度器事实状态、已保存策略、策略页草稿。
- 顶部 Auto 开关必须以已保存策略为基线，只提交目标 auto 字段变化。
- 顶部 Auto 开关提交失败后 UI 必须回滚，不保留乐观值。
- 策略页未保存草稿不得因为顶部开关被顺手提交或覆盖。

## Acceptance Criteria

- [ ] Runner status API 返回 last_error 字段并与失败任务状态一致。
- [ ] 非 leader skip 不再显示为 Last Failed。
- [ ] 顶部开关失败会回滚。
- [ ] 顶部开关 payload 不夹带策略页未保存草稿。
- [ ] 相关后端与前端回归测试通过。

## Definition of Done

- Tests added/updated for backend runner status and frontend toggle behavior.
- Targeted frontend typecheck/test checks pass or any blocker is documented.
- Scope remains limited to runner status / monitoring view / related API types and tests.

## Out of Scope

- 不处理当前 diff 中混入的 DataTable、Sidebar、GPT Intelligence、Campaign Rewards 等无关改动。
- 不新增 runner-status 拼写兼容路由，除非后续确认外部已依赖拼写错误路径。

## Technical Notes

- 适用 spec: .trellis/spec/backend/quality-guidelines.md, .trellis/spec/frontend/type-safety.md
- 必读 workflow: .trellis/spec/guides/downstream-fork-workflow.md
