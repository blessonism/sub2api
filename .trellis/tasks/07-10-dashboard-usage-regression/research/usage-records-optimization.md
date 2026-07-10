# 用户使用记录延迟健康列回归研究

## 结论

当前回归不是数据接口缺字段，而是一次典型的合并边界错配：同步提交 `2c503a333df1f7903a8ecd3471787391c1601c4c` 将用户使用记录列定义从 `first_token`、`duration` 改成了 `latency`，但下游 `frontend/src/views/user/UsageView.vue` 使用的是内联 `DataTable`，合并后仍只保留旧的 `cell-first_token`、`cell-duration` 槽位。`DataTable` 会按列 key 查找 `cell-latency`，因此该列标题存在而单元格为空。

最小完整恢复只需修改用户使用记录视图及其视图测试；现有延迟分档工具、双语文案和管理员用量表实现均已保留，不应重做公共契约，也不应把当前下游用户页整体替换成上游的公共 `UsageTable`。

## 研究依据

- 任务约束：`.trellis/tasks/07-10-dashboard-usage-regression/prd.md`。
- 下游同步约束：`.trellis/spec/guides/downstream-fork-workflow.md`。
- 前端类型约束：`.trellis/spec/frontend/type-safety.md`。
- 跨层检查：`.trellis/spec/guides/cross-layer-thinking-guide.md`。
- 代码定位先使用 Semble；关键查询只命中管理员表的 `cell-latency` 与 `frontend/src/utils/latencyHealth.ts`，未命中用户视图的 `cell-latency`，随后用 `rg` 和 Git 历史逐项确认。

## 原始优化提交与预期行为

原始优化来自上游提交 `1a3cc2a787dc79231012e57ff9e8ad8606afcc7a`（`refactor: 重新设计用量页布局并新增延迟健康列`，2026-07-09），而不是一个仅存在于下游的业务提交。该提交已被同步提交纳入当前历史。

它定义的用户可见行为是：

1. 将“首 Token”和“耗时”两列合并为单个 `latency` 列。
2. 单元格同时显示“首字”和“总耗时”；没有 `first_token_ms` 时首字显示 `-`，色条只按总耗时使用纯色。
3. 有首字数据时，左侧色条上端按首字延迟、下端按总耗时着色，并使用 `from-40% to-60%` 的短渐变。
4. 首字阈值为 `10s / 30s / 60s`，总耗时阈值为 `1min / 3min / 5min`；对应 `good / warn / slow / critical` 四档。
5. 时长小于一分钟显示毫秒或秒；一分钟以上显示 `Xm Ys`，一小时以上显示 `Xh Ym`。

对应实现现在仍完整存在于：

- `frontend/src/components/admin/usage/UsageTable.vue`：`cell-latency` 模板、颜色映射调用和紧凑时长格式。
- `frontend/src/utils/latencyHealth.ts`：阈值、严重度判定、文本色、纯色色条和渐变两端 class。
- `frontend/src/i18n/locales/{zh,en}/dashboard.ts`：`usage.latency`、`usage.latencyFirstToken`、`usage.latencyDuration`。
- `frontend/src/utils/__tests__/latencyHealth.spec.ts`：阈值边界测试。

## 合并后具体丢失点

同步提交 `2c503a33` 的第一父提交是下游 `c1560bf80e1e65e305962a82d0b4830cbaa74d95`，第二父提交是上游同步头 `12d811bd76572836d6df6e1fa8aa5ff91be3b12e`。

两边的页面结构不同：

- 上游优化后的用户页通过公共 `UsageTable` 渲染；延迟单元格实现位于该公共组件中，用户页本身只需把列 key 改成 `latency`。
- 下游用户页为了保留管理员代看、用量校准、图片/视频计费展示、错误请求 tab、虚拟表格等能力，使用内联 `DataTable` 并在页面内定义所有 `cell-*` 槽位。

合并结果只吸收了适用于上游架构的三行列定义变更：

- 删除 `first_token` 列声明。
- 删除 `duration` 列声明。
- 新增 `latency` 列声明。

但下游架构所需的配套内容没有迁入：

- 缺少 `#cell-latency` 模板。
- 旧的 `#cell-first_token` 和 `#cell-duration` 模板仍在，但已没有对应列，成为不可达代码。
- 缺少从 `@/utils/latencyHealth` 导入的 `firstTokenSeverity`、`durationSeverity` 及四组 class 映射。
- 用户页本地 `formatDuration` 仍把所有 `>= 1s` 的值显示为秒，例如 `300000ms` 显示成 `300.00s`，未获得 `5m 0s` / `Xh Ym` 的紧凑格式。

`git blame` 也印证了这种“半合并”状态：当前 `latency` 列声明归属于 `2c503a33`，而两个旧槽位和旧格式化函数仍来自更早的下游历史。

## 现有测试缺口

原始提交只新增了 `latencyHealth.spec.ts` 的阈值单元测试，没有给用户视图增加渲染测试。当前 `frontend/src/views/user/__tests__/UsageView.spec.ts` 的 `DataTableStub` 只转发 `cell-billing_mode`、`cell-tokens`、`cell-cost`，完全不渲染 `cell-latency`；测试消息表也仍只有旧的 `usage.firstToken`、`usage.duration`。因此“列 key 与槽位 key 不一致”不会被现有测试发现。

管理员侧不是本次丢失点：`frontend/src/components/admin/usage/UsageTable.vue` 当前仍有完整延迟健康列，`latencyHealth.spec.ts` 也仍覆盖阈值。

## 最小完整恢复范围

### `frontend/src/views/user/UsageView.vue`

1. 用单个 `#cell-latency` 替换不可达的 `#cell-first_token`、`#cell-duration`。
2. 复用管理员表的单元格语义：首字/总耗时两行、文本健康色、存在首字时的上下渐变色条、缺少首字时的总耗时纯色色条。
3. 从 `@/utils/latencyHealth` 导入现有严重度函数和 class 映射，不复制阈值或另建口径。
4. 将当前本地 `formatDuration` 对齐管理员表的紧凑格式：`<1s`、`<1min`、`Xm Ys`、`Xh Ym`。
5. 保留当前内联 `DataTable` 及页面其余结构，避免回退管理员代看、校准、图片/视频、错误请求和虚拟滚动能力。

### `frontend/src/views/user/__tests__/UsageView.spec.ts`

1. 让 `DataTableStub` 转发 `cell-latency`，并补齐三个延迟文案 key。
2. 增加视图级回归测试，使用跨阈值数据断言首字、总耗时、紧凑时长文本及对应文本色/渐变色 class。
3. 同一测试或第二个小用例覆盖 `first_token_ms = null`：首字为 `-`，色条采用总耗时纯色且不使用渐变。

无需修改：API 类型、后端接口、`latencyHealth.ts`、双语 locale、管理员 `UsageTable.vue`。这些部分当前均已满足契约。

## 验证建议

- 运行 `frontend/src/views/user/__tests__/UsageView.spec.ts`，证明列实际渲染而不只是声明。
- 保留并运行 `frontend/src/utils/__tests__/latencyHealth.spec.ts`，防止阈值漂移。
- 运行前端类型检查，确保模板导入及 `UsageLog` 的 nullable 延迟字段处理正确。

## Caveat

用户口中的“之前优化”在 Git 历史上来源于上游 `1a3cc2a78`；真正的下游特性是更复杂的用户页内联表格结构。回归发生在把上游公共组件假设合入下游结构时，而不是延迟工具或 API 数据被删除。恢复时应移植行为，不应整体恢复上游文件版本。
