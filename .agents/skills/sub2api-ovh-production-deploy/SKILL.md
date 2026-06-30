---
name: sub2api-ovh-production-deploy
description: "安全更新 Sub2API OVH 生产实例到 origin/custom/main 最新版本。Use when deploying this downstream Sub2API fork to the OVH host, updating /opt/sub2api-src and /opt/sub2api-deploy, building a sub2api-custom image tagged with the target commit, switching only the sub2api application container, handling pnpm/Docker build compatibility, validating health, or planning rollback."
---

# Sub2API OVH 生产更新部署

用于把当前下游二开仓库的 `origin/custom/main` 最新版本部署到 OVH 生产服务器。它不是迁移流程；迁移真实数据、DNS、证书或跨机器切流时使用 `sub2api-production-migration`。

## 安全原则

- 先只读盘点，再说明影响与回滚，最后等用户明确确认。
- 不打印 `.env`、证书私钥、数据库内容、用户数据或访问令牌。
- 默认只重建 `sub2api` 应用容器；不要重启 PostgreSQL、Redis、Nginx，除非用户明确要求。
- 不修改 DNS、Cloudflare、防火墙或反代配置。
- 镜像标签使用 12 位提交号：`sub2api-custom:<rev>`。
- 部署前先确认本地 `custom/main` 与 `origin/custom/main` 对齐；如果本机 HTTPS push 缺凭据，可在 OVH 上 `git fetch origin custom/main` 验证远端是否已有目标提交。

## 默认环境

- SSH 别名：`ovh`
- 生产源码目录：`/opt/sub2api-src`
- 生产部署目录：`/opt/sub2api-deploy`
- override 备份目录：`/opt/sub2api-backups`
- 当前应用容器名：`sub2api`
- 健康检查：`http://127.0.0.1:8080/health`

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
echo "[src]"
cd /opt/sub2api-src
git status --short --branch
git log --oneline -5
echo "[override]"
sed -n "1,80p" /opt/sub2api-deploy/docker-compose.override.yml
'
```

如果需要确认远端 GitHub 是否已有目标提交：

```bash
ssh ovh 'cd /opt/sub2api-src && git fetch origin custom/main && git rev-parse --short=12 origin/custom/main'
```

## 危险操作确认模板

执行部署前向用户说明：

- 目标服务器与当前版本、目标版本。
- 会执行：拉取 `origin/custom/main`、构建镜像、备份 override、切换 `sub2api` 容器、健康检查。
- 影响：`sub2api` 应用容器短暂重启；PostgreSQL、Redis、Nginx 不动。
- 回滚：把 override 改回旧镜像并 `docker compose up -d sub2api`。

只有用户明确回复“确认”后继续。

## 标准部署流程

1. 更新 OVH 源码到远端最新：

```bash
ssh ovh 'set -euo pipefail
cd /opt/sub2api-src
git checkout custom/main
git fetch origin custom/main
git reset --hard origin/custom/main
git rev-parse --short=12 HEAD
'
```

2. 构建目标镜像：

```bash
ssh ovh 'set -euo pipefail
cd /opt/sub2api-src
REV=$(git rev-parse --short=12 HEAD)
DOCKER_BUILDKIT=1 docker build -t "sub2api-custom:$REV" -f deploy/Dockerfile .
'
```

3. 如果 Dockerfile 使用 `pnpm@latest` 导致 frozen lockfile 失败，使用临时 Dockerfile 固定 `pnpm@9.15.9`：

```bash
ssh ovh 'set -euo pipefail
cd /opt/sub2api-src
REV=$(git rev-parse --short=12 HEAD)
TMP_DOCKERFILE="/tmp/sub2api-Dockerfile-$REV"
sed "s/corepack prepare pnpm@latest --activate/corepack prepare pnpm@9.15.9 --activate/" \
  deploy/Dockerfile > "$TMP_DOCKERFILE"
DOCKER_BUILDKIT=1 docker build -t "sub2api-custom:$REV" -f "$TMP_DOCKERFILE" .
'
```

4. 如果 OVH 资源不足，完整 `vue-tsc -b && vite build` 可能长时间压高负载。确认本地/CI 已做过类型检查后，可用临时 Dockerfile 只生成前端产物：

```bash
ssh ovh 'set -euo pipefail
cd /opt/sub2api-src
REV=$(git rev-parse --short=12 HEAD)
TMP_DOCKERFILE="/tmp/sub2api-Dockerfile-fast-$REV"
sed \
  -e "s/corepack prepare pnpm@latest --activate/corepack prepare pnpm@9.15.9 --activate/" \
  -e "s/RUN pnpm run build/RUN pnpm exec vite build/" \
  deploy/Dockerfile > "$TMP_DOCKERFILE"
DOCKER_BUILDKIT=1 docker build -t "sub2api-custom:$REV" -f "$TMP_DOCKERFILE" .
'
```

使用快速构建时，在最终汇报里必须说明跳过了容器内重复 `vue-tsc`，以及本地/CI 类型检查是否已完成。

## 切换应用容器

构建镜像成功后才切换：

```bash
ssh ovh 'set -euo pipefail
REV=$(cd /opt/sub2api-src && git rev-parse --short=12 HEAD)
OLD_IMAGE=$(docker inspect sub2api --format "{{.Config.Image}}")
OLD_TAG=${OLD_IMAGE#sub2api-custom:}
DEPLOY_DIR=/opt/sub2api-deploy
BACKUP_DIR=/opt/sub2api-backups
STAMP=$(date +%Y%m%d-%H%M%S)
mkdir -p "$BACKUP_DIR"
cd "$DEPLOY_DIR"
cp docker-compose.override.yml "$BACKUP_DIR/docker-compose.override.predeploy-$OLD_TAG-to-$REV-$STAMP.yml"
printf "services:\n  sub2api:\n    image: sub2api-custom:%s\n" "$REV" > docker-compose.override.yml
docker compose config --quiet
docker compose up -d sub2api
docker compose ps
'
```

## 验证

```bash
ssh ovh 'set -e
echo "[src]"
cd /opt/sub2api-src
git rev-parse --short=12 HEAD
git status --short --branch
echo "[override]"
cat /opt/sub2api-deploy/docker-compose.override.yml
echo "[containers]"
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
echo "[health 8080]"
curl -fsS --max-time 5 http://127.0.0.1:8080/health
'
```

HTTP Nginx 上的 `/health` 可能返回 `301 Moved Permanently` 跳 HTTPS；应用健康以 `127.0.0.1:8080/health` 为准。

## 回滚

把 override 改回旧镜像并只重启应用容器：

```bash
ssh ovh 'set -euo pipefail
OLD_REV=<old-rev>
cd /opt/sub2api-deploy
printf "services:\n  sub2api:\n    image: sub2api-custom:%s\n" "$OLD_REV" > docker-compose.override.yml
docker compose config --quiet
docker compose up -d sub2api
curl -fsS --max-time 5 http://127.0.0.1:8080/health
'
```

不要删除备份、旧镜像或构建临时文件，除非用户确认业务检查已完成。

## 本次部署记录模式

部署完成后汇报：

- 源码版本和镜像标签。
- 容器健康状态。
- override 备份路径。
- 是否修改数据库、`.env`、DNS、Cloudflare、防火墙。
- 构建中使用的兼容措施，例如固定 `pnpm@9.15.9` 或快速 Vite 构建。
