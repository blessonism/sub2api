# brainstorm: Token 自动策略页面 UI 调整

## Goal

优化管理员「Token 自动策略」页面的信息架构和操作体验，让策略列表更易扫描、编辑表单更易理解，并降低立即执行、清退、删除等高风险操作的误触概率。

## What I Already Know

- 用户认可先做「列表降噪、表单分区、规则摘要和高风险操作收纳」。
- 当前页面位于 `frontend/src/views/admin/TokenUsagePoliciesView.vue`。
- 页面已具备策略列表、新建/编辑、预览、历史、立即执行、清退、删除能力。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开仓库，任务上下文必须记录 downstream fork 工作流。

## Requirements

- 策略列表合并低频列，突出策略名称、目标分组、运行规则、执行状态和操作入口。
- 行内只保留核心操作，把编辑、历史、清退、删除等次要或高风险操作收纳到更多菜单。
- 新建/编辑表单按「基础信息、命中条件、档位规则、执行控制」分区。
- 表单内提供规则摘要，帮助管理员在保存前理解规则的实际效果。
- 档位编辑更贴近阶梯规则展示，并保留现有阈值、倍率校验。
- 预览结果保留现有统计和分组明细，不改变后端接口契约。

## Acceptance Criteria

- [ ] 管理员可以在列表中快速看懂每条策略的命中窗口、动作、频率、上次/下次执行状态。
- [ ] 单行操作不再出现 6 个并列按钮，高风险操作不与主操作平级展示。
- [ ] 创建和编辑策略时，字段分区清晰，并显示实时规则摘要。
- [ ] 现有创建、编辑、预览、历史、执行、清退、删除功能保持可用。
- [ ] 前端类型检查或构建检查通过，或明确记录无法验证的原因。

## Definition Of Done

- 代码改动聚焦前端页面与必要 i18n 文案。
- 不改动后端接口和数据库结构。
- 不触碰现有中继监控相关未提交改动。
- 遵守 downstream fork 工作流记录要求。

## Out Of Scope

- 后端策略执行逻辑调整。
- 新增 API 字段或迁移。
- 重构全站按钮、表格或弹窗设计系统。
- 执行 git commit、push 或生产操作。

## Technical Approach

在现有 Vue 单文件页面内做结构性 UI 调整，优先复用当前 `btn`、`input`、`Select`、`BaseDialog`、`EmptyState`、`Pagination` 和 `Icon` 组件。通过本地辅助函数生成列表摘要、状态说明、规则摘要与更多菜单状态，避免引入新依赖。

## Technical Notes

- 相关约束：
  - `.trellis/spec/guides/downstream-fork-workflow.md`
  - `.trellis/spec/frontend/type-safety.md`
- 初步涉及文件：
  - `frontend/src/views/admin/TokenUsagePoliciesView.vue`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`
