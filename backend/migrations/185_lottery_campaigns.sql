-- 185_lottery_campaigns.sql
-- Token 达标抽奖活动：活动配置、资格快照、开奖批次、中奖与余额发放审计。

CREATE TABLE IF NOT EXISTS lottery_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    rules_text TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    participation_mode VARCHAR(32) NOT NULL,
    draw_schedule_type VARCHAR(32) NOT NULL,
    prize_mode VARCHAR(32) NOT NULL,
    entry_mode VARCHAR(32) NOT NULL,
    threshold_tokens BIGINT NOT NULL,
    entry_step_tokens BIGINT NOT NULL DEFAULT 0,
    max_entries_per_user INTEGER NOT NULL DEFAULT 1,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    draw_at TIMESTAMPTZ,
    daily_draw_time VARCHAR(5) NOT NULL DEFAULT '',
    created_by BIGINT,
    updated_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_campaigns_status_valid CHECK (status IN ('draft', 'published', 'cancelled', 'archived')),
    CONSTRAINT lottery_campaigns_participation_valid CHECK (participation_mode IN ('auto', 'manual')),
    CONSTRAINT lottery_campaigns_schedule_valid CHECK (draw_schedule_type IN ('single', 'daily')),
    CONSTRAINT lottery_campaigns_prize_mode_valid CHECK (prize_mode IN ('single', 'multi')),
    CONSTRAINT lottery_campaigns_entry_mode_valid CHECK (entry_mode IN ('daily_once', 'stepped')),
    CONSTRAINT lottery_campaigns_time_order CHECK (end_at > start_at),
    CONSTRAINT lottery_campaigns_amounts_valid CHECK (threshold_tokens > 0 AND entry_step_tokens >= 0 AND max_entries_per_user > 0),
    CONSTRAINT lottery_campaigns_single_draw CHECK (draw_schedule_type <> 'single' OR draw_at IS NOT NULL),
    CONSTRAINT lottery_campaigns_daily_draw CHECK (draw_schedule_type <> 'daily' OR daily_draw_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$')
);

CREATE INDEX IF NOT EXISTS lottery_campaigns_status_time_idx
    ON lottery_campaigns (status, start_at, end_at);

CREATE TABLE IF NOT EXISTS lottery_prize_tiers (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES lottery_campaigns(id) ON DELETE CASCADE,
    tier_name VARCHAR(120) NOT NULL,
    winner_count INTEGER NOT NULL,
    reward_amount_cents BIGINT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_prize_tiers_positive CHECK (winner_count > 0 AND reward_amount_cents > 0)
);

CREATE INDEX IF NOT EXISTS lottery_prize_tiers_campaign_idx
    ON lottery_prize_tiers (campaign_id, sort_order, id);

CREATE TABLE IF NOT EXISTS lottery_entries (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES lottery_campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_date DATE NOT NULL,
    tokens BIGINT NOT NULL,
    entry_count INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL,
    enrolled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_entries_unique UNIQUE (campaign_id, user_id, entry_date),
    CONSTRAINT lottery_entries_status_valid CHECK (status IN ('eligible', 'enrolled')),
    CONSTRAINT lottery_entries_amounts_valid CHECK (tokens >= 0 AND entry_count > 0)
);

CREATE INDEX IF NOT EXISTS lottery_entries_draw_idx
    ON lottery_entries (campaign_id, entry_date, status);

CREATE TABLE IF NOT EXISTS lottery_draw_batches (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES lottery_campaigns(id) ON DELETE CASCADE,
    draw_date DATE NOT NULL,
    scheduled_draw_at TIMESTAMPTZ NOT NULL,
    batch_no VARCHAR(80) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'processing',
    trigger_type VARCHAR(32) NOT NULL DEFAULT 'scheduled',
    operator_id BIGINT,
    total_entries INTEGER NOT NULL DEFAULT 0,
    total_winners INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    drawn_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CONSTRAINT lottery_draw_batches_unique UNIQUE (campaign_id, draw_date),
    CONSTRAINT lottery_draw_batches_batch_unique UNIQUE (batch_no),
    CONSTRAINT lottery_draw_batches_status_valid CHECK (status IN ('processing', 'success', 'partial_success', 'failed'))
);

CREATE INDEX IF NOT EXISTS lottery_draw_batches_campaign_idx
    ON lottery_draw_batches (campaign_id, draw_date DESC);

CREATE TABLE IF NOT EXISTS lottery_winners (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES lottery_draw_batches(id) ON DELETE CASCADE,
    campaign_id BIGINT NOT NULL REFERENCES lottery_campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prize_tier_id BIGINT NOT NULL REFERENCES lottery_prize_tiers(id),
    entry_date DATE NOT NULL,
    reward_amount_cents BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    balance_before_snapshot NUMERIC(18, 6),
    balance_after_snapshot NUMERIC(18, 6),
    idempotency_key VARCHAR(180) NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT lottery_winners_user_once_per_batch UNIQUE (batch_id, user_id),
    CONSTRAINT lottery_winners_idempotency_unique UNIQUE (idempotency_key),
    CONSTRAINT lottery_winners_status_valid CHECK (status IN ('pending', 'success', 'failed')),
    CONSTRAINT lottery_winners_reward_positive CHECK (reward_amount_cents > 0)
);

CREATE INDEX IF NOT EXISTS lottery_winners_user_idx
    ON lottery_winners (campaign_id, user_id, created_at DESC);
