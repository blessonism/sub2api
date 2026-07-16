# 实现计划

1. 扩展后端 Key 选项与解析器，读取 `group_id`，并为缺失分组的 Key 增加明确 issue 常量。
2. 修改候选用量刷新：按当前 Key 分组聚合，修正刷新详情中的分组集合，保留多 Key 缓存与部分成功行为。
3. 扩展前端 API 类型和候选表单回填逻辑，选择 Key 时自动同步当前分组，保留手动回退。
4. 添加/更新后端解析、Key 列表、动态换组聚合测试及前端表单测试。
5. 增加候选当前分组 DTO 装饰逻辑并复用于列表、推荐；补充换组后的快照和推荐测试。
6. 将历史日期与当日动态分组路径分开，并覆盖历史归属与重复 Key 去重测试。

验证命令：

- `cd backend && go test ./internal/service -run 'TestUpstreamRelay' -count=1`
- `cd frontend && pnpm vitest run src/views/admin/__tests__/UpstreamRelayGroupMonitoringView.spec.ts`
- `cd frontend && pnpm typecheck`

风险点：`backend/internal/service/upstream_relay_group_monitoring.go` 的刷新函数被同步、历史用量和监控 Runner 共用；修改签名时需同步所有调用点。
