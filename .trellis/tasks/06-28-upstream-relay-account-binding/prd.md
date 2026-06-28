# 上游倍率监控账号维度绑定

## Goal

将上游分组倍率监控的候选绑定从“上游分组 -> 本站分组”改为“上游分组 -> 项目账号”。推荐策略排序和应用读取并更新 `accounts.priority`，不再依赖 `account_groups.priority`，也不要求账号属于某个本站分组。

## Requirements

- 候选绑定目标是项目账号：`connector_id + account_id + upstream_group_id`。
- 候选输入、候选列表、推荐建议和排除结果不再携带 `target_group_id` / `target_group_name`。
- 候选创建和更新只校验连接器、账号、上游 Group ID、探测模型和协议。
- 候选当前优先级读取 `accounts.priority`。
- 推荐应用更新 `accounts.priority`，并用建议生成时记录的旧值做 stale conflict 校验。
- 候选倍率和今日用量继续来自 `connector_id + upstream_group_id` 的上游快照。
- 不再使用 `usage_logs.group_id = target_group_id` 聚合上游分组本地用量。
- 管理端 UI 删除“目标分组”选择和目标分组列，展示“上游分组 -> 项目账号”。

## Acceptance Criteria

- [ ] 新迁移移除上游中继候选和建议表里的本站分组绑定语义，并建立账号维度唯一约束。
- [ ] 后端 API 类型、仓储 SQL、推荐生成和应用逻辑全部切到账号级 priority。
- [ ] 前端 API 类型、候选表单、列表、预览和应用弹窗不再引用目标本站分组。
- [ ] 后端 repository/service 测试覆盖账号级绑定、账号级 priority 应用和 stale conflict。
- [ ] 前端测试覆盖提交 payload 不含 `target_group_id`，页面不再展示目标分组列。
- [ ] 定向 `go test` 和 `pnpm --dir frontend typecheck` 通过。

## Definition of Done

- 代码、迁移、测试和 i18n 文案保持一致。
- 下游 fork 分支规则已遵守。
- 不提交、不推送、不执行生产数据库变更。

## Technical Approach

- 新增 `172_upstream_relay_account_binding.sql`，用增量迁移调整历史表结构和索引。
- 后端删除候选 DTO / 输入 / 建议 / 排除中的 target group 字段，仓储查询改为 join `accounts` 并读取 `accounts.priority`。
- 推荐建议继续持久化 `old_priority` / `new_priority`，但语义改为账号优先级。
- 前端移除 groups API 依赖在本页候选表单中的使用，账号下拉展示当前账号 priority。

## Out of Scope

- 不重构账号管理页或本站分组管理页。
- 不改变上游连接器认证、全量同步、探测协议和倍率解析逻辑。
- 不处理生产库实际迁移执行。

## Technical Notes

- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 相关规范：`.trellis/spec/backend/quality-guidelines.md`、`.trellis/spec/frontend/type-safety.md`。
- 当前分支：`feature/upstream-relay-policy-preview`。
