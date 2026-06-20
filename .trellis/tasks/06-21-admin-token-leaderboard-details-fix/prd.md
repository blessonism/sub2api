# fix: 管理员 Token 排行榜用户明细加载失败

## Goal

修复管理员 Token 排行榜展开用户明细时返回 500 的问题，让 API Key、分组、模型三个明细维度都能正常加载。

## What I already know

* 本仓库是 `Wei-Shaw/sub2api` 的下游二开仓库，仓库变更必须遵循 `.trellis/spec/guides/downstream-fork-workflow.md`。
* 当前修复分支为 `fix/admin-token-leaderboard-details`，目标合并基线为 `custom/main`。
* 本地日志显示 `GET /api/v1/admin/dashboard/token-leaderboard/users/2/details` 返回 500。
* 根因已验证：模型明细 SQL 使用 `GROUP BY model`，PostgreSQL 将其解析为 `usage_logs.model` 列，而不是 SELECT 别名，导致 `requested_model` 未出现在 GROUP BY 中。

## Requirements

* 管理员 Token 排行榜用户明细接口不再因为模型维度聚合 SQL 报错。
* 模型明细仍按 `requested/upstream/mapping` 模型源表达式聚合。
* 修复不改变排行榜主接口、前端展示字段或普通用户排行榜隐私边界。

## Acceptance Criteria

* [x] 后端测试覆盖 `requested_model` 存在时的用户明细模型聚合。
* [x] 本地定向后端测试通过。
* [x] 本地明细 SQL 或接口验证不再触发 PostgreSQL GROUP BY 错误。

## Technical Approach

* 将模型明细 SQL 的 `GROUP BY model` 改为按 SELECT 第一列或完整表达式分组，避免与 `usage_logs.model` 列名冲突。
* 补充仓储层测试，固定 requested/upstream/model 归一化表达式参与分组的行为。

## Out of Scope

* 不调整管理员排行榜 UI。
* 不修改生产数据库、迁移、部署配置或远程分支。
* 不提交或推送，除非用户另行要求。

## Technical Notes

* 相关后端文件：`backend/internal/repository/usage_log_repo.go`。
* 相关测试候选：`backend/internal/repository/*usage_log_repo*test.go`。
* 修复后验证：
  * `go test ./internal/repository -run 'TestUsageLogRepositoryGetAdminTokenLeaderboardUserDetails' -count=1 -timeout=60s`
  * `go test ./internal/repository -run 'TestUsageLogRepositoryGetAdminTokenLeaderboardUserDetails|TestUsageLogRepositoryGetAdminTokenLeaderboard' -count=1 -timeout=60s`
  * `go test ./internal/handler/admin -run 'TestAdminTokenLeaderboardDetailsParsesUserID|TestAdminTokenLeaderboardFiltersForwarded' -count=1 -timeout=60s`
  * `git diff --check`
* 当前运行中的 `sub2api-dev` 容器构建上下文是 `/Users/suki/code/sub2api-admin-token-usage-leaderboard`，且当前主工作区存在非本任务的 `frontend/src/components/layout/AppSidebar.vue` 未提交改动；本任务未重建运行容器，避免把无关前端改动打入镜像。
