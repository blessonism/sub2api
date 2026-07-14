# 实施计划

1. 刷新官方标签引用并校验 `v0.1.151`、工作区和分支基线。
2. 创建 `sync/upstream-v0.1.151`，执行 `git merge --no-commit --no-ff v0.1.151`。
3. 审阅所有重叠文件、迁移编号、下游业务模块和 locale overlay，并同步官方发布后的 `VERSION=0.1.151` 回写，处理语义集成问题。
4. 运行后端相关包测试、迁移回归测试、前端类型检查及设置组件测试。
5. 记录验证结论，保留未提交合并结果，等待用户确认是否提交、合回或部署。

## 验证结论

- 官方标签 `v0.1.151` 是当前 `MERGE_HEAD`，无未解决冲突，嵌入版本为 `0.1.151`，`git diff --check` 通过。
- OpenAI 请求身份、Fast/Flex 用户规则、Grok reasoning effort、OAuth/WS 身份配对的定向后端回归通过；迁移包测试通过。
- 前端 `vue-tsc --noEmit` 通过；`SettingsView` 与 locale overlay 定向测试 27/27 通过。
- 扩大检查发现 `custom/main` 既有测试漂移：后端 repository/service/dto 存在与本次同步文件无关的断言失败；前端全量测试 1133/1136 通过，3 个失败均为 `GroupsView.columnSettings.spec.ts` 未纳入既有 `visible_rate_multiplier` 列。这些失败未由本次合并引入，未在同步分支越界修复。
