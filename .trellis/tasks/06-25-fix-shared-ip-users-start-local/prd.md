# fix: 同 IP 命中用户截断与本地启动回退

## Goal

修复 code review 发现的两个问题：管理员同 IP 多用户面板不能静默隐藏第 51 个之后的命中用户；本地 Docker 启动脚本不能在 compose 已经执行失败后继续恢复旧容器，避免误以为正在验证当前代码。

## What I already know

- 当前分支是 `feature/admin-token-balance-calibration`，目标合入基线应为 `custom/main`。
- `backend/internal/repository/usage_log_repo.go` 的同 IP 用户聚合写死 `LIMIT 50`。
- `frontend/src/views/admin/UsageView.vue` 展示 `user_count` 和 `users`，但没有截断提示。
- `deploy/start-local.sh` 对 `start_with_compose` 的任何失败都会尝试 `start_existing_containers`。

## Assumptions

- 同 IP 命中用户摘要仍应有上限，避免大数据量筛查时返回过大 payload。
- 这次修复应优先显式暴露截断状态，而不是引入完整分页接口。
- 本地启动脚本只有在缺少 `deploy/.env`、没有 shell `POSTGRES_PASSWORD` 且不是重建时，才应走旧容器恢复。

## Requirements

- 后端同 IP 摘要响应明确返回用户摘要展示上限、是否截断、隐藏用户数。
- 前端在用户摘要被截断时展示清晰提示，管理员能知道仍有未展示命中用户。
- 本地启动脚本区分“compose 未尝试”和“compose 执行失败”，后者直接报错。
- 保持下游 fork 边界，不引入生产密钥或环境数据。

## Acceptance Criteria

- [ ] 当命中用户数大于展示上限时，API 返回 `users_truncated=true` 和正确隐藏数量。
- [ ] 管理员使用记录页面显示截断提示，不再把前 N 个用户伪装成完整列表。
- [ ] `deploy/start-local.sh` 仅在缺 env 且未尝试 compose 的启动场景恢复旧容器。
- [ ] 后端仓储/处理器测试覆盖新增同 IP 摘要字段。
- [ ] 前端视图测试覆盖截断提示。

## Definition of Done

- 针对性 Go 测试通过。
- 针对性前端 Vitest 通过。
- Shell 脚本语法检查通过。
- Trellis implement/check context 已登记下游 fork 规范。

## Out of Scope

- 不实现同 IP 用户摘要分页。
- 不调整同 IP 判定 SQL 口径。
- 不启动或修改本地 Docker 容器。

## Technical Notes

- 读取规范：`.trellis/spec/guides/downstream-fork-workflow.md`、跨层/复用思考指南、后端质量规范、前端类型规范。
- 关键文件：`backend/internal/pkg/usagestats/usage_log_types.go`、`backend/internal/repository/usage_log_repo.go`、`frontend/src/api/admin/usage.ts`、`frontend/src/views/admin/UsageView.vue`、`deploy/start-local.sh`。
