# 技术设计

## 分支与引用边界

- `custom/main`：同步基线，本次不移动。
- `upstream/main`：fetch 后固定的官方来源 `881f3202694c6bc932446931a30c27d9675178b9`，预检和合并必须使用同一对象。
- `sync/upstream-20260917`：从 `custom/main` 创建的同步分支。
- 独立 worktree：`/private/tmp/sub2api-sync-upstream-20260917`，承载切换、合并和检查，避免污染当前工作树。

## 执行数据流

1. 记录当前分支、提交、工作树状态和远程配置。
2. 分别 fetch 两个 remote，读取 `upstream/main` OID。
3. 基于本地 refs 运行同步预检，识别冲突候选和高风险能力。
4. 从 `custom/main` 创建同步 worktree/分支，以固定 OID 合并并复核冲突。
5. 生成检查矩阵计划，执行命中的检查并记录环境限制。

## 高风险能力

预检命中且必须在合并后复核：

- `dashboard-operational-metrics`
- `announcement-email-broadcast`
- `group-time-rate-strategy`
- `user-public-group-account-binding`

中风险：`usage-latency-column`、`i18n-overlay`。

无 `uncovered_risk_paths`。普通未覆盖路径：`wire_gen.go`、`config.go`、`deploy/README.md`，合并后人工复核。

## 兼容与回滚

- 合并未提交：在同步 worktree 执行 `git merge --abort`。
- 合并已提交：只在同步分支执行 `git revert -m 1 <merge-commit>`。
- `custom/main` 后续如需接收同步结果，另行通过 PR 和完整 CI。

## 风险

- 当前主工作树有用户内容，所有分支操作必须在独立 worktree 完成。
- 文本无冲突不代表行为兼容；重点依照预检命中的路径复核路由、DTO、仓储生产者和前端动态 key。
- 预估 105 个路径交集只是上界，不等于 Git 最终文本冲突数。
