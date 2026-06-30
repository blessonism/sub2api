ALTER TABLE upstream_relay_recommendation_runs
    ADD COLUMN IF NOT EXISTS closed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS closed_by BIGINT,
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_upstream_relay_recommendation_runs_closed
    ON upstream_relay_recommendation_runs (closed, created_at DESC);
