-- 上游中转分组：记录普通用户使用记录接口同步到的远端今日用量快照。

ALTER TABLE upstream_relay_group_rate_snapshots
    ADD COLUMN IF NOT EXISTS today_actual_cost NUMERIC(20, 8),
    ADD COLUMN IF NOT EXISTS today_total_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS today_usage_checked_at TIMESTAMPTZ;
