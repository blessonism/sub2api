-- Token 用量自动策略执行明细。
-- 记录每次真实执行时的用户级变化，便于管理员回看倍率由多少调整到多少。

CREATE TABLE IF NOT EXISTS token_usage_auto_run_changes (
    id                  BIGSERIAL PRIMARY KEY,
    run_id              BIGINT NOT NULL REFERENCES token_usage_auto_runs(id) ON DELETE CASCADE,
    policy_id           BIGINT NOT NULL REFERENCES token_usage_auto_policies(id) ON DELETE CASCADE,
    change_type         TEXT NOT NULL,
    user_id             BIGINT NOT NULL,
    user_name           TEXT NULL,
    user_email          TEXT NULL,
    token_usage         BIGINT NOT NULL DEFAULT 0,
    target_group_id     BIGINT NOT NULL,
    tier_id             BIGINT NULL REFERENCES token_usage_auto_policy_tiers(id) ON DELETE SET NULL,
    tier_min_tokens     BIGINT NULL,
    old_rate_multiplier DECIMAL(10,4) NULL,
    new_rate_multiplier DECIMAL(10,4) NULL,
    reason              TEXT NULL,
    group_granted       BOOLEAN NOT NULL DEFAULT FALSE,
    manual_takeover     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_token_usage_auto_run_changes_change_type CHECK (change_type IN ('create', 'update', 'downgrade', 'clear', 'skip_manual'))
);

CREATE INDEX IF NOT EXISTS idx_token_usage_auto_run_changes_run
    ON token_usage_auto_run_changes(run_id, id);

CREATE INDEX IF NOT EXISTS idx_token_usage_auto_run_changes_policy_user
    ON token_usage_auto_run_changes(policy_id, user_id);

COMMENT ON TABLE token_usage_auto_run_changes IS 'Token 用量自动策略执行的用户级变化明细';
