# 部署 OVH 渠道监控用户卡片

日期：2026-10-17

发布源码提交：`a28f537673bd3084b38d2bed7c598f63dafaa364`；生产镜像：`sub2api-custom:a28f537673bd`。

用户已授权部署并同意创建 Trellis 任务。本机固定 SHA 构建，SSH 加载后仅切换 sub2api。新增 246 分组关联迁移成功；健康、公开 API 与 9 项公网资源字节校验通过，其他 7 个容器及运行环境不变。

数据库/配置/应用数据备份：`/opt/sub2api-backups/predeploy-a28f537673bd-20261016T181826Z`。已验证数据库归档，保留旧镜像及 override 备份；回滚入口见归档任务 rollback-app.sh。

未推送 Git，未在 OVH 构建。采用标准安全脚本的固定提交副本，所有差异及证据归档于 `.trellis/tasks/archive/2026-10/10-17-deploy-ovh-channel-monitor-user-cards/`。独立记录会话，避免触碰原工作区已修改的 Edom index/journal。
