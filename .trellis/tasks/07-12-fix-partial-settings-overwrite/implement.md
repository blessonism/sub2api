# 实施计划

1. 盘点 `UpdateSettingsRequest`、handler/service 写入逻辑和前端所有调用方，选择稳定的全量请求门禁字段。
2. 增加排行榜专用 service、handler、route 与 DTO；确保 repository 只收到两个设置键。
3. 收紧通用前端 API 类型并迁移排行榜页面调用。
4. 添加后端 handler/service 和前端 API 回归测试。
5. 运行目标 Go 测试、前端测试与类型检查，复核差异不包含生产配置或无关改动。
