-- 上游中转分组倍率历史流水：仅记录首次基线之后的新增、移除和最终倍率变化。

CREATE TABLE IF NOT EXISTS upstream_relay_group_rate_snapshot_changes (
    id BIGSERIAL PRIMARY KEY,
    connector_id BIGINT NOT NULL REFERENCES upstream_relay_connectors(id) ON DELETE CASCADE,
    upstream_group_id VARCHAR(120) NOT NULL,
    group_name VARCHAR(160) NOT NULL DEFAULT '',
    platform VARCHAR(80) NOT NULL DEFAULT '',
    change_type VARCHAR(32) NOT NULL,
    old_final_rate_multiplier NUMERIC(20, 8),
    new_final_rate_multiplier NUMERIC(20, 8),
    old_status VARCHAR(40) NOT NULL DEFAULT '',
    new_status VARCHAR(40) NOT NULL DEFAULT '',
    source VARCHAR(40) NOT NULL DEFAULT '',
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_upstream_relay_snapshot_change_type CHECK (change_type IN ('added', 'removed', 'rate_changed'))
);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_snapshot_changes_connector_time
    ON upstream_relay_group_rate_snapshot_changes (connector_id, changed_at DESC);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_snapshot_changes_time
    ON upstream_relay_group_rate_snapshot_changes (changed_at DESC);
