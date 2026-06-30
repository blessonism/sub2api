# brainstorm: 显示上游账号倍率连接器数据

## Goal

上游账号倍率配置相关的上游连接器数据需要继续展示，即使当前规则下不推荐或不参与某些筛选，也要给管理员配置时提供足够参照，避免因为数据缺失导致难以判断倍率配置。

## What I already know

- 用户反馈：上游账号倍率的上游连接器“那些数据还是显示吧”，不显示会导致配置困难。
- 当前仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，业务改动以 `custom/main` 为基线。
- 当前工作区已有多项未提交改动，本任务需要保持改动范围窄，避免混入无关文件。
- 定位结果：连接器展开区原本只展示已配置候选映射；若连接器已经同步到上游分组倍率快照，但还没有绑定候选账号，展开区会显示“暂无候选分组”，导致配置时看不到上游账号倍率数据。

## Assumptions (temporary)

- “那些数据”指上游账号倍率/优先级/监控界面中与上游连接器相关的候选、监控或倍率参照数据。
- 目标不是改变倍率计算结果，而是调整展示/过滤策略，让配置所需数据可见。

## Open Questions

- 无阻塞问题；先从现有代码行为推导具体隐藏点。

## Requirements (evolving)

- 上游账号倍率配置相关界面应展示上游连接器参照数据。
- 若连接器不推荐或不可自动应用，应通过状态/原因表达，而不是直接隐藏。
- 保持后端返回、前端展示和测试语义一致。
- 连接器展开区应在“无候选映射但有同步快照”时展示快照分组、倍率、今日用量，并提供创建候选入口。

## Acceptance Criteria (evolving)

- [x] 管理员能在相关界面看到上游账号倍率配置所需的上游连接器数据。
- [x] 不推荐或受限的连接器仍可显示，并保留可解释原因。
- [x] 相关单元测试或视图测试覆盖显示规则。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green for touched scope
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 不调整生产数据和数据库结构。
- 不改变上游账号倍率的核心计费/计算模型，除非定位发现展示缺失来自错误的数据契约。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`，确认下游 fork 分支边界。
- 实现策略：前端连接器展开区合并候选行与快照行；同一 `connector_id + upstream_group_id` 已有候选时优先显示候选，否则显示快照行，避免改变后端倍率计算和推荐策略。
