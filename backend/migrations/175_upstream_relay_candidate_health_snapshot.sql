-- 上游中继候选健康快照：持久化最近一次探测健康摘要，避免重启或查询窗口变化后列表无法展示健康状态。

CREATE TABLE IF NOT EXISTS upstream_relay_candidate_health_snapshots (
    candidate_id BIGINT PRIMARY KEY REFERENCES upstream_relay_candidates(id) ON DELETE CASCADE,
    probe_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    success_rate NUMERIC(10, 6) NOT NULL DEFAULT 0,
    avg_latency_ms INTEGER,
    p95_latency_ms INTEGER,
    consecutive_successes INTEGER NOT NULL DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_error_class VARCHAR(60) NOT NULL DEFAULT '',
    last_success_at TIMESTAMPTZ,
    window_minutes INTEGER NOT NULL DEFAULT 1440,
    sample_size INTEGER NOT NULL DEFAULT 0,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_candidate_health_calculated_at
    ON upstream_relay_candidate_health_snapshots (calculated_at DESC);
