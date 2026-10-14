# 实现设计

公开旧 current.json / efficiency.json 已停留在旧日期，修读取上限不足以得到原站现行成绩。改为读取固定 CodexRadar 公共域名上的 /data/radar-bench-binding.json，从 model_efforts 发现实际可查询配置，再限并发读取公开 /api/radar-bench-score summary。不使用目录中的任意 transport_origin 发起请求，避免第三方字段成为任意 URL 输入。

HTTP 单请求 8 秒，整次采集最多 20 秒、并发 3、单响应 512 KiB 且读取 limit+1 明确判断超限，目录上限 256 个配置。目录中的 model/effort 去重，数据校验与 binding 的 benchmark/catalog/task digest/score version/max/task count 一致。保留源站 0 分；score=null 的无成绩配置不进入展示。读取失败只影响该配置；部分快照缓存 1 分钟，完整快照沿用一小时。全部读取失败报 unavailable，不回退旧 IQ。

DTO 沿用 latest/recent_days/comparisons 外形，增加可选 benchmark、单成绩 score_max/coverage/required_tasks/graded_runs_count，status 支持 provisional/complete。metadata 增加 requested_series/failed_series。最新成绩日期只来自合法的公开 UTC 日桶历史，未知时留空。覆盖不是通过题数，统计成本/Token 不与日期评分混合。

前端解析新增字段，保留旧 DTO 兼容；RadarBench 使用独立标题、评分口径和判分日文案，展示各系列覆盖与状态，部分失败提示。历史只使用新评分体系，不拼接旧曲线。已有 Prompt 管理保持原行为。

生产只需更新应用镜像，没有数据库迁移。提交及部署仍按任务聊天确认执行，回滚目标为当前健康镜像 sub2api-custom:8fec104efb1f；不推送 Git。
