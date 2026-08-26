# 技术设计

## 边界与数据流

`usage_logs` 聚合查询 → `usagestats.AccountStats` → `service.WindowStats` → 管理员今日统计批量接口 → `AccountTodayStatsCell`。

## 后端

- 在 `usagestats.AccountStats` 和 `service.WindowStats` 增加 `input_tokens`、`cache_creation_tokens`、`cache_read_tokens`。
- 扩展 `GetAccountTodayStats` 与 `GetAccountWindowStatsBatch` 的 SQL 聚合和扫描顺序，保证批量接口一次返回每个账号的缓存拆分数据。
- `GetTodayStats`、`GetTodayStatsBatch` 通过现有转换函数透传字段，不新增接口或迁移。
- SQL 继续使用 `timezone.Today()` 作为当天起点，沿用当前应用时区。

## 前端

- 扩展 `frontend/src/types/index.ts` 的 `WindowStats`。
- 在 `AccountTodayStatsCell.vue` 现有紧凑今日统计下增加缓存命中 Token 和命中率。
- 命中率使用 `cache_read_tokens / (input_tokens + cache_creation_tokens + cache_read_tokens)`；分母为零显示 `-`，百分比保留 1 位小数。
- 在中英文账号 locale 增加字段文案。

## 测试与回滚

- 更新后端账号今日统计集成测试，断言缓存字段。
- 增加 `AccountTodayStatsCell` 组件测试覆盖命中、无数据和零分母。
- 保留现有 API/账号列表测试；如失败，优先回滚字段展示和聚合变更，不涉及数据库结构。
