# 监控反馈闭环设计

## 边界与数据流

1. 后端 `fetchUpstreamGroupUsageForDate` 以候选绑定为最小执行单元：无 Key 或单 Key 请求失败只记录结构化 issue，其他绑定继续刷新。
2. `RefreshConnectorMetrics` 返回 `usage_detail.issues`、`updated_groups` 与 `missing_groups`。前端只依据稳定 code 生成业务说明，原始 message 进入技术详情。
3. 页面通过一个通用 `OperationResultPanel` 展示四类操作。业务层负责组装“动作 / 结果 / 影响 / 下一步 / 技术详情”，组件只负责一致布局和状态语义。
4. 修复入口把来源连接器记录在候选弹窗上下文中。候选保存成功后先重载候选，再刷新来源连接器 metrics，并合并结果卡。
5. Runner 定格失败沿用现有 `last_error` 契约，但后端把日期、连接器标识和失败原因完整编码；前端对可识别错误本地化并保留折叠详情。

## 状态归属

- 全局刷新反馈属于页面级 `monitoringRefreshResult`，可关闭；切换 Tab 时清理。
- 批量探测反馈属于候选 Tab；批量同步反馈属于触发时的 Tab，不再固定渲染在自动监控 Tab。
- Priority 应用反馈属于应用弹窗，失败不写页面级 `error`。
- 字段校验属于候选弹窗，提交后端前完成；后端失败保留为弹窗级技术详情。

## 兼容与风险

- `usage_detail.issue` 继续保留为首个问题的兼容字段，新增/使用 `issues` 作为完整问题集合。
- 原始字符串解析仅用于兼容旧响应，稳定路径使用 issue code。
- 不修改数据库结构、不执行生产操作、不改变默认刷新是否探测/生成/应用建议的边界。
