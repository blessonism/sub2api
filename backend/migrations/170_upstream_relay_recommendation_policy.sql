CREATE TABLE IF NOT EXISTS upstream_relay_recommendation_policy (
    id SMALLINT PRIMARY KEY DEFAULT 1,
    snapshot_freshness_minutes INTEGER NOT NULL DEFAULT 1440,
    usage_delta_freshness_minutes INTEGER NOT NULL DEFAULT 1440,
    probe_freshness_minutes INTEGER NOT NULL DEFAULT 30,
    min_success_rate NUMERIC(6, 5) NOT NULL DEFAULT 0.5,
    min_sample_size INTEGER NOT NULL DEFAULT 3,
    exclude_consecutive_failures BOOLEAN NOT NULL DEFAULT TRUE,
    priority_start INTEGER NOT NULL DEFAULT 10,
    priority_step INTEGER NOT NULL DEFAULT 10,
    sort_fields TEXT[] NOT NULL DEFAULT ARRAY['rate_asc', 'success_rate_desc', 'latency_asc']::TEXT[],
    updated_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_upstream_relay_recommendation_policy_singleton CHECK (id = 1),
    CONSTRAINT chk_upstream_relay_recommendation_policy_freshness CHECK (
        snapshot_freshness_minutes > 0
        AND usage_delta_freshness_minutes > 0
        AND probe_freshness_minutes > 0
    ),
    CONSTRAINT chk_upstream_relay_recommendation_policy_success_rate CHECK (
        min_success_rate >= 0
        AND min_success_rate <= 1
    ),
    CONSTRAINT chk_upstream_relay_recommendation_policy_priority CHECK (
        min_sample_size > 0
        AND priority_step > 0
    )
);
