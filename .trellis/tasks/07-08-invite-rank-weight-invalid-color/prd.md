# 修复邀请排名权重非法态颜色

## Goal

修复创建邀请活动排名权重表格在权重非法但数值合计恰好为 100 时，合计提示错误显示绿色成功态的问题。

## Requirements

- 合计提示只有在权重全部合法且合计等于 100 时才显示成功颜色。
- 权重非法时继续显示非法权重文案，并使用告警颜色。
- 增删行、提交 payload 和既有校验语义保持不变。

## Acceptance Criteria

- [ ] 非整数权重合计为 100 时，`rank-weight-total` 不使用绿色成功态。
- [ ] 目标测试和类型检查通过。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 frontend type-safety 指南。
