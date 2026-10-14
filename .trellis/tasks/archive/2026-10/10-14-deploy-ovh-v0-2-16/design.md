# 部署设计

## 固定发布源

当前生产镜像为 `sub2api-custom:ac033ba1ce82`（v0.2.6），本地目标为 `8fec104efb1fac129ee0665da6bae866baa80de1`。目标包含已通过完整测试的同步提交 `8983b05ff58c71a73beb405f9034158b8f7e9039`。不再次执行完整开发套件。

用户要求不推送且部署刚更新的本地版本，覆盖部署技能默认的 `origin/custom/main` 对齐要求。用 `git archive` 导出固定提交的干净构建目录，排除本地原有未跟踪文件和部署任务记录；不伪造或改写远端引用。

## 标准脚本的必要偏离

`deploy/ovh-safe-deploy.sh` 强制远端对齐、工作树完全干净，因此本次从该入口生成临时发布副本：把远端对齐检查替换为固定本地提交及 tracked tree 校验；继续复用外部构建、镜像加载、override 备份、健康检查和失败回滚。生产 compose 切换使用 `--no-deps --pull never`，确保只重建应用且使用刚加载的镜像。加载后校验镜像 ID；切换前校验生产旧镜像未被其他部署更改。临时改动不写回标准脚本，具体差异保存在 `deploy-wrapper.diff`。

标准本地 Dockerfile 的 builder 阶段下载 pnpm 时出现网络失败，因此改为在同一干净源码快照中使用本地 pnpm 10.33.2/Node 25.7.0 构建前端，并用模块要求的 Go 1.27.2 原生交叉编译 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`。现有 node_modules 对应未变更的 package/lockfile，未重新解析依赖。后端仍包含 `-tags embed` 和正式版本/提交/build-type 元数据。

运行时 Dockerfile 从标准文件最终阶段导出，仅将 builder 的 COPY 改成复制本地生成的二进制和目标提交资源；保留 Alpine 3.20 的已解析 digest、runtime 依赖、非 root 用户、entrypoint、healthcheck、数据目录和端口。运行时构建仍在本地完成，不修改正式 Dockerfile。

## 数据兼容与回滚

v0.2.6 到 v0.2.16 增加 10 个 SQL 迁移：新增字段/表/索引、平台约束调整，以及旧 reasoning pricing JSON 到新 map 的转换。没有删除业务表或列。启动迁移前确认生产已应用迁移校验值与目标一致，并建立生产机内的逻辑数据库备份，备份内容不离开服务器。

应用回滚恢复 override 并仅重建应用。数据库迁移为 forward-only，应用回滚不等于数据库回退；定价 JSON 转换存在旧代码读取差异，因此发生需要完整数据回退的故障时停止并评估写入影响，不自动覆盖迁移后的数据库。备份使用单进程 `pg_dump -Fc -Z1`，备份文件权限 0600，保存于生产机 0700 子目录；`pg_restore -f /dev/null` 完整解析通过。应用数据归档排除持续变化的 runtime logs，保留配置和初始化标记。

## 验证边界

固定镜像 ID、二进制版本、容器 health 和启动日志；探针只读且无需凭据。对比依赖容器 ID/启动时间和配置摘要。真实计费请求、用户登录和支付不在无人值守探针范围内。
