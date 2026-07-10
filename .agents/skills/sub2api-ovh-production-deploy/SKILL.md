---
name: sub2api-ovh-production-deploy
description: "安全更新 Sub2API OVH 生产实例到 origin/custom/main 最新版本。Use when deploying this downstream Sub2API fork to the OVH host by building outside OVH, loading/pulling a sub2api-custom image tagged with the target commit, switching only the sub2api application container, validating health, or planning rollback."
---

# Sub2API OVH 生产更新部署

用于把当前下游二开仓库的 `origin/custom/main` 最新版本部署到 OVH 生产服务器。它不是迁移流程；迁移真实数据、DNS、证书或跨机器切流时使用 `sub2api-production-migration`。

## 事故硬约束

- **禁止在 OVH 生产机上执行 `docker build`、`pnpm run build`、`pnpm exec vite build`、`go build` 等构建命令。**
- OVH 生产机只允许做：加载/拉取已构建镜像、备份 override、切换 `sub2api` 应用容器、健康检查、回滚。
- 标准入口是仓库内的 `deploy/ovh-safe-deploy.sh`，从本地工作站或 CI 运行。
- 如果用户要求“直接在 OVH 上构建”，必须拒绝并说明风险：该机器资源有限，构建曾导致 OOM、SSH 不可用和健康检查超时。
- 不要把 `deploy/Dockerfile` 临时改成快速构建后放到 OVH 生产机上执行；快速构建仍可能压垮生产机。

## 安全原则

- 先只读盘点，再说明影响与回滚，最后等用户明确确认。
- 不打印 `.env`、证书私钥、数据库内容、用户数据或访问令牌。
- 默认只切换 `sub2api` 应用容器；不要重启 PostgreSQL、Redis、Caddy/Nginx，除非用户明确要求。
- 不修改 DNS、Cloudflare、防火墙或反代配置。
- 镜像标签使用 12 位提交号：`sub2api-custom:<rev>`。
- 部署前确认本地 `custom/main` 与 `origin/custom/main` 对齐，且工作树干净。

## 默认环境

- SSH 别名：`ovh`
- 生产部署目录：`/opt/sub2api-deploy`
- override 备份目录：`/opt/sub2api-backups`
- 当前应用容器名：`sub2api`
- 健康检查：`http://127.0.0.1:8080/health`
- 安全部署脚本：`deploy/ovh-safe-deploy.sh`

如果这些路径不匹配，先只读盘点，不要猜。

## 部署前只读盘点

本地：

```bash
git status --short --branch
git rev-list --left-right --count origin/custom/main...custom/main
git log --oneline -5
```

OVH：

```bash
ssh ovh 'set -eu
echo "[containers]"
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
echo "[override]"
sed -n "1,120p" /opt/sub2api-deploy/docker-compose.override.yml
echo "[health 8080]"
curl -fsS --max-time 5 http://127.0.0.1:8080/health
'
```

只读盘点允许查看 `/opt/sub2api-src` 的 git 状态，但不要在生产机源码目录执行 `git reset` 后接着构建。生产发布以外部构建镜像为准。

## 危险操作确认模板

执行部署前向用户说明：

- 目标服务器与当前镜像、目标镜像。
- 会执行：本地/CI 构建 `sub2api-custom:<rev>`、通过 SSH 加载到 OVH、备份 override、只切换 `sub2api` 容器、健康检查。
- 影响：`sub2api` 应用容器短暂重启；PostgreSQL、Redis、Caddy/Nginx 不动。
- 回滚：恢复 override 备份并 `docker compose up -d sub2api`。
- 明确说明：不会在 OVH 生产机上构建镜像。

只有用户明确回复“确认”后继续。

## 标准部署流程

1. 确认本地仓库位于干净的 `custom/main`，且已推送到 `origin/custom/main`。

```bash
git switch custom/main
git fetch origin custom/main
git status --short --branch
git rev-list --left-right --count origin/custom/main...custom/main
git rev-parse --short=12 HEAD
```

`git rev-list` 必须输出 `0  0`，且 `git status` 不能有未提交文件。

2. 使用安全脚本从本地/CI 构建并加载镜像到 OVH。

```bash
./deploy/ovh-safe-deploy.sh
```

非交互环境可显式确认：

```bash
./deploy/ovh-safe-deploy.sh --yes
```

该脚本会：

- 拒绝在疑似 OVH 生产机环境运行。
- 构建 `sub2api-custom:<rev>`，默认平台 `linux/amd64`。
- `docker save` 后通过 SSH 在 OVH 上 `docker load`。
- 备份 `/opt/sub2api-deploy/docker-compose.override.yml`。
- 只替换 override 中 `sub2api-custom:*` 的镜像行，保留已有 environment 等配置。
- 只执行 `docker compose up -d sub2api`。
- 健康检查失败时恢复备份并回滚旧应用容器。

## 只切换已存在镜像

如果镜像已由 CI 构建并加载到本机 Docker，可跳过本地构建：

```bash
./deploy/ovh-safe-deploy.sh --skip-build --yes
```

如果改为 registry 模式，仍必须保持 OVH 只 `docker pull`/切换容器，不允许 `docker build`。

## 验证

```bash
ssh ovh 'set -e
echo "[override]"
cat /opt/sub2api-deploy/docker-compose.override.yml
echo "[containers]"
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
echo "[health 8080]"
curl -fsS --max-time 5 http://127.0.0.1:8080/health
'
```

HTTP 公网 `/health` 可能经 Caddy 跳转或代理；应用健康以 `127.0.0.1:8080/health` 为准。

## 回滚

优先使用 `deploy/ovh-safe-deploy.sh` 自动生成的 override 备份路径。手动回滚时只恢复 override 并重启应用容器：

```bash
ssh ovh 'set -euo pipefail
BACKUP_FILE=<backup-file>
cd /opt/sub2api-deploy
cp "$BACKUP_FILE" docker-compose.override.yml
docker compose config --quiet
docker compose up -d sub2api
curl -fsS --max-time 5 http://127.0.0.1:8080/health
'
```

不要删除备份、旧镜像或构建临时文件，除非用户确认业务检查已完成。

## 事故后恢复检查

如果生产机因历史构建残留或 OOM 被重启，先恢复服务，不要继续部署：

```bash
ssh ovh 'set -eu
echo "[containers]"
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
echo "[override]"
sed -n "1,120p" /opt/sub2api-deploy/docker-compose.override.yml
echo "[health 8080]"
curl -fsS --max-time 5 http://127.0.0.1:8080/health
'
```

如果旧容器没有恢复，优先把 override 恢复到上一个已知健康镜像并只拉起 `sub2api`。

## 本次部署记录模式

部署完成后汇报：

- 目标提交和镜像标签。
- 容器健康状态。
- override 备份路径。
- 是否修改数据库、`.env`、DNS、Cloudflare、防火墙。
- 是否使用 `deploy/ovh-safe-deploy.sh`；如没有使用，必须说明原因。
- 明确说明未在 OVH 生产机上执行构建命令。
