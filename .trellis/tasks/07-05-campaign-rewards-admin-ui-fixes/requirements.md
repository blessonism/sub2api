# 管理员邀请活动页面 UI 修复与优化 - Requirements Document

修复管理员邀请活动页面（admin/CampaignRewardsView.vue）中的布局问题、交互逻辑缺陷和功能完整性问题。

## Core Features

### 布局修复

**R1 页面标题区域**
页面顶部缺少标题，管理员进入后没有位置感。需在顶部添加页面标题和描述，将操作按钮整合进标题行右侧。

**R2 生命周期步骤网格**
5个步骤卡片在普通桌面（xl 但非 2xl）渲染为 2+2+1 布局，最后一个步骤单独占行。需在 xl 断点启用5列。

**R3 用户预览卡与时间线卡比例**
左侧预览卡（0.9fr）内容稀少、大量留白；右侧时间线（1.1fr）内容密集。比例应调整为约 2:3。

**R4 调整卡与版本卡高度不匹配**
两卡并排时因内容高度差异导致右卡底部大片空白。需统一卡片内布局使高度更接近，或改变并排策略。

**R5 操作按钮换行混乱**
两行 flex-wrap 按钮在中等宽度下会不规则换行，大小号按钮混排。需重新设计按钮区域布局，保证分组稳定。

**R6 计算指标与奖励结果表关联性**
计算指标4格与下方奖励结果表逻辑强关联，但无包裹容器，视觉平级。需将二者纳入同一语义区块。

### 交互逻辑修复

**R7 操作按钮状态感知禁用**
freeze / preview / finalize / payout 按钮不根据活动状态禁用，依赖后端报错。需按生命周期状态控制可用性：
- publish：仅 draft 可用（已有）
- freeze：仅 active 可用
- preview：非 draft 时可用
- finalize：frozen/active 后可用
- payout：有 final calculation 且非 paid 状态

**R8 生命周期步骤 warmup/active 共用 index**
`lifecycleStageIndex` 中 warmup 和 active 均返回 1，预热期间步骤卡显示为"进行中（active）"，与实际不符。需区分或在步骤列表中加入 warmup 节点。

### 数据展示修复

**R9 时间线过滤缺失节点**
时间线固定渲染 8 个节点，含大量 null 时间字段，显示为橙色 missing，造成视觉噪音。需过滤掉 `raw === null` 且非当前 active 的节点。

**R10 奖励结果表显示 user_id**
奖励结果表只显示 `#user_id`，而排行榜已有 username/masked_email。需在结果行显示用户名或 masked_email。

## User Stories

- 作为管理员，我希望进入活动页面时有清晰的标题和说明，快速确认自己在哪个功能模块。
- 作为管理员，我希望操作按钮在不可用时自动禁用并有 tooltip 说明原因，避免误操作后才收到报错。
- 作为管理员，我希望5个生命周期步骤在普通桌面上横向排开，一眼看到当前进度。
- 作为管理员，我希望时间线只显示已配置的节点，不被"未设置"项目干扰视线。
- 作为管理员，我希望奖励结果表中能看到用户名，而非无意义的数字 ID。

## Acceptance Criteria

- [ ] R1：页面顶部有标题行，包含活动名称/页面名称和操作按钮
- [ ] R2：xl 断点下生命周期步骤为5列（`xl:grid-cols-5`）
- [ ] R3：预览卡与时间线卡比例调整为 2:3 左右
- [ ] R4：调整卡与版本卡在 xl 并排时无明显高度空白
- [ ] R5：操作按钮在所有断点下分组稳定，不出现跨组换行
- [ ] R6：计算指标与奖励结果表有共同的视觉容器或分区标题
- [ ] R7：freeze/preview/finalize/payout 按钮按状态表禁用，disabled 时有 title tooltip
- [ ] R8：warmup 状态下生命周期步骤正确高亮 warmup 节点（或与 active 合并逻辑自洽）
- [ ] R9：时间线仅渲染有值的节点，null 节点不显示
- [ ] R10：奖励结果表的用户列显示 username 或 masked_email，回退到 user_id

## Non-functional Requirements

- 所有修改不引入新的 TypeScript 类型错误
- 所有修改在深色模式下保持正确的颜色对比
- 响应式行为在 sm / lg / xl / 2xl 四个断点下均验证
- 不修改 script 逻辑超出必要范围，优先在模板层解决布局问题
