CREATE TABLE IF NOT EXISTS campaign_leaderboard_adjustments (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    adjustment_type VARCHAR(32) NOT NULL DEFAULT 'manual_delta',
    valid_invite_delta INTEGER NOT NULL DEFAULT 0,
    recharge_amount_delta_cents BIGINT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    operator_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_leaderboard_adjustment_type_valid CHECK (adjustment_type IN ('manual_delta')),
    CONSTRAINT campaign_leaderboard_adjustment_non_empty CHECK (valid_invite_delta <> 0 OR recharge_amount_delta_cents <> 0)
);

CREATE INDEX IF NOT EXISTS campaign_leaderboard_adjustments_campaign_user_idx
    ON campaign_leaderboard_adjustments (campaign_id, user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS campaign_invite_record_adjustments (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    invite_record_id BIGINT NOT NULL REFERENCES campaign_invite_records(id) ON DELETE CASCADE,
    inviter_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    old_status VARCHAR(32) NOT NULL,
    new_status VARCHAR(32) NOT NULL,
    old_effective_recharge_amount_cents BIGINT NOT NULL,
    new_effective_recharge_amount_cents BIGINT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    operator_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS campaign_invite_record_adjustments_record_idx
    ON campaign_invite_record_adjustments (invite_record_id, created_at DESC);
