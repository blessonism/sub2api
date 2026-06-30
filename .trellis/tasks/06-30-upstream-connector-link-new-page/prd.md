# 连接器链接支持新页面打开

## Goal

让上游监控连接器列表中展示的连接器地址可点击，并在新页面打开，减少管理员复制链接再访问的操作成本。

## Requirements

- 连接器 `base_url` 为合法 `http/https` 绝对 URL 时，渲染为可点击链接。
- 点击链接使用新标签页打开，并带 `rel="noopener noreferrer"`。
- 非法或非绝对 URL 保持纯文本展示，避免异常字符串变成可点击入口。
- 不改变连接器创建、编辑、同步、删除等业务逻辑。

## Acceptance Criteria

- [ ] 连接器列表名称下方的合法 `base_url` 可点击并新标签页打开。
- [ ] 非 `http/https` URL 不渲染为链接。
- [ ] 前端类型检查通过。

## Definition of Done

- 复用项目已有 URL 安全工具。
- 不修改后端接口。
- 任务记录包含下游 fork 工作流和前端 type-safety 约束。

## Technical Approach

在 `UpstreamRelayGroupMonitoringView.vue` 中复用 `sanitizeUrl`，用一个局部 helper 判断连接器 URL 是否可作为外链。模板根据 helper 返回值在 `<a>` 与纯文本之间切换。

## Out of Scope

- 不新增连接器详情页。
- 不调整 `base_url` 的存储格式和后端校验。
- 不为 URL 增加复制按钮或测速按钮。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 `.trellis/spec/frontend/type-safety.md`。
- 已确认 `frontend/src/utils/url.ts` 提供 `sanitizeUrl`，默认只接受 `http/https` 绝对 URL。
