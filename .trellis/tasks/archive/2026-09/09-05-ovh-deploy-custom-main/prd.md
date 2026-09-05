# 部署 origin/custom/main 到 OVH 生产

## Goal

将 origin/custom/main 的已发布版本安全部署到 OVH，仅切换 sub2api 容器并验证健康状态。

## Requirements

- 目标发布源必须是 `origin/custom/main` 的最新提交。
- 部署前确认工作树干净，且本地 `custom/main` 与 `origin/custom/main` 对齐。
- 仅使用 `deploy/ovh-safe-deploy.sh` 在本地构建并通过 SSH 加载镜像；禁止在 OVH 生产机上构建。
- 默认只切换 `sub2api` 应用容器，不重启 PostgreSQL、Redis、Caddy/Nginx，不修改 `.env`、DNS、防火墙或代理配置。
- 切换前备份 `/opt/sub2api-deploy/docker-compose.override.yml`，健康检查失败时恢复备份并回滚应用容器。

## Acceptance Criteria

- [x] 已将目标提交 `3e9e6b7aa733` 推送到 `origin/custom/main`，并以 12 位提交号构建镜像。
- [x] 已使用 `deploy/ovh-safe-deploy.sh` 将 `sub2api-custom:3e9e6b7aa733` 部署到 OVH；未在 OVH 生产机执行构建命令。
- [x] OVH 的 `sub2api` 容器为 healthy，`127.0.0.1:8080/health` 返回 `{"status":"ok"}`。
- [x] 已生成 override 备份：`/opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_2fc77ed1d9d7-to-3e9e6b7aa733-20260905-140957.yml`。
- [x] PostgreSQL、Redis、Caddy/Nginx 未重启；数据库、`.env`、DNS、防火墙和代理配置未修改。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
