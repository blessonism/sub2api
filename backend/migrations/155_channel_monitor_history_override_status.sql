ALTER TABLE channel_monitor_histories
    ADD COLUMN IF NOT EXISTS override_status VARCHAR(20);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'channel_monitor_histories_override_status_check'
    ) THEN
        ALTER TABLE channel_monitor_histories
            ADD CONSTRAINT channel_monitor_histories_override_status_check
            CHECK (override_status IS NULL OR override_status IN ('operational', 'degraded', 'failed'));
    END IF;
END $$;
