# brainstorm: GPT 智力检验内置题删除按钮可见

## Goal

修复管理员编辑 GPT 智力检验内置题时看不到删除按钮的问题，让删除动作在题目详情区域也清晰可见。

## Requirements

- 管理员打开任意题（包括内置题）时都能看到删除按钮。
- 删除按钮应在题目标题区域可见，不只依赖弹窗 footer。
- 删除后仍通过保存按钮发布全局题库。

## Acceptance Criteria

- [x] 内置题详情区域展示删除按钮。
- [x] 聚焦前端测试覆盖删除按钮可见。

## Technical Notes

- 下游 fork 约束：`.trellis/spec/guides/downstream-fork-workflow.md`
- 2026-06-30：删除按钮移动到题目详情标题区域，并增加 `data-test="delete-intelligence-template"` 覆盖内置题删除入口。
