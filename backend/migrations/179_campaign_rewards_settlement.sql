-- 179_campaign_rewards_settlement.sql
-- 限时邀请奖励活动：活动、版本、邀请、奖金池、结算、发放与追偿审计表。

CREATE TABLE IF NOT EXISTS campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    rules_text TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    warmup_start_at TIMESTAMPTZ,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    audit_start_at TIMESTAMPTZ,
    audit_end_at TIMESTAMPTZ,
    publicity_start_at TIMESTAMPTZ,
    publicity_end_at TIMESTAMPTZ,
    payout_due_at TIMESTAMPTZ,
    published_config_version_id BIGINT,
    created_by BIGINT,
    updated_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaigns_time_order CHECK (end_at > start_at),
    CONSTRAINT campaigns_status_valid CHECK (status IN ('draft', 'warmup', 'active', 'auditing', 'publicizing', 'pending_payout', 'paid', 'cancelled', 'terminated'))
);

CREATE UNIQUE INDEX IF NOT EXISTS campaigns_single_active_idx
    ON campaigns ((status))
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS campaigns_status_time_idx
    ON campaigns (status, start_at, end_at);

CREATE TABLE IF NOT EXISTS campaign_config_versions (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    version_scope VARCHAR(32) NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    recharge_threshold_cents BIGINT NOT NULL,
    allow_accumulated_recharge BOOLEAN NOT NULL DEFAULT TRUE,
    pool_injection_rate NUMERIC(12, 8) NOT NULL,
    rank_pool_ratio NUMERIC(12, 8) NOT NULL,
    contribution_pool_ratio NUMERIC(12, 8) NOT NULL,
    rank_reward_count INTEGER NOT NULL,
    rank_weights_json JSONB NOT NULL,
    min_payout_amount_cents BIGINT NOT NULL,
    payout_method VARCHAR(32) NOT NULL DEFAULT 'balance',
    payout_channel VARCHAR(32) NOT NULL DEFAULT 'account_balance',
    change_reason TEXT NOT NULL DEFAULT '',
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_config_version_unique UNIQUE (campaign_id, version),
    CONSTRAINT campaign_config_version_scope_valid CHECK (version_scope IN ('publish_snapshot', 'threshold_adjustment', 'injection_rate_adjustment')),
    CONSTRAINT campaign_config_amounts_non_negative CHECK (recharge_threshold_cents >= 0 AND min_payout_amount_cents >= 0),
    CONSTRAINT campaign_config_ratios_valid CHECK (pool_injection_rate >= 0 AND rank_pool_ratio >= 0 AND contribution_pool_ratio >= 0),
    CONSTRAINT campaign_config_rank_count_positive CHECK (rank_reward_count > 0)
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'campaigns_published_config_version_fk'
    ) THEN
        ALTER TABLE campaigns
            ADD CONSTRAINT campaigns_published_config_version_fk
            FOREIGN KEY (published_config_version_id)
            REFERENCES campaign_config_versions(id)
            DEFERRABLE INITIALLY DEFERRED;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS campaign_participants (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invite_code_snapshot VARCHAR(64) NOT NULL DEFAULT '',
    invite_link_snapshot TEXT NOT NULL DEFAULT '',
    participant_status VARCHAR(32) NOT NULL DEFAULT 'normal',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_invite_count INTEGER NOT NULL DEFAULT 0,
    pending_invite_count INTEGER NOT NULL DEFAULT 0,
    invalid_invite_count INTEGER NOT NULL DEFAULT 0,
    invitee_recharge_amount_cents BIGINT NOT NULL DEFAULT 0,
    estimated_rank_reward_cents BIGINT NOT NULL DEFAULT 0,
    estimated_contribution_reward_cents BIGINT NOT NULL DEFAULT 0,
    estimated_total_reward_cents BIGINT NOT NULL DEFAULT 0,
    final_rank_reward_cents BIGINT NOT NULL DEFAULT 0,
    final_contribution_reward_cents BIGINT NOT NULL DEFAULT 0,
    final_total_reward_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_participant_unique UNIQUE (campaign_id, user_id),
    CONSTRAINT campaign_participant_status_valid CHECK (participant_status IN ('normal', 'restricted', 'test_account', 'internal_account', 'disqualified'))
);

CREATE TABLE IF NOT EXISTS campaign_invite_records (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    config_version_id BIGINT NOT NULL REFERENCES campaign_config_versions(id),
    inviter_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invite_source VARCHAR(32) NOT NULL DEFAULT 'affiliate_code',
    threshold_snapshot_cents BIGINT NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL,
    qualified_at TIMESTAMPTZ,
    effective_recharge_amount_cents BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'registered',
    risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
    invalid_reason TEXT NOT NULL DEFAULT '',
    audit_status VARCHAR(32) NOT NULL DEFAULT 'not_reviewed',
    audit_by BIGINT,
    audit_at TIMESTAMPTZ,
    audit_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_invite_unique_invitee UNIQUE (campaign_id, invitee_user_id),
    CONSTRAINT campaign_invite_not_self CHECK (inviter_user_id <> invitee_user_id),
    CONSTRAINT campaign_invite_status_valid CHECK (status IN ('registered', 'recharge_unqualified', 'pending_audit', 'effective', 'invalid', 'risk_review')),
    CONSTRAINT campaign_invite_risk_valid CHECK (risk_level IN ('low', 'medium', 'high', 'confirmed_violation')),
    CONSTRAINT campaign_invite_audit_valid CHECK (audit_status IN ('not_reviewed', 'approved', 'rejected', 'needs_review'))
);

CREATE INDEX IF NOT EXISTS campaign_invites_inviter_idx
    ON campaign_invite_records (campaign_id, inviter_user_id, status);

CREATE TABLE IF NOT EXISTS campaign_pool_entries (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    config_version_id BIGINT NOT NULL REFERENCES campaign_config_versions(id),
    invite_record_id BIGINT NOT NULL REFERENCES campaign_invite_records(id) ON DELETE CASCADE,
    invitee_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type VARCHAR(32) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    source_success_at TIMESTAMPTZ NOT NULL,
    effective_recharge_amount_cents BIGINT NOT NULL,
    injection_rate_snapshot NUMERIC(12, 8) NOT NULL,
    pool_amount_cents BIGINT NOT NULL,
    pool_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    confirmed_at TIMESTAMPTZ,
    deducted_amount_cents BIGINT NOT NULL DEFAULT 0,
    last_deduct_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_pool_entry_source_unique UNIQUE (campaign_id, source_type, source_id),
    CONSTRAINT campaign_pool_entry_status_valid CHECK (pool_status IN ('pending', 'confirmed', 'deducted', 'excluded')),
    CONSTRAINT campaign_pool_entry_amounts_non_negative CHECK (effective_recharge_amount_cents >= 0 AND pool_amount_cents >= 0 AND deducted_amount_cents >= 0)
);

CREATE INDEX IF NOT EXISTS campaign_pool_entries_campaign_status_idx
    ON campaign_pool_entries (campaign_id, pool_status, source_success_at);

CREATE TABLE IF NOT EXISTS campaign_pool_deductions (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    pool_entry_id BIGINT NOT NULL REFERENCES campaign_pool_entries(id) ON DELETE CASCADE,
    source_type VARCHAR(32) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    deduct_amount_cents BIGINT NOT NULL,
    idempotency_key VARCHAR(160) NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_pool_deduction_idempotency_unique UNIQUE (idempotency_key),
    CONSTRAINT campaign_pool_deduction_amount_positive CHECK (deduct_amount_cents > 0)
);

CREATE TABLE IF NOT EXISTS campaign_pool_adjustments (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    adjustment_type VARCHAR(32) NOT NULL,
    amount_cents BIGINT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    operator_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_pool_adjustment_type_valid CHECK (adjustment_type IN ('initial_bonus', 'additional_bonus', 'manual_compensation', 'exception_deduction', 'low_amount_recycle', 'rounding_residual_recycle'))
);

CREATE TABLE IF NOT EXISTS campaign_leaderboard_snapshots (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    snapshot_type VARCHAR(32) NOT NULL,
    rank INTEGER NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    valid_invite_count INTEGER NOT NULL,
    invitee_recharge_amount_cents BIGINT NOT NULL,
    reached_count_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ,
    estimated_reward_cents BIGINT NOT NULL DEFAULT 0,
    final_reward_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_leaderboard_type_valid CHECK (snapshot_type IN ('live_estimate', 'end_frozen', 'final')),
    CONSTRAINT campaign_leaderboard_rank_positive CHECK (rank > 0)
);

CREATE INDEX IF NOT EXISTS campaign_leaderboard_snapshot_rank_idx
    ON campaign_leaderboard_snapshots (campaign_id, snapshot_type, rank);

CREATE TABLE IF NOT EXISTS campaign_reward_results (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    config_version_id BIGINT NOT NULL REFERENCES campaign_config_versions(id),
    calculation_batch_no VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rank INTEGER,
    rank_reward_amount_cents BIGINT NOT NULL DEFAULT 0,
    contribution_weight NUMERIC(20, 10) NOT NULL DEFAULT 0,
    contribution_reward_amount_cents BIGINT NOT NULL DEFAULT 0,
    gross_reward_amount_cents BIGINT NOT NULL DEFAULT 0,
    min_payout_amount_snapshot_cents BIGINT NOT NULL,
    final_payout_amount_cents BIGINT NOT NULL DEFAULT 0,
    withheld_amount_cents BIGINT NOT NULL DEFAULT 0,
    withheld_reason TEXT NOT NULL DEFAULT '',
    rounding_residual_cents BIGINT NOT NULL DEFAULT 0,
    calculation_status VARCHAR(32) NOT NULL,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_reward_result_unique UNIQUE (campaign_id, calculation_status, user_id),
    CONSTRAINT campaign_reward_status_valid CHECK (calculation_status IN ('preview', 'frozen', 'final')),
    CONSTRAINT campaign_reward_amounts_non_negative CHECK (
        rank_reward_amount_cents >= 0
        AND contribution_reward_amount_cents >= 0
        AND gross_reward_amount_cents >= 0
        AND final_payout_amount_cents >= 0
        AND withheld_amount_cents >= 0
        AND rounding_residual_cents >= 0
    )
);

CREATE INDEX IF NOT EXISTS campaign_reward_results_campaign_status_idx
    ON campaign_reward_results (campaign_id, calculation_status, final_payout_amount_cents DESC);

CREATE TABLE IF NOT EXISTS campaign_reward_adjustments (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reward_result_id BIGINT REFERENCES campaign_reward_results(id),
    adjustment_type VARCHAR(32) NOT NULL,
    amount_cents BIGINT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    operator_id BIGINT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    payout_batch_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT campaign_reward_adjustment_type_valid CHECK (adjustment_type IN ('supplement', 'recovery', 'manual_correction')),
    CONSTRAINT campaign_reward_adjustment_status_valid CHECK (status IN ('pending', 'included_in_settlement', 'paid', 'voided'))
);

CREATE TABLE IF NOT EXISTS campaign_payout_batches (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    batch_no VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    operator_id BIGINT,
    total_users INTEGER NOT NULL DEFAULT 0,
    total_amount_cents BIGINT NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_payout_batch_unique UNIQUE (campaign_id, batch_no),
    CONSTRAINT campaign_payout_batch_status_valid CHECK (status IN ('pending', 'processing', 'partial_success', 'success', 'failed'))
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'campaign_reward_adjustments_payout_batch_fk'
    ) THEN
        ALTER TABLE campaign_reward_adjustments
            ADD CONSTRAINT campaign_reward_adjustments_payout_batch_fk
            FOREIGN KEY (payout_batch_id)
            REFERENCES campaign_payout_batches(id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS campaign_payout_items (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES campaign_payout_batches(id) ON DELETE CASCADE,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reward_result_id BIGINT NOT NULL REFERENCES campaign_reward_results(id),
    amount_cents BIGINT NOT NULL,
    balance_before_snapshot NUMERIC(20, 8),
    balance_after_snapshot NUMERIC(20, 8),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(160) NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_payout_item_idempotency_unique UNIQUE (idempotency_key),
    CONSTRAINT campaign_payout_item_status_valid CHECK (status IN ('pending', 'success', 'failed', 'skipped')),
    CONSTRAINT campaign_payout_item_amount_non_negative CHECK (amount_cents >= 0)
);

CREATE TABLE IF NOT EXISTS campaign_payout_recoveries (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    payout_item_id BIGINT REFERENCES campaign_payout_items(id),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type VARCHAR(32) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    recover_amount_cents BIGINT NOT NULL,
    recovered_amount_cents BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending_review',
    idempotency_key VARCHAR(160) NOT NULL,
    balance_before_snapshot NUMERIC(20, 8),
    balance_after_snapshot NUMERIC(20, 8),
    balance_ledger_id BIGINT,
    error_message TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    review_by BIGINT,
    review_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_payout_recovery_idempotency_unique UNIQUE (idempotency_key),
    CONSTRAINT campaign_payout_recovery_status_valid CHECK (status IN ('pending_review', 'pending_deduct', 'partial_deducted', 'deducted', 'insufficient_balance', 'voided')),
    CONSTRAINT campaign_payout_recovery_amount_positive CHECK (recover_amount_cents > 0 AND recovered_amount_cents >= 0)
);
