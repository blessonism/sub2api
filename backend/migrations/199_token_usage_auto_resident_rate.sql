-- Token 自动策略常驻档位：按全历史累计用量判断的保底倍率。
-- 常驻档位复用现有条件模式（token / actual_cost / both），但统计口径为全站累计。

ALTER TABLE token_usage_auto_policy_tiers
    ADD COLUMN IF NOT EXISTS is_resident BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE token_usage_auto_policy_tiers
    DROP CONSTRAINT IF EXISTS uq_token_usage_auto_policy_tiers_condition;

ALTER TABLE token_usage_auto_policy_tiers
    ADD CONSTRAINT uq_token_usage_auto_policy_tiers_condition
    UNIQUE (policy_id, is_resident, condition_mode, min_tokens, min_actual_cost);

-- 用户全历史累计用量（全站口径，actual_cost > 0 成功落账）。
-- last_processed_id 记录已纳入统计的最大 usage_logs.id，用于增量刷新。
CREATE TABLE IF NOT EXISTS token_usage_auto_user_totals (
    user_id            BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_tokens       BIGINT NOT NULL DEFAULT 0,
    total_actual_cost  DECIMAL(20,10) NOT NULL DEFAULT 0,
    last_processed_id  BIGINT NOT NULL DEFAULT 0,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE token_usage_auto_user_totals IS 'Token 自动策略常驻档位使用的用户全历史累计用量';

ALTER TABLE token_usage_auto_assignments
    ADD COLUMN IF NOT EXISTS resident_tier_id BIGINT NULL
        REFERENCES token_usage_auto_policy_tiers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS last_total_token_usage BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_total_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0;

ALTER TABLE token_usage_auto_run_changes
    ADD COLUMN IF NOT EXISTS total_token_usage BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS resident_tier_id BIGINT NULL
        REFERENCES token_usage_auto_policy_tiers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS resident_tier_min_tokens BIGINT NULL,
    ADD COLUMN IF NOT EXISTS resident_tier_condition_mode TEXT NULL,
    ADD COLUMN IF NOT EXISTS resident_tier_min_actual_cost DECIMAL(20,10) NULL;

ALTER TABLE token_usage_auto_run_changes
    DROP CONSTRAINT IF EXISTS chk_token_usage_auto_run_changes_resident_condition;

ALTER TABLE token_usage_auto_run_changes
    ADD CONSTRAINT chk_token_usage_auto_run_changes_resident_condition
    CHECK (resident_tier_condition_mode IS NULL
           OR resident_tier_condition_mode IN ('token', 'actual_cost', 'both'));
