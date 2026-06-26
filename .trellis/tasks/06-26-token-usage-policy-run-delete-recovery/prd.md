# fix: Token 自动策略执行落库与删除恢复

## Goal

修复生产环境 Token 自动策略在调整阈值后出现“页面显示已执行，但实际倍率未按预期清退/落库”，以及已有自动归属记录导致策略无法删除的问题。目标是让策略执行结果可追溯、可恢复，并允许管理员安全退出一条不再需要的自动策略。

## What I already know

- 生产中曾将一档阈值从 3 亿 Token 调整到 5 亿 Token，但 3 亿 Token 用户仍疑似享受倍率优惠。
- 用户反馈页面显示策略执行过，但实际没有按预期执行。
- 用户反馈无法删除该自动策略。
- 现有服务会在低于最低档位时生成 `clear` 变更，并由仓储层清理本策略产生的专属倍率或恢复旧手工倍率。
- 现有删除逻辑会在存在自动归属记录时拒绝删除策略。
- 后端规范明确：纯 `manual-skip` 接管记录不应阻止删除。

## Assumptions

- 不能直接在代码任务里操作生产数据库。
- 修复应优先通过后端逻辑提供安全清退能力，避免要求管理员手写 SQL。
- 删除策略前应保证本策略产生的倍率/授权已经被清理，不能误删管理员手工设置。
- 执行历史必须能区分真正变更、清除、跳过与失败，避免“看起来成功但无实际落库”的误判。

## Requirements

- 删除策略时，纯手动接管/跳过记录不能阻止删除。
- 对仍存在自动写入倍率或策略授予分组的策略，删除前必须有安全清退路径。
- 清退必须只影响本策略产生的配置：恢复 `previous_rate_multiplier` 或清空本策略倍率，保留 `rpm_override` 和非本策略授予的分组。
- 立即执行与定时执行的成功状态必须建立在 apply、执行明细、run summary 同一事务成功的基础上。
- 针对无法删除的生产策略，代码应支持管理员通过一次显式操作先清退，再删除。

## Acceptance Criteria

- [x] 已自动清退或仅包含手动跳过记录的策略可以删除。
- [x] 仍有本策略实际接管配置的策略直接删除时继续被拒绝，防止误删。
- [x] 提供一个后端接口或等价能力，用于清退某策略当前仍接管的用户配置。
- [x] 清退操作写入执行历史，管理员能看到 `clear` 变更。
- [x] 针对删除阻塞和清退路径补充后端测试。

## Definition of Done

- 相关后端单元测试通过。
- 不执行生产数据库写操作。
- 若新增 API，前端类型和页面入口保持一致。
- 明确给出生产修复后的操作顺序和回滚建议。

## Technical Notes

- 任务基线：`custom/main`
- 工作分支：`fix/token-usage-policy-run-delete-recovery`
- 关键文件：
  - `backend/internal/service/token_usage_policy.go`
  - `backend/internal/repository/token_usage_policy_repo.go`
  - `backend/internal/handler/admin/token_usage_policy_handler.go`
  - `frontend/src/api/admin/tokenUsagePolicies.ts`
  - `frontend/src/views/admin/TokenUsagePoliciesView.vue`
- 已读约束：
  - `.trellis/spec/guides/downstream-fork-workflow.md`
  - `.trellis/spec/backend/quality-guidelines.md`
