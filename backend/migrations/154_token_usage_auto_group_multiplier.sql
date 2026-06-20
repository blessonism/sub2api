-- Token 用量自动分组倍率策略
-- 单条策略固定目标分组，根据用户近 7/30 天 token 用量命中档位后写入用户专属倍率。

CREATE TABLE IF NOT EXISTS token_usage_auto_policies (
    id                 BIGSERIAL PRIMARY KEY,
    name               TEXT NOT NULL,
    enabled            BOOLEAN NOT NULL DEFAULT TRUE,
    window_days        INTEGER NOT NULL,
    target_group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    action_mode        TEXT NOT NULL,
    conflict_mode      TEXT NOT NULL DEFAULT 'manual_priority',
    schedule_frequency TEXT NOT NULL DEFAULT 'daily',
    filter_group_id    BIGINT NULL REFERENCES groups(id) ON DELETE SET NULL,
    filter_model       TEXT NULL,
    filter_request_type SMALLINT NULL,
    filter_billing_type SMALLINT NULL,
    last_run_at        TIMESTAMPTZ NULL,
    next_run_at        TIMESTAMPTZ NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_token_usage_auto_policies_window_days CHECK (window_days IN (7, 30)),
    CONSTRAINT chk_token_usage_auto_policies_action_mode CHECK (action_mode IN ('rate_only', 'grant_group_and_rate')),
    CONSTRAINT chk_token_usage_auto_policies_conflict_mode CHECK (conflict_mode IN ('manual_priority', 'auto_priority')),
    CONSTRAINT chk_token_usage_auto_policies_schedule_frequency CHECK (schedule_frequency IN ('every_6h', 'daily', 'weekly')),
    CONSTRAINT chk_token_usage_auto_policies_filter_request_type CHECK (filter_request_type IS NULL OR filter_request_type IN (0, 1, 2, 3, 4)),
    CONSTRAINT chk_token_usage_auto_policies_filter_billing_type CHECK (filter_billing_type IS NULL OR filter_billing_type IN (0, 1))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_token_usage_auto_policies_enabled_target_group
    ON token_usage_auto_policies(target_group_id)
    WHERE enabled = TRUE;

CREATE INDEX IF NOT EXISTS idx_token_usage_auto_policies_due
    ON token_usage_auto_policies(enabled, next_run_at)
    WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS token_usage_auto_policy_tiers (
    id                  BIGSERIAL PRIMARY KEY,
    policy_id           BIGINT NOT NULL REFERENCES token_usage_auto_policies(id) ON DELETE CASCADE,
    min_tokens          BIGINT NOT NULL,
    rate_multiplier     DECIMAL(10,4) NOT NULL,
    sort_order          INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_token_usage_auto_policy_tiers_min_tokens CHECK (min_tokens >= 0),
    CONSTRAINT chk_token_usage_auto_policy_tiers_rate_multiplier CHECK (rate_multiplier > 0),
    CONSTRAINT uq_token_usage_auto_policy_tiers_min_tokens UNIQUE (policy_id, min_tokens)
);

CREATE INDEX IF NOT EXISTS idx_token_usage_auto_policy_tiers_policy_order
    ON token_usage_auto_policy_tiers(policy_id, sort_order, min_tokens);

CREATE TABLE IF NOT EXISTS token_usage_auto_assignments (
    id                         BIGSERIAL PRIMARY KEY,
    policy_id                  BIGINT NOT NULL REFERENCES token_usage_auto_policies(id) ON DELETE CASCADE,
    user_id                    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_group_id            BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    tier_id                    BIGINT NULL REFERENCES token_usage_auto_policy_tiers(id) ON DELETE SET NULL,
    last_token_usage           BIGINT NOT NULL DEFAULT 0,
    last_rate_multiplier       DECIMAL(10,4) NULL,
    group_granted_by_policy    BOOLEAN NOT NULL DEFAULT FALSE,
    previous_rate_multiplier   DECIMAL(10,4) NULL,
    manual_takeover            BOOLEAN NOT NULL DEFAULT FALSE,
    manual_takeover_reason     TEXT NULL,
    manual_takeover_at         TIMESTAMPTZ NULL,
    last_applied_at            TIMESTAMPTZ NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_token_usage_auto_assignments_policy_user UNIQUE (policy_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_token_usage_auto_assignments_user_group
    ON token_usage_auto_assignments(user_id, target_group_id);

CREATE INDEX IF NOT EXISTS idx_token_usage_auto_assignments_policy_group
    ON token_usage_auto_assignments(policy_id, target_group_id);

CREATE TABLE IF NOT EXISTS token_usage_auto_runs (
    id              BIGSERIAL PRIMARY KEY,
    policy_id       BIGINT NOT NULL REFERENCES token_usage_auto_policies(id) ON DELETE CASCADE,
    run_type        TEXT NOT NULL,
    status          TEXT NOT NULL,
    total_users     INTEGER NOT NULL DEFAULT 0,
    create_count    INTEGER NOT NULL DEFAULT 0,
    update_count    INTEGER NOT NULL DEFAULT 0,
    downgrade_count INTEGER NOT NULL DEFAULT 0,
    clear_count     INTEGER NOT NULL DEFAULT 0,
    skip_count      INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_token_usage_auto_runs_run_type CHECK (run_type IN ('preview', 'manual', 'scheduled')),
    CONSTRAINT chk_token_usage_auto_runs_status CHECK (status IN ('running', 'success', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_token_usage_auto_runs_policy_created
    ON token_usage_auto_runs(policy_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_token_usage_auto_runs_one_running
    ON token_usage_auto_runs(policy_id)
    WHERE status = 'running';

COMMENT ON TABLE token_usage_auto_policies IS 'Token 用量自动分组倍率策略';
COMMENT ON TABLE token_usage_auto_policy_tiers IS 'Token 用量自动分组倍率策略档位';
COMMENT ON TABLE token_usage_auto_assignments IS 'Token 用量自动策略产生的用户归属与接管状态';
COMMENT ON TABLE token_usage_auto_runs IS 'Token 用量自动策略执行历史';
