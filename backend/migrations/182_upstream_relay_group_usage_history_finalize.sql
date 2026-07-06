ALTER TABLE upstream_relay_group_usage_history
    ADD COLUMN IF NOT EXISTS finalized_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_upstream_relay_group_usage_history_pending_finalize
    ON upstream_relay_group_usage_history (connector_id, usage_date)
    WHERE finalized_at IS NULL;
