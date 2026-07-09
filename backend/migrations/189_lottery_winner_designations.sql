-- 189_lottery_winner_designations.sql
-- 管理员预置获奖者：在开奖批次创建前指定某日期的获奖者名单（与批次解耦）。
-- Draw() 执行时读取此表：先按预置填入指定获奖者，再对剩余名额随机补齐。

CREATE TABLE IF NOT EXISTS lottery_winner_designations (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES lottery_campaigns(id) ON DELETE CASCADE,
    draw_date DATE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prize_tier_id BIGINT NOT NULL REFERENCES lottery_prize_tiers(id),
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_winner_designations_unique UNIQUE (campaign_id, draw_date, user_id)
);

CREATE INDEX IF NOT EXISTS lottery_winner_designations_campaign_date_idx
    ON lottery_winner_designations (campaign_id, draw_date);
