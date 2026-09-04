-- Migration: 234_channel_monitor_gateway_group
-- 分组级渠道监控（v2 效果 + v1 展示）：
--   1. channel_monitor_histories.error_category：探测错误归类（复用 v2 taxonomy
--      类别名，本期消费 rate_or_capacity），前端区分"限流/拥挤"与真故障。
--   2. channel_monitors.target_kind：探测目标形态标注。
--      endpoint       默认，直连外部上游（现状）。
--      gateway_group  指向本站网关入口（绑定分组的 API key），探测请求经网关
--                     完整选号 + failover，探测结果即分组级可用性。
--                     纯展示标注，探测引擎行为不变。
-- 两列均带 DEFAULT，回滚不破坏旧数据。

ALTER TABLE channel_monitor_histories
    ADD COLUMN IF NOT EXISTS error_category VARCHAR(40) NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        WHERE t.relname = 'channel_monitors'
          AND c.conname = 'channel_monitors_target_kind_check'
    ) THEN
        ALTER TABLE channel_monitors
            ADD COLUMN IF NOT EXISTS target_kind VARCHAR(20) NOT NULL DEFAULT 'endpoint';
        ALTER TABLE channel_monitors
            ADD CONSTRAINT channel_monitors_target_kind_check
            CHECK (target_kind IN ('endpoint', 'gateway_group'));
    END IF;
END $$;
