CREATE TABLE IF NOT EXISTS admin_usage_calibrations (
    id BIGSERIAL PRIMARY KEY,
    target_user_id BIGINT NOT NULL REFERENCES users(id),
    admin_user_id BIGINT NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,

    token_mode VARCHAR(20),
    token_input_value BIGINT,
    token_before_value BIGINT,
    token_after_value BIGINT,
    token_delta BIGINT,
    token_calculation_start_date DATE,
    token_calculation_end_date DATE,
    token_calculation_timezone TEXT,

    balance_mode VARCHAR(20),
    balance_input_value NUMERIC(18, 6),
    balance_before_value NUMERIC(18, 6),
    balance_after_value NUMERIC(18, 6),
    balance_delta NUMERIC(18, 6),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT admin_usage_calibrations_token_mode_check
        CHECK (token_mode IS NULL OR token_mode IN ('delta', 'target')),
    CONSTRAINT admin_usage_calibrations_balance_mode_check
        CHECK (balance_mode IS NULL OR balance_mode IN ('delta', 'target')),
    CONSTRAINT admin_usage_calibrations_has_adjustment_check
        CHECK (token_delta IS NOT NULL OR balance_delta IS NOT NULL),
    CONSTRAINT admin_usage_calibrations_token_range_check
        CHECK (
            token_delta IS NULL
            OR (
                token_calculation_start_date IS NOT NULL
                AND token_calculation_end_date IS NOT NULL
                AND token_calculation_start_date <= token_calculation_end_date
            )
        )
);

CREATE INDEX IF NOT EXISTS idx_admin_usage_calibrations_target_user_created
    ON admin_usage_calibrations (target_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_usage_calibrations_admin_created
    ON admin_usage_calibrations (admin_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS admin_usage_calibration_daily_allocations (
    id BIGSERIAL PRIMARY KEY,
    calibration_id BIGINT NOT NULL REFERENCES admin_usage_calibrations(id) ON DELETE CASCADE,
    target_user_id BIGINT NOT NULL REFERENCES users(id),
    allocation_date DATE NOT NULL,
    original_tokens BIGINT NOT NULL DEFAULT 0,
    token_delta BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT admin_usage_calibration_daily_allocations_original_tokens_check
        CHECK (original_tokens >= 0),
    CONSTRAINT admin_usage_calibration_daily_allocations_unique_day
        UNIQUE (calibration_id, allocation_date)
);

CREATE INDEX IF NOT EXISTS idx_admin_usage_calibration_allocations_user_date
    ON admin_usage_calibration_daily_allocations (target_user_id, allocation_date);

CREATE INDEX IF NOT EXISTS idx_admin_usage_calibration_allocations_date
    ON admin_usage_calibration_daily_allocations (allocation_date);

COMMENT ON TABLE admin_usage_calibrations IS '管理员专用的隐藏 Token 与余额校准审计记录。';
COMMENT ON TABLE admin_usage_calibration_daily_allocations IS '仅用于汇总统计的 Token 校准按日分摊记录。';
