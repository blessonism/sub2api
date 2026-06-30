# 修复上游连接器凭据弹窗活跃监听误关闭

## Goal

修复管理员新建上游连接器时，点击“账号密码”模式或输入账号密码会导致弹窗退出的问题。修复应覆盖真实弹窗事件链，并避免全局前台活跃监听干扰凭据输入体验。

## What I Already Know

- 既有提交 `bb4efcded` 已经禁用了连接器凭据弹窗的 Escape 和外部点击关闭，并隔离了部分本地事件。
- 后续前台活跃时间上报引入了全局 `pointerdown` / `touchstart` 监听，凭据表单内部交互会触发这条全局链路。
- 当前问题表现不是 `BaseDialog` 自身的外部点击关闭，而是凭据输入区域事件被全局监听捕获后间接造成弹窗状态被刷新。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开仓库，修复基线为 `custom/main`。

## Requirements

- 连接器凭据弹窗中的账号密码模式切换不应关闭弹窗。
- 账号、密码输入框的鼠标、指针、触摸事件不应触发前台活跃上报链路。
- 全局前台活跃监听应支持显式忽略区域，避免未来类似敏感弹窗重复补局部 hack。
- 回归测试应覆盖 stub 弹窗、真实 `BaseDialog` 弹窗、触摸事件链和活跃监听忽略区域。

## Acceptance Criteria

- [ ] 新建上游连接器后点击“账号密码”，弹窗仍保持打开，并显示邮箱与密码输入框。
- [ ] 在邮箱、密码输入框中输入内容，弹窗不关闭，且不会提交创建请求。
- [ ] 标记为忽略区域的 DOM 内触发 `pointerdown` 不会调用前台活跃上报 API。
- [ ] 相关前端单元测试通过。

## Definition of Done

- 仅修改前端状态监听、连接器弹窗和对应测试。
- 不改动生产配置、数据库迁移或部署脚本。
- 遵守下游 fork 工作流，避免混入无关工作区改动。

## Out of Scope

- 不重新设计 `BaseDialog` 的通用关闭模型。
- 不调整前台活跃上报的后端接口或数据模型。
- 不处理无关的 Makefile、部署 skill 或其他已存在工作区改动。

## Technical Notes

- 关键文件：
  - `frontend/src/stores/auth.ts`
  - `frontend/src/stores/__tests__/auth.spec.ts`
  - `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`
  - `frontend/src/views/admin/__tests__/UpstreamRelayGroupMonitoringView.spec.ts`
- 必读约束：
  - `.trellis/spec/guides/downstream-fork-workflow.md`
