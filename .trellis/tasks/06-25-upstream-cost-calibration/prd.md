# 上游渠道真实成本校准与优先级建议

> 废弃说明：该方向已被“上游中转站分组倍率监控与托管”替代。本任务仅保留为历史记录；代码入口、页面和运行时表由后续清理迁移移除。

## Goal

把上游渠道真实扣费校准做成系统内置能力：管理员可配置一组同分组、同模型、同协议的账号，系统用统一测试请求采样上游余额前后差，计算真实扣费并生成账号分组优先级调整建议。第一版只生成建议，管理员确认后才更新 `account_groups.priority`。

## Requirements

- 成本校准以现有 `Account` 为渠道主体，目标写入位置为 `account_groups.priority`。
- 管理员可创建、更新、删除校准任务，并配置名称、目标分组、模型、测试提示词、测试次数、余额适配器、待校准账号。
- 系统支持手动触发一次校准运行，运行结果记录每个账号的余额前后值、扣费差、请求成功状态、延迟和错误原因。
- 第一版提供可扩展 `BalanceAdapter` 抽象，并内置 `manual` 适配器用于后台录入/模拟余额，避免在未明确具体上游 API 前写死站点协议。
- 成本排名只纳入测试请求成功、余额可读、扣费差非负的账号；异常账号保留记录但不参与建议排序。
- 系统根据有效成本从低到高生成 `account_groups.priority` 调整建议；建议默认不应用。
- 管理员确认应用某次运行建议后，系统更新对应账号在目标分组的 priority，并记录原值、新值、操作者和应用时间。
- 后台页面展示任务列表、运行历史、账号结果、建议状态，并提供手动运行和确认应用入口。

## Acceptance Criteria

- [x] 管理端 API 支持校准任务 CRUD、手动运行、查看运行详情、确认应用建议。
- [x] 未确认应用前，校准运行不会修改 `account_groups.priority`。
- [x] 确认应用后，只有本次有效建议涉及的账号分组 priority 被更新。
- [x] 失败、余额不可读、扣费为负的账号不会被排入低成本建议。
- [x] 后台能展示最近运行结果、每个账号扣费和建议变更。
- [x] 后端单元/集成测试覆盖排序、过滤、未应用不写 priority、应用后写入和审计。

## Definition of Done

- Tests added/updated for backend service/repository/handler behavior where practical.
- Frontend typecheck/build relevant checks pass or known blockers are documented.
- Migration/schema changes are explicit and do not touch production data outside normal migration files.
- Downstream fork workflow is respected; no commit/push without explicit confirmation.

## Technical Approach

- 新增成本校准 service/repository/handler，参考 `token-usage-policies` 的 admin CRUD + run/history 模式。
- 新增数据库表：校准任务、任务账号、运行记录、账号运行结果、priority 建议/应用审计。
- 使用 service 层 DTO 对外暴露 snake_case JSON；repository 只负责持久化模型。
- 第一版 `BalanceAdapter` 支持 `manual`，从账号 `extra` 或任务账号配置中读取 `before_balance` / `after_balance` 测试值；后续真实上游适配器按同一接口扩展。
- 测试请求第一版通过系统内部模拟执行结果落库，不直接拿真实用户请求；真实上游请求执行可在后续 adapter 确定后接入。

## Out of Scope

- 不做全自动切流。
- 不把真实用户请求作为成本测试样本。
- 不在第一版接入未知站点的网页抓取式余额解析。
- 不跨不同余额单位、不同模型、不同协议强行比较。
- 不提供长期独立外部脚本；如需脚本，仅作为调用系统 API 的薄入口。

## Technical Notes

- 当前分支：`feature/upstream-cost-calibration`，base branch：`custom/main`。
- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 参考现有后台策略模块：`token-usage-policies`。
