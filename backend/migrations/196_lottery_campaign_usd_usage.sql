-- 抽奖活动支持按 Token 或用户实际美元消费计算资格。

ALTER TABLE lottery_campaigns
    ADD COLUMN IF NOT EXISTS usage_mode VARCHAR(16) NOT NULL DEFAULT 'token',
    ADD COLUMN IF NOT EXISTS threshold_cost_microusd BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS entry_step_cost_microusd BIGINT NOT NULL DEFAULT 0;

ALTER TABLE lottery_entries
    ADD COLUMN IF NOT EXISTS cost_microusd BIGINT NOT NULL DEFAULT 0;

ALTER TABLE lottery_campaigns DROP CONSTRAINT IF EXISTS lottery_campaigns_amounts_valid;
ALTER TABLE lottery_campaigns DROP CONSTRAINT IF EXISTS lottery_campaigns_usage_mode_valid;
ALTER TABLE lottery_campaigns
    ADD CONSTRAINT lottery_campaigns_usage_mode_valid CHECK (usage_mode IN ('token', 'usd')),
    ADD CONSTRAINT lottery_campaigns_amounts_valid CHECK (
        max_entries_per_user > 0
        AND threshold_tokens >= 0
        AND entry_step_tokens >= 0
        AND threshold_cost_microusd >= 0
        AND entry_step_cost_microusd >= 0
        AND ((usage_mode = 'token' AND threshold_tokens > 0) OR (usage_mode = 'usd' AND threshold_cost_microusd > 0))
    );

ALTER TABLE lottery_entries DROP CONSTRAINT IF EXISTS lottery_entries_amounts_valid;
ALTER TABLE lottery_entries
    ADD CONSTRAINT lottery_entries_amounts_valid CHECK (tokens >= 0 AND cost_microusd >= 0 AND entry_count > 0);
