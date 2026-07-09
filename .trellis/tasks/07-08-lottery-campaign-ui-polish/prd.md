# Lottery Campaign UI Polish

## Goal

在已完成抽奖活动 million 单位改造的基础上，继续打磨管理侧与用户侧 UI，让运营配置门槛时更确定，用户看到进度时更有行动指向，同时保持后端 API 的 raw token 契约不变。

## Requirements

- 管理表单使用 i18n 文案表达 million 单位，不在模板里拼接裸 `(M)`。
- 门槛和阶梯步长输入框右侧固定展示 `M` 单位，并实时显示等价 raw tokens。
- 每日一次模式隐藏阶梯步长和最多机会数字段；阶梯多次模式展示并校验。
- 管理表单按基础信息、参与门槛、开奖设置、奖项配置分组，并展示实时规则预览。
- 活动列表展示可扫标签，例如门槛、开奖节奏、奖项结构、活动中心展示。
- 详情指标中门槛显示 `x.xxM` 主值，并提供 raw tokens 副文本。
- 硬删除从主操作区移到更明确的危险操作区域。
- 用户侧进度区展示“还差 x.xxM 达标”或“已达标”提示。
- 多档奖项配置呈现更接近表格的紧凑布局。
- 补充边界测试：million 输入转换、`daily_once` 提交 `entry_step_tokens: 0`、小数换算。

## Acceptance Criteria

- [x] 输入 `1.25M` 会提交 `threshold_tokens: 1250000`。
- [x] 输入 `0.005M` 按当前转换规则提交 `5000` raw tokens。
- [x] `daily_once` 模式提交时 `entry_step_tokens` 为 `0`，且无关字段不出现在表单 UI。
- [x] 用户侧在未达标时显示剩余 `M` 数，在达标后显示达标提示。
- [x] `admin.lotteryCampaigns.*` 新增/使用的 key 在中英文 locale 均存在。

## Definition of Done

- 相关前端测试覆盖新增 UI 与转换行为。
- `pnpm --dir frontend test:run` 的相关用例通过。
- `pnpm --dir frontend typecheck` 通过。

## Technical Approach

继续复用 `frontend/src/utils/usagePricing.ts` 的 `TOKENS_PER_MILLION` 和 `formatTokenMillions`。组件内部保持 `threshold_token_millions` / `entry_step_token_millions` 作为表单态，API payload 仍输出 `threshold_tokens` / `entry_step_tokens` raw tokens。

## Out of Scope

- 不改后端字段、路由、抽奖资格计算。
- 不引入新的 UI 组件库。
- 不重做活动中心整体视觉系统。

## Technical Notes

- 已读取下游 fork 工作流；当前分支 `feature/user-activity-center` 上已有大量未提交活动中心相关改动，本任务只追加 scoped UI polish，不回滚既有改动。
- 设计方向：这是后台运营表单和用户侧活动进度，不套用营销页式视觉；优先信息密度、可扫性、输入确定性和低误触风险。

## Verification

- `pnpm --dir frontend test:run src/components/admin/activities/__tests__/LotteryCampaignAdminPanel.spec.ts src/views/user/__tests__/CampaignRewardsView.spec.ts src/views/user/__tests__/LeaderboardView.spec.ts`
- `pnpm --dir frontend typecheck`
