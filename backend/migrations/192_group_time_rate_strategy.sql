ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS time_rate_priority VARCHAR(32) NOT NULL DEFAULT 'schedule_first',
    ADD COLUMN IF NOT EXISTS time_rate_periods JSONB NOT NULL DEFAULT '[]'::jsonb;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'groups' AND column_name = 'peak_rate_enabled'
    ) THEN
        UPDATE groups
        SET time_rate_priority = 'proportional',
            time_rate_periods = jsonb_build_array(jsonb_build_object(
                'start_time', to_char(peak_start::time, 'HH24:MI'),
                'end_time', to_char(peak_end::time, 'HH24:MI'),
                'rate_multiplier', rate_multiplier * peak_rate_multiplier,
                'visible_rate_multiplier', COALESCE(visible_rate_multiplier, rate_multiplier) * peak_rate_multiplier,
                'enabled', true
            ))
        WHERE peak_rate_enabled = true
          AND peak_start ~ '^([01]?[0-9]|2[0-3]):[0-5][0-9]$'
          AND peak_end ~ '^([01]?[0-9]|2[0-3]):[0-5][0-9]$'
          AND peak_start::time < peak_end::time;

        ALTER TABLE groups
            DROP COLUMN IF EXISTS peak_rate_enabled,
            DROP COLUMN IF EXISTS peak_start,
            DROP COLUMN IF EXISTS peak_end,
            DROP COLUMN IF EXISTS peak_rate_multiplier;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'groups_time_rate_priority_check'
          AND conrelid = to_regclass('groups')
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_time_rate_priority_check
            CHECK (time_rate_priority IN ('schedule_first', 'user_first', 'proportional'));
    END IF;
END $$;

COMMENT ON COLUMN groups.time_rate_priority IS '分时倍率优先模式：schedule_first、user_first、proportional';
COMMENT ON COLUMN groups.time_rate_periods IS '按服务器时区每日循环的分时倍率区间 JSON 数组';
