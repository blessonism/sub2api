# 管理员邀请活动页面 UI 修复与优化 - Task List

## Implementation Tasks

- [x] 1. **布局：页面标题区域**
    - [x] 1.1. 将顶部 justify-end 操作栏改为标题+操作并排的 header 行
        - *Goal*: 页面有明确标题，操作按钮与标题同行
        - *Details*: flex justify-between 布局，左侧标题+描述，右侧刷新+创建按钮
        - *Requirements*: R1

- [x] 2. **布局：生命周期步骤网格**
    - [x] 2.1. 将 `sm:grid-cols-2 2xl:grid-cols-5` 改为 `sm:grid-cols-2 xl:grid-cols-5`
        - *Goal*: xl 断点（1280px+）下5步骤横向排开
        - *Details*: 仅改 class，一行改动
        - *Requirements*: R2

- [x] 3. **布局：预览卡与时间线卡比例**
    - [x] 3.1. 将 `xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]` 调整为 `xl:grid-cols-[minmax(280px,2fr)_minmax(0,3fr)]`
        - *Goal*: 左侧预览卡更紧凑，右侧时间线更宽
        - *Requirements*: R3

- [x] 4. **布局：操作按钮区域重构**
    - [x] 4.1. 将两行 flex-wrap 按钮改为按语义分组的稳定布局
        - *Goal*: 生命周期操作（publish/freeze/preview/finalize/payout）与辅助操作（copy/delete）分组清晰，换行时不混乱
        - *Details*: 用 `gap-x-1 gap-y-2` 替代双行方案，或用分隔线区分危险操作，去掉 btn-sm 大小不一致问题
        - *Requirements*: R5

- [x] 5. **布局：计算指标与奖励结果表关联**
    - [x] 5.1. 用 `<template v-if="calculation">` 包裹计算指标4格和奖励结果表，并加分区标题
        - *Goal*: 二者有共同的视觉区块，逻辑关联清晰
        - *Requirements*: R6

- [x] 6. **逻辑：操作按钮状态感知禁用**
    - [x] 6.1. 为 freeze / preview / finalize / payout 添加 :disabled 绑定和 :title tooltip
        - *Goal*: 按活动状态自动禁用不可用的操作按钮
        - *Details*:
          - freeze: `status !== 'active'`
          - preview: `status === 'draft'`
          - finalize: `!['active','frozen','auditing','publicizing'].includes(status)`
          - payout: `!hasFinalCalculation || status === 'paid'`
        - *Requirements*: R7

- [x] 7. **逻辑：lifecycleStageIndex warmup 修正**
    - [x] 7.1. 在 lifecycleSteps 数组中插入 warmup 步骤，并修正 lifecycleStageIndex 映射
        - *Goal*: warmup 状态下高亮正确步骤节点
        - *Details*: 在 draft/active 之间插入 warmup 步骤，index 重新映射
        - *Requirements*: R8

- [x] 8. **数据展示：时间线过滤缺失节点**
    - [x] 8.1. 在 timelineItems computed 末尾 filter 掉 `!item.raw && !item.active` 的节点
        - *Goal*: 时间线只显示已配置的节点，消除橙色 missing 噪音
        - *Requirements*: R9

- [x] 9. **数据展示：奖励结果表用户标识**
    - [x] 9.1. 将结果行的 `#{{ result.user_id }}` 替换为 `result.username || result.masked_email || '#' + result.user_id`
        - *Goal*: 管理员能直接认出用户
        - *Details*: 需确认 CampaignRewardResult 类型是否有 username/masked_email 字段，若无需在 script 中 join leaderboard 数据
        - *Requirements*: R10

## Task Dependencies

- Task 1-5 为纯模板层布局改动，互相独立可并行
- Task 6 依赖对活动状态枚举的确认，独立
- Task 7 修改 lifecycleSteps computed，与 Task 6 无依赖
- Task 8、9 为模板层改动，独立
- 建议顺序：2 → 6 → 7 → 8 → 9 → 3 → 4 → 1 → 5（先改逻辑，再改布局）
