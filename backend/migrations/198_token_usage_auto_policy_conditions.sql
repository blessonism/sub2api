ALTER TABLE token_usage_auto_policy_tiers
    ADD COLUMN IF NOT EXISTS condition_mode TEXT NOT NULL DEFAULT 'token',
    ADD COLUMN IF NOT EXISTS min_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0;

ALTER TABLE token_usage_auto_policy_tiers
    DROP CONSTRAINT IF EXISTS uq_token_usage_auto_policy_tiers_min_tokens;

ALTER TABLE token_usage_auto_policy_tiers
    DROP CONSTRAINT IF EXISTS chk_token_usage_auto_policy_tiers_condition_mode,
    DROP CONSTRAINT IF EXISTS chk_token_usage_auto_policy_tiers_min_actual_cost,
    DROP CONSTRAINT IF EXISTS uq_token_usage_auto_policy_tiers_condition;

ALTER TABLE token_usage_auto_policy_tiers
    ADD CONSTRAINT chk_token_usage_auto_policy_tiers_condition_mode
        CHECK (condition_mode IN ('token', 'actual_cost', 'both')),
    ADD CONSTRAINT chk_token_usage_auto_policy_tiers_min_actual_cost
        CHECK (min_actual_cost >= 0),
    ADD CONSTRAINT uq_token_usage_auto_policy_tiers_condition
        UNIQUE (policy_id, condition_mode, min_tokens, min_actual_cost);

ALTER TABLE token_usage_auto_assignments
    ADD COLUMN IF NOT EXISTS last_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0;

ALTER TABLE token_usage_auto_run_changes
    ADD COLUMN IF NOT EXISTS actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tier_condition_mode TEXT,
    ADD COLUMN IF NOT EXISTS tier_min_actual_cost DECIMAL(20,10);

ALTER TABLE token_usage_auto_run_changes
    DROP CONSTRAINT IF EXISTS chk_token_usage_auto_run_changes_tier_condition_mode;

ALTER TABLE token_usage_auto_run_changes
    ADD CONSTRAINT chk_token_usage_auto_run_changes_tier_condition_mode
        CHECK (tier_condition_mode IS NULL OR tier_condition_mode IN ('token', 'actual_cost', 'both'));
