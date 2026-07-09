-- 邀请活动：补齐冻结榜单状态和运营临时暂停状态。

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'campaigns_status_valid'
    ) THEN
        ALTER TABLE campaigns
            DROP CONSTRAINT campaigns_status_valid;
    END IF;

    ALTER TABLE campaigns
        ADD CONSTRAINT campaigns_status_valid
        CHECK (status IN ('draft', 'warmup', 'active', 'frozen', 'paused', 'auditing', 'publicizing', 'pending_payout', 'paid', 'cancelled', 'terminated'));
END $$;
