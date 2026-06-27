CREATE TABLE IF NOT EXISTS upstream_relay_connectors (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    base_url TEXT NOT NULL,
    auth_mode VARCHAR(40) NOT NULL DEFAULT 'manual_session',
    bearer_token_encrypted TEXT,
    refresh_token_encrypted TEXT,
    login_email_encrypted TEXT,
    cookie_encrypted TEXT,
    user_agent_encrypted TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'invalid',
    credential_version BIGINT NOT NULL DEFAULT 1,
    last_verified_at TIMESTAMPTZ,
    last_synced_at TIMESTAMPTZ,
    last_error TEXT,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_upstream_relay_connector_auth_mode CHECK (auth_mode IN ('manual_session', 'password_login')),
    CONSTRAINT chk_upstream_relay_connector_status CHECK (status IN ('active', 'needs_reauth', 'invalid', 'paused'))
);

ALTER TABLE upstream_relay_connectors
    ADD COLUMN IF NOT EXISTS refresh_token_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS login_email_encrypted TEXT;

CREATE TABLE IF NOT EXISTS upstream_relay_group_rate_snapshots (
    id BIGSERIAL PRIMARY KEY,
    connector_id BIGINT NOT NULL REFERENCES upstream_relay_connectors(id) ON DELETE CASCADE,
    upstream_group_id VARCHAR(120) NOT NULL,
    name VARCHAR(160) NOT NULL DEFAULT '',
    platform VARCHAR(80) NOT NULL DEFAULT '',
    status VARCHAR(40) NOT NULL DEFAULT '',
    default_rate_multiplier NUMERIC(20, 8) NOT NULL DEFAULT 1,
    override_rate_multiplier NUMERIC(20, 8),
    final_rate_multiplier NUMERIC(20, 8) NOT NULL DEFAULT 1,
    source VARCHAR(40) NOT NULL DEFAULT 'login_available_groups',
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_upstream_relay_snapshot_connector_group UNIQUE (connector_id, upstream_group_id),
    CONSTRAINT chk_upstream_relay_snapshot_source CHECK (source IN ('login_available_groups', 'login_user_group_rates', 'usage_cost_delta'))
);

CREATE TABLE IF NOT EXISTS upstream_relay_candidates (
    id BIGSERIAL PRIMARY KEY,
    connector_id BIGINT NOT NULL REFERENCES upstream_relay_connectors(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    upstream_group_id VARCHAR(120) NOT NULL,
    probe_model VARCHAR(160) NOT NULL,
    probe_protocol VARCHAR(40) NOT NULL DEFAULT 'chat_completions',
    target_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notes TEXT NOT NULL DEFAULT '',
    last_probe_result_id BIGINT,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_upstream_relay_candidate_protocol CHECK (probe_protocol IN ('chat_completions', 'responses'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_upstream_relay_candidate_account_group_active
    ON upstream_relay_candidates (account_id, target_group_id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS upstream_relay_probe_results (
    id BIGSERIAL PRIMARY KEY,
    candidate_id BIGINT NOT NULL REFERENCES upstream_relay_candidates(id) ON DELETE CASCADE,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms INTEGER,
    http_status INTEGER,
    error_class VARCHAR(60) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    probed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE upstream_relay_candidates
    ADD CONSTRAINT fk_upstream_relay_candidates_last_probe
    FOREIGN KEY (last_probe_result_id) REFERENCES upstream_relay_probe_results(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS upstream_relay_recommendation_runs (
    id BIGSERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'success',
    total_candidates INTEGER NOT NULL DEFAULT 0,
    suggestion_count INTEGER NOT NULL DEFAULT 0,
    applied BOOLEAN NOT NULL DEFAULT FALSE,
    applied_by BIGINT,
    applied_at TIMESTAMPTZ,
    error_message TEXT,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_upstream_relay_recommendation_status CHECK (status IN ('success', 'failed'))
);

CREATE TABLE IF NOT EXISTS upstream_relay_recommendation_suggestions (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES upstream_relay_recommendation_runs(id) ON DELETE CASCADE,
    candidate_id BIGINT NOT NULL REFERENCES upstream_relay_candidates(id) ON DELETE CASCADE,
    connector_id BIGINT NOT NULL REFERENCES upstream_relay_connectors(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    upstream_group_id VARCHAR(120) NOT NULL,
    target_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    old_priority INTEGER,
    new_priority INTEGER NOT NULL,
    final_rate_multiplier NUMERIC(20, 8) NOT NULL DEFAULT 1,
    health_status VARCHAR(40) NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    applied BOOLEAN NOT NULL DEFAULT FALSE,
    applied_by BIGINT,
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_upstream_relay_recommendation_run_candidate UNIQUE (run_id, candidate_id)
);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_connectors_status
    ON upstream_relay_connectors (status) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_upstream_relay_snapshots_connector_seen
    ON upstream_relay_group_rate_snapshots (connector_id, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_candidates_connector
    ON upstream_relay_candidates (connector_id, enabled) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_upstream_relay_probe_candidate_time
    ON upstream_relay_probe_results (candidate_id, probed_at DESC);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_recommendation_runs_created
    ON upstream_relay_recommendation_runs (created_at DESC);
