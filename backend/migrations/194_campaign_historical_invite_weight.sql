-- 邀请活动：按活动配置折算历史已充值达标邀请，并保留可审计快照。

ALTER TABLE campaign_config_versions
    ADD COLUMN IF NOT EXISTS historical_invite_ratio NUMERIC(12, 8) NOT NULL DEFAULT 0;

ALTER TABLE campaign_config_versions
    DROP CONSTRAINT IF EXISTS campaign_config_historical_invite_ratio_valid;

ALTER TABLE campaign_config_versions
    ADD CONSTRAINT campaign_config_historical_invite_ratio_valid
    CHECK (historical_invite_ratio >= 0 AND historical_invite_ratio <= 1);

ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS historical_invite_snapshot_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS campaign_historical_invite_snapshots (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    config_version_id BIGINT NOT NULL REFERENCES campaign_config_versions(id),
    inviter_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_at TIMESTAMPTZ NOT NULL,
    snapshot_cutoff_at TIMESTAMPTZ NOT NULL,
    threshold_snapshot_cents BIGINT NOT NULL,
    historical_recharge_amount_cents BIGINT NOT NULL,
    ratio_snapshot NUMERIC(12, 8) NOT NULL,
    qualified_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT campaign_historical_invite_unique_invitee UNIQUE (campaign_id, invitee_user_id),
    CONSTRAINT campaign_historical_invite_not_self CHECK (inviter_user_id <> invitee_user_id),
    CONSTRAINT campaign_historical_invite_amounts_valid CHECK (
        threshold_snapshot_cents >= 0 AND historical_recharge_amount_cents >= threshold_snapshot_cents
    ),
    CONSTRAINT campaign_historical_invite_ratio_valid CHECK (ratio_snapshot >= 0 AND ratio_snapshot <= 1)
);

CREATE INDEX IF NOT EXISTS campaign_historical_invites_inviter_idx
    ON campaign_historical_invite_snapshots (campaign_id, inviter_user_id);

ALTER TABLE campaign_leaderboard_snapshots
    ALTER COLUMN valid_invite_count TYPE NUMERIC(20, 8) USING valid_invite_count::numeric;
