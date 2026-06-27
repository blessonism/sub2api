-- 上游中转监控 Phase 2：探测窗口统计、usage delta 兜底与结构化建议原因。

ALTER TABLE upstream_relay_recommendation_suggestions
    ADD COLUMN IF NOT EXISTS reason_code VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS confidence VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS health_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS rate_source VARCHAR(40) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS upstream_relay_usage_delta_samples (
    id BIGSERIAL PRIMARY KEY,
    candidate_id BIGINT NOT NULL REFERENCES upstream_relay_candidates(id) ON DELETE CASCADE,
    probe_result_id BIGINT REFERENCES upstream_relay_probe_results(id) ON DELETE SET NULL,
    model VARCHAR(160) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'insufficient',
    before_cost NUMERIC(20, 8),
    before_actual_cost NUMERIC(20, 8),
    after_cost NUMERIC(20, 8),
    after_actual_cost NUMERIC(20, 8),
    cost_delta NUMERIC(20, 8),
    actual_cost_delta NUMERIC(20, 8),
    derived_rate_multiplier NUMERIC(20, 8),
    unreliable_reason TEXT NOT NULL DEFAULT '',
    sampled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_upstream_relay_usage_delta_status CHECK (status IN ('reliable', 'insufficient', 'unavailable'))
);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_usage_delta_candidate_time
    ON upstream_relay_usage_delta_samples (candidate_id, sampled_at DESC);
