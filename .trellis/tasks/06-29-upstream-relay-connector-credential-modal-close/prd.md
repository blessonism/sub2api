# fix: 上游倍率监控新增连接器账号密码选择不退出弹窗

## Goal

修复管理员上游倍率监控页面中，新建连接器弹窗切换到账号密码登录后，选择浏览器保存的账号或密码会意外退出弹窗表单的问题，避免管理员录入凭据时丢失未保存内容。

## Requirements

- 新建连接器弹窗内切换“账号密码”登录模式后，选择/填写邮箱和密码不应关闭弹窗或重置表单。
- 弹窗仍应支持明确点击取消、右上角关闭按钮或提交成功后关闭。
- 修复范围聚焦前端交互，不改后端连接器保存契约。

## Acceptance Criteria

- [x] 点击“新建连接器”后弹窗打开。
- [x] 点击“账号密码”后弹窗保持打开，并显示邮箱、密码字段。
- [x] 在邮箱和密码字段输入值后弹窗保持打开。
- [x] 对连接器认证模式切换触发 pointer/mouse/click 事件时不应冒泡成弹窗关闭。
- [x] 对连接器弹窗触发 Escape 场景时不再丢弃表单。
- [x] 相关前端单测覆盖上述回归路径。

## Definition of Done

- 前端代码通过针对性测试验证。
- 只修改与本交互相关的 Vue 组件、测试和 Trellis 任务记录。
- 下游 fork 工作流上下文已写入 implement/check。

## Technical Notes

- 入口组件：`frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
- 通用弹窗组件：`frontend/src/components/common/BaseDialog.vue`
- 首轮仅禁用 Escape 仍未解决，说明实际问题更接近认证模式切换事件在真实浏览器/弹窗层中冒泡到关闭链路；需要在模式切换按钮上阻断 pointer/mouse/click 传播。
