# 实施计划

1. 校验官方签名标签、分支基线和差异范围，创建版本化同步分支。
2. 合入 `v0.1.152`，按跨层字段集合解决两个冲突并审阅所有自动合并重叠文件。
3. 核对版本、迁移、下游独有模块和 locale overlay，执行后端与前端范围测试。
4. 提交同步分支，合回 `custom/main`，复核提交拓扑后推送 `origin/custom/main`。

## Validation

- 后端：受影响包定向测试、迁移测试、`go test` 范围检查（单项最长 60 秒）。
- 前端：`vue-tsc --noEmit`、上游新增组件测试及下游关键组件回归。
- Git：`git diff --check`、冲突标记扫描、标签祖先关系和下游文件存在性检查。

## Validation Result

- 后端全包编译通过；受影响的 service、repository、handler、routes、migrations、
  ent/migrate 与 apicompat 定向测试通过。
- 前端类型检查通过；上游新增功能、设置/分组和下游抽奖回归共 96 项通过。
- 前端全量测试 1158/1160 通过；两个 TokenLeaderboardView 失败属于同步前既有
  设置保存测试漂移，本次上游增量未修改该模块。
- repository 全量测试仍有同步前既有失败，集中在会话扫描参数、用量日志 SQL mock、
  排行榜 SQL 和中继策略 mock；本次变更相关的 repository 定向测试通过。
- Ent 重新生成修正了合并后字段索引；鉴权快照版本升至 v16，避免两侧分别使用
  v15 时接受缺少 Web Search 或下游倍率字段的旧缓存。
