# 部署方案

本地源树含此前未提交的日志，远端 custom/main 落后本地。按用户要求部署刚合并版本，沿用此前固定 SHA 的 archive 构建方案，不执行 Git push。

原始安全脚本要求远端分支对齐，临时副本以固定 SHA、源码 blob 校验、镜像 revision/ID、原生产镜像 ID、配置哈希、已验证数据库/配置备份门禁替代。保留标准脚本的 SSH 加载、override 备份、仅应用切换与健康失败回滚，增加 --no-deps --pull never。

本机构建前端和 linux/amd64 静态 Go 程序。当前生产运行时镜像在本机已存在且 ID 一致；Dockerfile、entrypoint、resources、依赖清单与当前生产源码均无变更，因此复用该运行时并仅替换新程序，避免拉取基础镜像及重复完整构建。每项差异记录留档。

新增 246_channel_monitor_group_id.sql 为可空外键列，旧程序可兼容。应用回滚保留新增列，不自动恢复数据库。
