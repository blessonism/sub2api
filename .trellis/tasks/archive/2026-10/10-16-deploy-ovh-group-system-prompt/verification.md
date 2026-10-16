# OVH 部署验收记录

已部署 `sub2api-custom:7f4d33b06144`，固定源码提交 `7f4d33b061444a8c93778da289c0abf366bbcdf4`，版本 0.2.16。

## 已通过

- 本机干净 git archive：5,569 个源文件与固定提交一致；前端 i18n、类型检查、生产构建通过；Go 1.27.2 交叉编译 linux/amd64 通过。
- 新增真实 repository 集成测试 2 个通过，覆盖 PostgreSQL/Redis 持久化、认证投影/缓存命中与失效、复制、数据库默认值和 durable 失效触发器。补足功能交付时 Docker 未启动的验证缺口。
- 镜像程序版本/提交正确，运行时环境、entrypoint、healthcheck 继承原生产镜像，配置一致。
- OVH 仅切换 sub2api，Docker health=healthy、restart_count=0；其余 7 个容器的 ID、启动时间和环境快照一致。
- 347 条数据库迁移与源码校验和全部一致，唯一新增迁移为 245_group_system_prompt_injection.sql。新列 JSONB、NOT NULL；所有既有分组值均为默认关闭；缓存失效触发器包含新字段。
- 首页、/setup/status、公开配置 API 正常；管理员分组与 models 无认证请求均 401。
- 通过 OVH 对公网域名 https://api.edom37.online 校验 TLS、首页及 7 个资源字节哈希（含 GroupsView 新配置界面），均与本次构建一致。
- .env、docker-compose.yml 内容未变；override 整份对比只改变镜像行。运行时环境按 key/value 规范化，与未改动的 Compose 定义一致。

## 构建及部署偏离

本地 custom/main 尚未推送，且原工作树含此前未提交的 journal/任务记录。用户明确要求部署刚合入的本地版本，因此没有附带 Git push。构建来自固定 SHA 的 git archive，原工作树修改未纳入。临时副本保留标准安全脚本的加载/备份/健康回滚，并使用固定提交、输入校验、镜像 ID、旧生产镜像 ID、配置哈希、备份存在门禁替代 origin 对齐要求；增加 --no-deps --pull never。具体差异见 deploy-wrapper.diff。

Docker Hub manifest 请求超时，复用本机与当前生产 ID 完全一致的运行时镜像，仅添加本次新编译程序。resources、entrypoint 和标准 Dockerfile 在两发布提交间无变更；运行时差异见 runtime-dockerfile.diff。所有构建均在本机完成，OVH 未执行构建。

## 访问和验证边界

本机首页返回 403，响应明确为 Caddy“中国大陆不开放服务”的地区策略；OVH 访问同一公网域名返回 200，TLS 和资源验证通过。没有修改地区限制、DNS、反代、防火墙或 .env，也没有执行真实收费模型请求或使用管理员凭据。

首次只读验证脚本的 JSON 字符串转义和 setup 路径写错，修正后重新执行通过。初始环境快照按数组顺序计算哈希，不能用于判断 key/value 是否变化；最终按实际有效 Compose 定义与运行环境规范化比对，不记录环境变量值。

## 备份与回滚

- 数据库/配置/应用数据备份目录：`/opt/sub2api-backups/predeploy-7f4d33b06144-20261016T135139Z`；数据库归档 714612702 bytes，pg_restore 完整解析通过，目录 0700/数据库 0600。
- override 备份：`/opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_eb5bff47e499-to-7f4d33b06144-20261016-215640.yml`，权限 0600。
- 保留旧镜像 sub2api-custom:eb5bff47e499；回滚只恢复 override 并仅重启应用。不自动恢复数据库，避免覆盖后续业务写入。
- 本机构建和源码暂存于 `/var/folders/1s/1y5j398106j6vnny35bgwt7m0000gn/T/sub2api-ovh-7f4d33b06144-9qqh_rzg`，按部署技能保留至业务验收。
- 部署后任务归档脚本自动提交记录；该提交仅包含本次部署档案。生产仍运行固定提交 7f4d33b06144 的镜像，未推送 Git。

归档脚本曾自动暂存此前未跟踪的 10-14 部署档案；收尾已从本次本地提交中排除，文件内容和未跟踪状态恢复原样。
