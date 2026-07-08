# 修复邀请活动排名权重 Review 问题

## Goal

修复创建邀请活动排名权重表格在非法输入时的实时反馈问题，避免权重合计文案出现 `NaN`。

## What I Already Know

- Review 发现空输入或非法数字会让 `rankWeightSum` 变成 `NaN`。
- 提交校验已经能阻止非法输入，但实时合计提示会失真。
- 当前工作区包含其他未提交活动中心/抽奖活动变更，不能擅自回滚。

## Requirements

- 非有限权重输入时，合计提示展示权重非法提示，而不是计算“超出 NaN”。
- 表单提交校验继续阻止非法权重。
- 不回滚与本次修复无关的现有工作区改动。

## Acceptance Criteria

- [ ] 清空或非法权重输入时，弹窗显示 `createWeightsInvalid`。
- [ ] 默认、新增、删除权重行的既有行为不变。
- [ ] 目标测试和 typecheck 通过。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 frontend type-safety 指南。
