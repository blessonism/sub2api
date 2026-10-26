# 执行计划

- [x] 读取部署技能、Trellis 和下游 fork 规则；只读检查服务器与版本。
- [x] 固定 SHA、创建干净源码快照，检查迁移校验与备份条件。
- [x] 本地构建 amd64 镜像，验证版本、提交、镜像 ID。
- [x] 在生产机创建数据库逻辑备份并验证归档可读性。
- [x] 从标准入口生成仅必要偏离的临时副本，语法检查并保留具体差异证据。
- [x] SSH 加载镜像，备份 override，仅切换应用容器，失败自动回滚。
- [x] 验证启动迁移、Docker health、页面/公开 API、配置和其他容器不变。
- [x] 写入验收记录，归档任务，清理不用的本地临时源码/脚本，保留旧镜像和生产备份。

## 验证

- `git diff --quiet` / `git diff --cached --quiet`，固定发布 SHA 与 `custom/main` 相同。
- `bash -n` 临时部署脚本；镜像 `docker image inspect`；`docker run --rm <image> -version`。
- 生产 `curl -fsS http://127.0.0.1:8080/health`，首页和公开 API HTTP 状态。
- `docker inspect` 的镜像 ID、健康状态和未切换容器快照。
- 仅查询 `schema_migrations` 元数据并和目标 SQL 的 SHA-256 校验值对比。

## 回滚点

镜像切换前失败则保持当前应用；切换后健康检查失败则恢复 override 并 `docker compose up -d --no-deps sub2api`。保留数据库备份，但不自动恢复数据库。
