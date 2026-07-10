# 固化 OVH 安全部署防护

## Goal

把本次 OVH 生产机因镜像构建导致资源耗尽的事故沉淀为可执行约束：以后部署 `custom/main` 时，生产机只负责加载/拉取已构建镜像和切换应用容器，禁止在 OVH 生产机上执行 Docker build。

## Requirements

- OVH 生产部署流程必须明确禁止在 `/opt/sub2api-src` 或生产宿主机上构建镜像。
- 提供一个可执行的安全部署入口，从本地或 CI 构建 `sub2api-custom:<12位commit>`，再传输到 OVH 并只切换 `sub2api` 应用容器。
- 部署入口必须在切换前备份 `docker-compose.override.yml`，并尽量保留 override 中已有配置，仅替换 `sub2api` 镜像行。
- 部署入口必须包含回滚路径：切换失败或健康检查失败时恢复旧 override 并重新拉起 `sub2api`。
- 部署技能文档必须把“外部构建 + OVH 只加载/切换”设为标准流程，删除或降级原先在 OVH 上构建的做法。
- 文档必须记录事故教训与人工恢复检查步骤，避免后续 agent 重复执行高风险构建。

## Acceptance Criteria

- [x] `.agents/skills/sub2api-ovh-production-deploy/SKILL.md` 明确禁止在 OVH 生产机上 Docker build。
- [x] `deploy/` 下存在安全部署脚本，脚本默认从本地/CI 构建并通过 SSH 加载镜像到 OVH。
- [x] 脚本在疑似生产机环境运行时会中止，避免误把生产机当构建机。
- [x] 脚本只修改 `sub2api` 应用容器镜像，不重启 PostgreSQL、Redis、Caddy/Nginx。
- [x] 脚本包含 override 备份、健康检查和自动回滚逻辑。
- [x] 部署文档说明新流程、禁止事项和事故后重启恢复检查。

## Definition of Done

- 修改完成后运行 shell 语法检查。
- 检查文档和技能中不再把 OVH Docker build 作为标准路径。
- 不触碰生产服务器，不执行新的部署动作。

## Technical Approach

- 新增 `deploy/ovh-safe-deploy.sh`，作为本地/CI 使用的 OVH 安全部署入口。
- 更新 `deploy/README.md`，补充 OVH 下游二开生产部署章节。
- 更新 `sub2api-ovh-production-deploy` skill，把生产部署标准流程改为本地/CI 构建、SSH 加载镜像、OVH 只切应用容器。

## Decision (ADR-lite)

**Context**: OVH 生产机资源有限，多次在生产机上构建镜像会导致内存耗尽、SSH 不可用和健康检查超时。

**Decision**: 禁止生产机承担构建职责。构建移到本地或 CI，OVH 只做镜像加载/拉取、override 备份、应用容器切换和健康检查。

**Consequences**: 部署前置时间可能转移到本地/CI，但生产机资源风险显著降低；需要维护一个明确的安全部署脚本和技能文档。

## Out of Scope

- 不迁移 DNS、证书、数据库或服务器。
- 不调整生产 `.env`、Cloudflare、防火墙或反向代理。
- 不在本任务中重新执行生产部署。

## Technical Notes

- 必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md` 的下游二开分支边界。
- 事故现场观察：标准 Docker build 在前端 `vue-tsc && vite build` 阶段 OOM；快速 Vite build 仍导致生产机负载和内存耗尽。
- 当前恢复优先级：用户人工重启后，先验证旧容器健康，再考虑按新流程重新部署。
