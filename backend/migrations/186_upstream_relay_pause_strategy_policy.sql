ALTER TABLE upstream_relay_recommendation_policy
    ADD COLUMN IF NOT EXISTS pause_rate_gap_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pause_rate_gap_threshold NUMERIC(20, 8) NOT NULL DEFAULT 0.04,
    ADD COLUMN IF NOT EXISTS pause_consecutive_failures_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pause_consecutive_failures_threshold INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS pause_success_rate_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE upstream_relay_recommendation_policy
    DROP CONSTRAINT IF EXISTS chk_upstream_relay_recommendation_policy_pause_strategy,
    ADD CONSTRAINT chk_upstream_relay_recommendation_policy_pause_strategy CHECK (
        pause_rate_gap_threshold > 0
        AND pause_consecutive_failures_threshold > 0
    );
