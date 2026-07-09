-- 188_lottery_campaign_featured.sql
-- Token 抽奖活动：后台指定活动中心展示的当前抽奖活动，并允许编辑奖项时保留历史中奖记录引用的旧奖项。

ALTER TABLE lottery_campaigns
    ADD COLUMN IF NOT EXISTS is_featured BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE lottery_prize_tiers
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

CREATE UNIQUE INDEX IF NOT EXISTS lottery_campaigns_single_featured_idx
    ON lottery_campaigns (is_featured)
    WHERE is_featured = TRUE;

CREATE INDEX IF NOT EXISTS lottery_campaigns_featured_active_idx
    ON lottery_campaigns (is_featured, status, start_at, end_at);

CREATE INDEX IF NOT EXISTS lottery_prize_tiers_active_campaign_idx
    ON lottery_prize_tiers (campaign_id, is_active, sort_order, id);
