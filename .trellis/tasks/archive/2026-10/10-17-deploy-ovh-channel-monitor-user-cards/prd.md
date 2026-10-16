# OVH 渠道监控卡片部署

## 范围
发布本地 custom/main 固定提交 a28f537673bd3084b38d2bed7c598f63dafaa364，当前生产 sub2api-custom:10f2f9a00dca。用户已明确要求部署并同意创建任务。

## 验收
- 所有编译在本机完成，使用 git archive 的固定源码，不纳入原工作区修改。
- 备份数据库和配置；仅切换 sub2api，迁移新增可空 group_id/FK。
- 应用健康、版本正确；迁移校验和、列与 FK 正确，公开前端资源与构建一致。
- PostgreSQL/Redis/反代等其他容器不重启；.env 与基础 Compose 不变。
- 保留旧镜像和 override 备份；失败恢复 override，仅回滚应用，不覆盖新业务数据。
