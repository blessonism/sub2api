# OVH 全平台注入修复部署验收

已部署 `sub2api-custom:10f2f9a00dca`，固定提交 `10f2f9a00dcae6b7d48bd4cd516bb09a11c7a47a`，版本 0.2.16。新应用 health=healthy、restart_count=0；公开 TLS、首页及 7 个资源字节哈希（含 GroupsView）与本次构建一致。

## 验证

- 本机 git archive 的 5608 个源文件与固定提交 Git blob 一致；前端 i18n、类型检查、生产构建及 Go linux/amd64 embed 构建通过。
- 实际运行镜像 ID `sha256:c4d1904fae842bd21927d4329b9cd0767225b443a44464d40e4948c5c7c0f882` 与本机一致，程序版本输出含 `10f2f9a00dca`。
- 347 条迁移校验和与源码、部署前一致，无新增迁移；其余 7 个容器的 ID、启动时间、环境哈希和健康状态不变。
- .env 和 Compose 原配置哈希不变，override 仅切换镜像行，应用环境按 key/value 规范化后哈希不变。
- 首页、setup 状态、公开设置 API 正常；管理员分组及 models 无认证请求仍返回 401。
- 功能测试、真实 PostgreSQL/Redis 测试、前端检查与新增 lint 检查结果见 test-summary.json。

## 构建与发布方式

延续用户对合并本地 custom/main 与部署 OVH 的明确授权，不额外 Git push。工作树已有 journal 和其他任务记录，构建使用固定提交的干净 git archive。临时安全脚本保留标准脚本的 SSH 加载、override 备份、应用切换和健康失败回滚；用固定 SHA、构建输入校验、镜像 ID/标签、原生产镜像及配置哈希替代 origin 对齐与整个工作树干净要求，并用 --no-deps --pull never 限定应用切换。差异保存在 deploy-wrapper.diff。

npm 下载超时后，复用本机已通过测试的 node_modules；其 .pnpm/lock.yaml 与目标提交 pnpm-lock.yaml 字节一致。运行时复用本机与原生产镜像 ID 完全一致的 sub2api-custom:7f4d33b06144，仅加入新程序；Dockerfile、entrypoint、resources、go.mod/go.sum 均未变更，差异见 runtime-dockerfile.diff。OVH 未执行构建命令。

## 回滚

- 旧镜像：sub2api-custom:7f4d33b06144，仍保留。
- override 备份：`/opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_7f4d33b06144-to-10f2f9a00dca-20261017-000319.yml`。
- rollback-app.sh 恢复该 override，并仅重启 sub2api；不恢复数据库，避免覆盖部署后的业务写入。
- 本机固定源码、程序及镜像构建文件保留在 `/var/folders/1s/1y5j398106j6vnny35bgwt7m0000gn/T/sub2api-ovh-all-prompts-10f2f9a00dca-aq0fx6px`，供业务验收与追溯。
