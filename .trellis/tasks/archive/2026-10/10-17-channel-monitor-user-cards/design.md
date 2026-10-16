# 技术设计

## 边界与数据流
沿用 ChannelMonitorService.ListUserView 的现有基础聚合，独立展示服务按用户可用分组补充倍率和统计。管理表单 -> admin DTO -> service -> Ent/SQL -> user view -> user DTO -> 卡片；新字段均可空，原契约兼容。

## 持久化与接口
channel_monitors 增加可空 group_id，FK groups(id) ON DELETE SET NULL；Ent 普通可空字段，与 account_id 模式一致。更新省略/null 不变，0 解绑，正数绑定；创建 null/省略无绑定。group_name 保留旧展示用途。复制保留 group_id，分组不存在/已删除则不得新增绑定。
用户列表增加 group_id、primary_checked_at、visible_rate_multiplier、cache_rate_24h、output_tps_5m。primary_checked_at 取主模型 latest，避免附加模型时间替代。独立服务仅对 GetAvailableGroups 返回的 ID 下发倍率和使用统计，倍率直接使用已解析的最终可见倍率。

## 统计
usage_logs 批量筛选 (group_id, COALESCE(NULLIF(TRIM(requested_model),''),NULLIF(TRIM(model),''),'unknown'))。24h 缓存比例沿用 V2；5m TPS 沿用 ops queryOutputTPS 的有效文本请求过滤及逐请求完整耗时 AVG。一次批量 SQL，现有 group_id/created_at 索引。按 group/model 缓存 30 秒，按 key 的在途结果合并相同及重叠的并发批次；权限/倍率不共享缓存，查询 2 秒超时，错误不缓存为零，降级不阻断列表。纯 quota 跳过统计。

## 界面与兼容
改造 MonitorCard/MonitorTimeline/MonitorCardGrid；卡片去掉外边框，使用轻柔阴影；深色主题和悬停分别调整阴影强度，骨架也不显示外边框；沿用 statusLabel、状态颜色、平台图标和窗口可用率缓存。Ping 在响应时间 title。共享时钟更新相对时间，卸载清理；保留整卡 button，避免嵌套交互元素。全部新文案进入 zh/en custom overlay。独立于规模隐藏开关显示生成速度。

## 发布与回滚
新列默认 NULL，无自动按名回填；管理员后续绑定。迁移先于新服务启动。旧代码可忽略新列，回退应用即可保留数据。
