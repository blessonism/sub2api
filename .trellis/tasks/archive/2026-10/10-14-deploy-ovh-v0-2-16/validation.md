# OVH v0.2.16 部署验收

状态：**通过**。2026-10-14 18:48（Asia/Shanghai）完成应用切换，随后完成服务器及公网验证。

## 发布结果

- SSH：`ovh`；站点：`https://api.edom37.online`。
- 目标源码：本地 `custom/main`，`8fec104efb1fac129ee0665da6bae866baa80de1`。
- 版本：`0.2.16`，包含已验收的二开同步与旧失败修复。
- 新镜像：`sub2api-custom:8fec104efb1f`，`linux/amd64`。
- 镜像 ID：`sha256:6a108ce3562339c9038afcb85990374444d835db5f95b671e0718d393c0e009c`，本地、加载镜像和运行容器一致。
- 二进制：Go 1.27.2，`CGO_ENABLED=0`，嵌入前端，`BuildType=release`；版本输出匹配 `0.2.16 / 8fec104efb1f`。
- 旧镜像：`sub2api-custom:ac033ba1ce82`（v0.2.6），保留用于回滚。

## 验证证据

| 检查 | 结果 |
| --- | --- |
| Docker health / restart count | healthy / 0 |
| 本机 `/health`、首页、`/setup/status`、公开设置 | HTTP 200；setup 已完成；公开设置 code=0 |
| 未鉴权管理员接口与模型接口 | HTTP 401 |
| 数据库迁移 | 原有 336 个校验一致；新增 10 个成功；全部 346 个与目标校验值一致 |
| 启动日志 | fatal、migration failure、ERROR level 均为 0 |
| 工作站公网 HTTPS | TLS 校验通过；health 和公开设置正常；首页引用本次资源 |
| 公网 6 个核心 JS/CSS | 文件内容及 SHA-256 与本地发布文件完全一致 |
| OVH 上的 HTTPS origin 与公网探针 | 首页 HTTP 200，origin health 正常 |
| PostgreSQL / Redis / Caddy / Databasus / sub4api 及其依赖 | 7 个容器 ID、镜像、启动时间和重启次数均未变化，均 healthy |
| 生产 `.env`、主 Compose、应用环境变量 | 摘要一致；override 仅镜像标签改变，权限仍为 0600 |

详细结果：`server-validation.json`、`server-after.json`、`public-validation.json`、`migration-preflight.json`、`build-evidence.json`。本次没有重复执行已通过的完整开发套件；未使用真实用户凭据执行登录、计费或支付请求。

## 生产备份与回滚

override 备份：

```text
/opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_ac033ba1ce82-to-8fec104efb1f-20261014-184804.yml
```

迁移前数据库与应用数据备份目录：

```text
/opt/sub2api-backups/predeploy-8fec104efb1f-20261014T103609Z
```

目录权限 0700，备份文件 0600；数据库 dump 710,012,021 字节，已用 `pg_restore -f /dev/null` 完整解析验证；应用数据归档 15,916 字节，排除运行日志，校验文件为 `SHA256SUMS`。真实备份始终留在 OVH，没有下载或提交到仓库。

应用回滚命令（本次未执行）：

```bash
ssh ovh 'set -euo pipefail
cd /opt/sub2api-deploy
cp /opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_ac033ba1ce82-to-8fec104efb1f-20261014-184804.yml docker-compose.override.yml
docker compose config --quiet
docker compose up -d --no-deps --pull never sub2api
curl -fsS --max-time 5 http://127.0.0.1:8080/health
'
```

迁移为 forward-only，恢复旧镜像不会恢复数据库。第 239 个迁移转换旧定价 JSON，若需要完整数据回退，应评估迁移后的写入并安排维护窗口，不能自动覆盖生产库。

## 部署入口与边界

使用 `deploy/ovh-safe-deploy.sh` 的临时固定提交副本；差异保存在 `deploy-wrapper.diff`。用户要求部署尚未推送的本地版本，因此用固定 `custom/main` SHA 和 tracked tree 校验替代远端对齐检查，并从 `git archive` 干净快照构建。没有改写远端 refs，也没有推送 Git。

标准 Docker builder 下载 pnpm 时网络失败，原生交叉编译缺少的 Linux 依赖首次下载也失败；依赖通过保留 Go checksum 校验的镜像源补齐。前端改用本地已验收且 lockfile 未变的依赖，后端使用原生 Go 1.27.2 交叉编译，运行时阶段按标准 Dockerfile 封装，差异保存在 `runtime-dockerfile.diff`。运行时基础镜像固定到已解析的 Alpine 3.20 digest。

OVH 未执行任何 Docker、Node 或 Go 构建命令。只备份、加载镜像、切换应用容器和检查；启动执行了上述 10 个数据库迁移。没有修改 `.env`、DNS、Cloudflare、防火墙和反代配置。应用启动早期首次健康连接被重置，随后自动检查成功；工作站公网首页首次 TLS 握手失败，有界重试及独立 origin/公网探针全部通过。

## 收尾与规范复核

本次是发布已有实现，没有新增持久的 API、脚本或配置契约，规范不作修改；临时偏离和网络故障留在本任务记录，不改变默认部署工作流。原有 7 项未跟踪工作保留。生产备份和新旧镜像保留；本地临时源码、程序和发布脚本在证据写入后清理。部署记录与 journal 仅本地保存，不新增代码提交或推送。
