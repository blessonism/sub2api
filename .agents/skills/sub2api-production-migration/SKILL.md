---
name: sub2api-production-migration
description: "迁移 Sub2API 生产实例与真实数据的安全工作流。Use when moving a Sub2API deployment between servers, preparing OVH or another VPS, migrating Docker Compose local-directory data from heyun or production hosts, preserving PostgreSQL/Redis/app data, configuring Nginx/OpenResty reverse proxy, switching Cloudflare DNS, validating health, or planning rollback for production cutover."
---

# Sub2API 生产迁移

用于把正在运行的 Sub2API 生产实例迁移到新服务器，并完成反代、Cloudflare 切流和回滚保护。

## 核心原则

- 先盘点，后停机；先服务器端就绪，后 DNS 切流。
- 不在聊天里打印 `.env`、证书私钥、数据库备份内容或用户数据。
- 任何停服、打包真实数据、改 DNS、改反代、防火墙变更都必须先说明影响和回滚方式，并获得明确确认。
- 源服务器在新服务器验证完成前保持冻结，不删除源数据，不清理备份。
- 对下游二开仓库部署，优先使用 `origin/custom/main` 构建本地镜像，不直接跑上游 `weishaw/sub2api:latest`。

## 只读盘点

先确认当前生产形态，不要停容器：

```bash
ssh <source> 'docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"'
ssh <source> 'find /opt /root /home /srv /var/www -maxdepth 5 \( -name "docker-compose*.yml" -o -name ".env" \) 2>/dev/null | sort'
ssh <source> 'systemctl is-active sub2api-datamanagementd 2>/dev/null || true'
```

确认 compose 标签和挂载：

```bash
ssh <source> 'docker inspect sub2api --format "working_dir={{index .Config.Labels \"com.docker.compose.project.working_dir\"}}"'
ssh <source> 'for c in sub2api sub2api-postgres sub2api-redis; do echo "[$c]"; docker inspect "$c" --format "{{range .Mounts}}{{println .Type .Source \"->\" .Destination}}{{end}}"; done'
```

典型本地目录版迁移清单：

- `.env`
- `docker-compose.yml`
- `docker-compose.override.yml`
- `data/`
- `postgres_data/`
- `redis_data/`
- 如果启用了 `datamanagementd`，再包含它的 SQLite 数据目录。

## 新服务器预热

在目标服务器先准备运行环境，但不要用空数据启动最终生产实例。

1. 确认 Docker、Compose、Git、OpenSSL 可用。
2. 克隆 `origin/custom/main` 到源码目录。
3. 构建下游二开镜像，标签建议带提交号，例如 `sub2api-custom:<commit>`。
4. 创建部署骨架目录，但不创建最终 `.env`，避免 `AUTO_SETUP=true` 初始化空库。
5. 让 compose override 指向本地二开镜像。

示例：

```bash
git clone --branch custom/main --depth 1 https://github.com/blessonism/sub2api.git /opt/sub2api-src
cd /opt/sub2api-src
REV="$(git rev-parse --short=12 HEAD)"
DOCKER_BUILDKIT=1 docker build -t "sub2api-custom:$REV" -f Dockerfile .
```

## 一致性归档

停写窗口内执行。推荐顺序：

1. 停应用容器，阻止继续写入。
2. Redis 同步落盘。
3. 停 Redis 和 PostgreSQL。
4. 打包迁移清单。
5. 计算 SHA256。

示例：

```bash
cd /opt/sub2api
STAMP="$(date +%Y%m%d-%H%M%S)"
ARCHIVE="/opt/sub2api-prod-${STAMP}.tar.gz"

docker compose stop sub2api
docker exec sub2api-redis sh -lc 'env -u REDISCLI_AUTH redis-cli save'
docker compose stop redis postgres

tar --numeric-owner --xattrs --acls -czf "$ARCHIVE" \
  .env docker-compose.yml docker-compose.override.yml data postgres_data redis_data
sha256sum "$ARCHIVE" > "$ARCHIVE.sha256"
```

如果归档失败，立即在源服务器回滚：

```bash
cd /opt/sub2api && docker compose up -d
```

## 传输与恢复

优先服务器间直传；如果目标服务器没有源服务器 SSH 私钥，用本机临时目录中转。`.sha256` 里如果记录了源服务器绝对路径，校验时取第一列哈希再比对本地文件。

恢复到目标服务器时：

1. 备份目标现有空骨架目录。
2. 解压归档到正式部署目录。
3. 重写 `docker-compose.override.yml`，确保应用镜像是目标服务器已构建的新二开镜像。
4. `chmod 600 .env`。
5. `docker compose config --quiet`。
6. 启动服务并等待健康。

示例 override：

```yaml
services:
  sub2api:
    image: sub2api-custom:<commit>
```

验证：

```bash
cd /opt/sub2api-deploy
docker compose up -d
docker compose ps
curl -sS http://127.0.0.1:8080/health
```

数据库只查计数和大小，不输出敏感数据：

```bash
docker exec sub2api-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "select current_database(), pg_size_pretty(pg_database_size(current_database()));"'
```

## 反代与 Cloudflare

Cloudflare 橙云开启时，公开 DNS 只会显示 Cloudflare IP，不能用普通 A 记录判断源站。切流前必须确认目标服务器有正常的 `80/443` 入口。

如果目标服务器只有 Sub2API 的 `8080`，需要先配置 Nginx/OpenResty：

- `80` 可以跳转到 HTTPS。
- `443` 使用站点证书，反代到 `127.0.0.1:8080` 或 `host.docker.internal:8080`。
- 保留 `Host`、`X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto`、`Upgrade` 等头。
- 复用源服务器证书时，只复制 `fullchain.pem` 和 `privkey.pem`，不要在聊天里输出私钥内容。

验证真实域名：

```bash
curl -sS https://api.example.com/health
curl -sS -I https://api.example.com/ | sed -n '1,12p'
```

注意：如果后端对 `HEAD /health` 返回 404，但 `GET /health` 返回 `{"status":"ok"}`，以 GET 健康检查为准。

## 收尾与回滚

切流成功后记录：

- 源服务器归档路径和 SHA256。
- 目标服务器归档路径、恢复前备份目录、部署目录。
- 当前镜像标签和提交号。
- 源服务器是否保持停止。

源站回滚模板：

```bash
cd /opt/sub2api && docker compose up -d
```

目标站回滚模板：

```bash
cd /opt/sub2api-deploy && docker compose down
mv /opt/sub2api-deploy /opt/sub2api-deploy-bad-$(date +%Y%m%d-%H%M%S)
mv /opt/sub2api-deploy-pre-restore-<stamp> /opt/sub2api-deploy
```

只有在用户确认业务检查完成后，才建议清理临时归档或旧目录。
