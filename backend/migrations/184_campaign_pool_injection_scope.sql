-- 184_campaign_pool_injection_scope.sql
-- 邀请活动：支持配置奖池充值注入范围，并允许非邀请充值仅注入奖池。

ALTER TABLE campaign_config_versions
    ADD COLUMN IF NOT EXISTS pool_injection_scope VARCHAR(32) NOT NULL DEFAULT 'invitees_only';

ALTER TABLE campaign_pool_entries
    ALTER COLUMN invite_record_id DROP NOT NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'campaign_config_version_scope_valid'
    ) THEN
        ALTER TABLE campaign_config_versions
            DROP CONSTRAINT campaign_config_version_scope_valid;
    END IF;

    ALTER TABLE campaign_config_versions
        ADD CONSTRAINT campaign_config_version_scope_valid
        CHECK (version_scope IN ('publish_snapshot', 'threshold_adjustment', 'injection_rate_adjustment', 'pool_scope_adjustment'));
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'campaign_config_pool_injection_scope_valid'
    ) THEN
        ALTER TABLE campaign_config_versions
            ADD CONSTRAINT campaign_config_pool_injection_scope_valid
            CHECK (pool_injection_scope IN ('invitees_only', 'all_users'));
    END IF;
END $$;
