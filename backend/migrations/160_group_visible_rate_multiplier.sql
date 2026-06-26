-- 分组倍率双口径：rate_multiplier 保持真实扣费倍率，visible_rate_multiplier 仅用于用户侧展示。
ALTER TABLE IF EXISTS groups
    ADD COLUMN IF NOT EXISTS visible_rate_multiplier DECIMAL(10,4);

ALTER TABLE IF EXISTS user_group_rate_multipliers
    ADD COLUMN IF NOT EXISTS visible_rate_multiplier DECIMAL(10,4);

ALTER TABLE IF EXISTS usage_logs
    ADD COLUMN IF NOT EXISTS visible_rate_multiplier DECIMAL(10,4);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_visible_rate_multiplier_positive_check'
          AND conrelid = to_regclass('groups')
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_visible_rate_multiplier_positive_check
            CHECK (visible_rate_multiplier IS NULL OR visible_rate_multiplier > 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_group_rate_visible_multiplier_positive_check'
          AND conrelid = to_regclass('user_group_rate_multipliers')
    ) THEN
        ALTER TABLE user_group_rate_multipliers
            ADD CONSTRAINT user_group_rate_visible_multiplier_positive_check
            CHECK (visible_rate_multiplier IS NULL OR visible_rate_multiplier > 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_visible_rate_multiplier_positive_check'
          AND conrelid = to_regclass('usage_logs')
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_visible_rate_multiplier_positive_check
            CHECK (visible_rate_multiplier IS NULL OR visible_rate_multiplier > 0);
    END IF;
END $$;

COMMENT ON COLUMN groups.visible_rate_multiplier IS '分组默认用户可见倍率；NULL 表示沿用实际分组倍率。';
COMMENT ON COLUMN user_group_rate_multipliers.visible_rate_multiplier IS '用户专属可见倍率；NULL 表示继承分组可见倍率或实际倍率。';
COMMENT ON COLUMN usage_logs.visible_rate_multiplier IS '请求发生时的用户可见倍率快照；NULL 表示历史数据，用户侧回退 rate_multiplier。';
