# OVH 部署验收记录

已部署 `sub2api-custom:a28f537673bd`，固定源码提交 `a28f537673bd3084b38d2bed7c598f63dafaa364`，版本 0.2.16。

## 已通过

- git archive 的 5642 个文件逐一与 Git blob 校验一致；原工作区未提交内容未进入构建。
- 本机生产前端构建（含 i18n 3 项测试和类型检查）通过；Go 1.27.2 静态交叉编译 linux/amd64、嵌入前端通过。功能定向测试见前一任务 `../10-17-channel-monitor-user-cards/verification.md`。
- 镜像中的版本、提交、平台正确；运行时环境、entrypoint、healthcheck 等配置与原生产镜像一致，仅增加新程序层。
- sub2api 容器 healthy，restart_count=0；其他 7 个容器的 ID、启动时间和环境哈希均保持一致。
- 348 条数据库迁移与源码校验和全部一致，唯一新增迁移为 246_channel_monitor_group_id.sql。group_id 为可空 bigint，外键引用 groups(id)，ON DELETE SET NULL。
- 本机与生产 /health 返回 200；首页、setup/status、公开配置 API 正常。用户/管理监控接口未认证请求均为 401。
- 从 OVH 经公网域名/TLS 校验首页及 9 个资源字节，包含 V1 指标卡片 JS、无边框/柔和阴影 CSS 和管理端关联分组产物，全部与本次构建一致。
- .env、基础 docker-compose.yml 和应用环境变量未变；override 规范化后哈希一致，仅镜像行改变。

## 构建与部署方式

采用 sub2api-ovh-production-deploy 技能。所有构建均在本机完成，OVH 仅接收镜像和切换应用，没有执行构建命令。

本地 custom/main 尚未推送，原工作树包含此前未提交的日志/任务。用户要求部署刚合并版本，因此沿用既有固定 SHA 发布方案，不附带 Git push。标准 deploy/ovh-safe-deploy.sh 的临时副本以固定源码/镜像 ID、原生产镜像 ID、配置哈希和已验证备份门禁替代 origin 对齐要求；保留 SSH 加载、override 备份、应用切换、健康失败回滚，并增加 --no-deps --pull never。差异见 deploy-wrapper.diff。

标准 Dockerfile、entrypoint、resources、依赖清单在当前生产提交 10f2f9a00dca 与目标提交间均无变更。本机已有与生产 ID 完全一致的运行时镜像，因此只替换新编译程序；差异见 runtime-dockerfile.diff。

## 备份与回滚

- 数据库/配置/应用数据备份：`/opt/sub2api-backups/predeploy-a28f537673bd-20261016T181826Z`。数据库归档 714718605 bytes，pg_restore 完整解析通过；目录 0700，文件 0600。
- override 备份：`/opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_10f2f9a00dca-to-a28f537673bd-20261017-022420.yml`。
- 保留旧镜像 `sub2api-custom:10f2f9a00dca`；rollback-app.sh 可恢复 override 并仅回滚应用。新增可空 group_id 列与旧版兼容，回滚不恢复数据库，避免覆盖后续业务写入。
- 本机构建资料保留于 `/var/folders/1s/1y5j398106j6vnny35bgwt7m0000gn/T/sub2api-ovh-a28f537673bd-mukwza4p`，供业务验收与复核。

## 业务说明

存量监控 group_id 保持 NULL，需要管理员补选关联分组后才显示倍率、缓存率和 TPS。未自动按名称回填，原探测仍可展示。未执行真实收费模型请求。没有修改 DNS、Cloudflare、反代、防火墙或 .env。
