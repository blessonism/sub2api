-- 上游中转分组：按本地日期保存每连接器每上游分组的最终日用量汇总。

CREATE TABLE IF NOT EXISTS upstream_relay_group_usage_history (
    id BIGSERIAL PRIMARY KEY,
    usage_date DATE NOT NULL,
    connector_id BIGINT NOT NULL REFERENCES upstream_relay_connectors(id) ON DELETE CASCADE,
    upstream_group_id VARCHAR(120) NOT NULL,
    group_name VARCHAR(160) NOT NULL DEFAULT '',
    platform VARCHAR(80) NOT NULL DEFAULT '',
    actual_cost NUMERIC(20, 8) NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    checked_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_upstream_relay_group_usage_history_day_group UNIQUE (usage_date, connector_id, upstream_group_id)
);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_group_usage_history_connector_date
    ON upstream_relay_group_usage_history (connector_id, usage_date DESC);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_group_usage_history_date
    ON upstream_relay_group_usage_history (usage_date DESC);
