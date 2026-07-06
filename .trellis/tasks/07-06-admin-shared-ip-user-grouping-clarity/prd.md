# 管理员同 IP 多用户分组清晰度优化

## Goal

当前管理员的“同 IP 多用户”入口已经能找出命中用户和记录，但结果仍以用户摘要为主，管理员需要自己逐个对照哪些用户共享了同一个 IP。这个任务要把排查视角从“用户列表”前移到“IP 分组”，让管理员一眼看到每个可疑 IP 下有哪些用户、各自请求和消耗情况，并能继续展开证据记录。

## What I already know

- 用户反馈：目前同 IP 多用户不够清晰，管理员仍需要一个个匹配哪些是相同 IP 的用户。
- 现有任务 `06-20-usage-same-ip-account-detection` 已实现管理员使用记录页的 `shared_ip_users=true` 快捷筛选。
- 当前接口响应 `shared_ip_users_summary` 包含整体命中 IP 数、用户数、记录数，以及最多 50 个命中用户摘要。
- 当前前端在 `frontend/src/views/admin/UsageView.vue` 中先展示“命中用户”表格，每行有用户、IP 数、记录数、最近使用、Token、费用和 IP 摘要。
- 当前后端 `GetSharedIPUsersSummary` 聚合的是用户级结果，缺少“按 IP 分组 -> 组内用户”的结构。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开仓库，本任务 base branch 已设置为 `custom/main`，实现分支为 `feature/admin-shared-ip-user-groups`。

## Assumptions (temporary)

- 这次优化仍然作用在管理员使用记录页，不新增独立风控系统。
- “清晰”优先指排查路径清晰：先看共享 IP 分组，再看该 IP 下的用户，再按需展开原始使用记录。
- MVP 不做自动封禁、自动告警或跨登录日志关联。

## Requirements (evolving)

- Confirmed MVP scope: only implement IP grouping, expandable users under each IP, and keep original record detail expansion; do not add auto-blocking, marking, or export in this task.
- 管理员启用“同 IP 多用户”后，应优先看到按 `ip_address` 聚合的分组列表。
- 每个 IP 分组至少显示：IP、涉及用户数、记录数、最近使用时间、Token、实际费用。
- 每个 IP 分组应能直接看到或展开看到组内用户，组内用户至少显示：邮箱/用户 ID、记录数、最近使用时间、Token、实际费用、是否已删除。
- 分组结果必须继续复用当前筛选条件，例如时间范围、用户、API Key、上游账号、分组、模型、请求类型和计费模式。
- 使用记录明细继续作为二级证据，不应默认淹没 IP 分组视图。
- 仍需排除空 IP，并遵守管理员权限边界，普通用户接口不暴露跨账号 IP 信息。

## Acceptance Criteria (evolving)

- [x] 启用“同 IP 多用户”后，管理员无需手工比对用户行中的 IP 摘要，即可看到每个命中 IP 下关联的用户。
- [x] 每个 IP 分组只包含当前筛选条件命中的记录。
- [x] 空 IP 不出现在命中分组中。
- [x] IP 分组和组内用户均有稳定排序，优先把记录数/用户数/最近活动更高的风险项放前面。
- [x] 命中结果很多时有摘要上限与截断提示，不静默隐藏。
- [x] 管理员仍可展开原始使用记录明细追查证据。

## Definition of Done

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Technical Approach

Recommended MVP: 在现有 `shared_ip_users_summary` 中新增 IP 分组数组，例如 `ip_groups`，而不是新增独立接口。

- 后端在 `usage_log_repo.go` 复用当前筛选条件和同 IP 命中条件，新增 IP 级聚合查询。
- DTO 增加 `SharedIPGroupSummaryItem` 与组内用户项，控制每层返回上限，避免大结果一次性塞满页面。
- 前端将同 IP 面板的主表从“命中用户”改为“命中 IP 分组”，每个分组可展开组内用户。
- 原有用户摘要可移除或降级为辅助统计，避免两个主视角并列造成认知负担。

## Feasible Approaches

**Approach A: IP 分组主视图（Recommended）**

- How: `shared_ip_users_summary` 增加 `ip_groups`，前端优先显示 IP 分组，展开查看组内用户。
- Pros: 最贴合当前痛点，改动集中，复用现有入口和筛选条件。
- Cons: 后端需要新增一组聚合查询和测试，前端面板布局需要调整。

**Approach B: 保留用户主视图，增加“按 IP 排序/分组标签”**

- How: 继续返回用户列表，但按 IP 聚类排序，并在用户行展示同组标签。
- Pros: 改动较小。
- Cons: 管理员仍要在用户行之间建立关联，不能彻底解决“一个个匹配”的问题。

**Approach C: 新增独立同 IP 风险页**

- How: 新建管理员风控页，专门做 IP 分组、用户关系和处置。
- Pros: 后续扩展空间最大。
- Cons: 对当前问题偏重，导航、接口和权限面更大。

## Expansion Sweep

### Future evolution

- 后续可扩展为“风险组”：同 IP、同 API Key 行为、同上游账号异常、同设备指纹等统一聚合。
- 可保留组级处置入口的位置，例如标记观察、导出证据、跳转用户详情。

### Related scenarios

- 管理员可能需要从 IP 分组跳转到某个用户的余额/使用记录/账号详情。
- 使用记录导出后续可以考虑携带 IP 分组上下文，但本 MVP 不强制。

### Failure & edge cases

- 大量共享出口 IP 可能导致分组过大，需要每层上限和截断提示。
- NAT、公司网络、校园网等共享 IP 可能是正常行为，界面文案应避免直接定性为作弊。
- 当前筛选范围过窄时，同一个 IP 可能不再满足多用户条件，这应与现有筛选组合语义一致。

## Open Questions

- MVP 是否只做 Approach A 的 IP 分组主视图，还是同时加入导出/标记/跳转等处置入口？

## Out of Scope (explicit)

- 自动封禁、自动限流或自动扣费。
- 登录 IP、设备指纹、地理位置归因。
- 独立风控看板或告警系统。
- 对历史空 IP 记录补填。

## Technical Notes

- Prior PRD: `.trellis/tasks/06-20-usage-same-ip-account-detection/prd.md`
- Existing frontend: `frontend/src/views/admin/UsageView.vue`
- Existing frontend types: `frontend/src/api/admin/usage.ts`
- Existing backend types: `backend/internal/pkg/usagestats/usage_log_types.go`
- Existing backend repository: `backend/internal/repository/usage_log_repo.go`
- Existing frontend test: `frontend/src/views/admin/__tests__/UsageView.spec.ts`
- Downstream fork rules: `.trellis/spec/guides/downstream-fork-workflow.md`
