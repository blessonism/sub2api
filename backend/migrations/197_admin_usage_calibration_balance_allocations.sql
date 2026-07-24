ALTER TABLE admin_usage_calibration_daily_allocations
    ADD COLUMN IF NOT EXISTS balance_delta NUMERIC(18, 6);

ALTER TABLE admin_usage_calibrations
    ADD COLUMN IF NOT EXISTS consumption_mode VARCHAR(20),
    ADD COLUMN IF NOT EXISTS consumption_input_value NUMERIC(18, 6),
    ADD COLUMN IF NOT EXISTS consumption_before_value NUMERIC(18, 6),
    ADD COLUMN IF NOT EXISTS consumption_after_value NUMERIC(18, 6),
    ADD COLUMN IF NOT EXISTS consumption_delta NUMERIC(18, 6),
    ADD COLUMN IF NOT EXISTS consumption_start_date DATE,
    ADD COLUMN IF NOT EXISTS consumption_end_date DATE,
    ADD COLUMN IF NOT EXISTS consumption_timezone TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'admin_usage_calibrations_consumption_mode_check'
    ) THEN
        ALTER TABLE admin_usage_calibrations
            ADD CONSTRAINT admin_usage_calibrations_consumption_mode_check
            CHECK (consumption_mode IS NULL OR consumption_mode IN ('delta', 'target'));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'admin_usage_calibrations_consumption_range_check'
    ) THEN
        ALTER TABLE admin_usage_calibrations
            ADD CONSTRAINT admin_usage_calibrations_consumption_range_check
            CHECK (
                consumption_delta IS NULL OR (
                    consumption_start_date IS NOT NULL
                    AND consumption_end_date IS NOT NULL
                    AND consumption_start_date <= consumption_end_date
                )
            );
    END IF;
END $$;

-- 补齐旧组合校准中因 Token 差额舍入为 0 而未落表的原始用量日期。
INSERT INTO admin_usage_calibration_daily_allocations (
    calibration_id,
    target_user_id,
    allocation_date,
    original_tokens,
    token_delta
)
SELECT
    c.id,
    c.target_user_id,
    (u.created_at AT TIME ZONE COALESCE(NULLIF(c.token_calculation_timezone, ''), 'UTC'))::date,
    SUM(u.input_tokens + u.output_tokens + u.cache_creation_tokens + u.cache_read_tokens),
    0
FROM admin_usage_calibrations c
JOIN usage_logs u
  ON u.user_id = c.target_user_id
 AND u.created_at >= (c.token_calculation_start_date::timestamp AT TIME ZONE COALESCE(NULLIF(c.token_calculation_timezone, ''), 'UTC'))
 AND u.created_at < ((c.token_calculation_end_date + 1)::timestamp AT TIME ZONE COALESCE(NULLIF(c.token_calculation_timezone, ''), 'UTC'))
WHERE c.balance_delta IS NOT NULL
  AND c.token_calculation_start_date IS NOT NULL
  AND c.token_calculation_end_date IS NOT NULL
GROUP BY c.id, c.target_user_id, 3
HAVING SUM(u.input_tokens + u.output_tokens + u.cache_creation_tokens + u.cache_read_tokens) > 0
ON CONFLICT (calibration_id, allocation_date) DO NOTHING;

WITH weights AS (
    SELECT
        a.id,
        c.id AS calibration_id,
        c.balance_delta,
        a.allocation_date,
        a.original_tokens,
        SUM(a.original_tokens) OVER (PARTITION BY c.id) AS total_tokens,
        ROUND(ABS(c.balance_delta) * 1000000, 0) AS total_units
    FROM admin_usage_calibration_daily_allocations a
    JOIN admin_usage_calibrations c ON c.id = a.calibration_id
    WHERE c.balance_delta IS NOT NULL
      AND a.balance_delta IS NULL
), fractions AS (
    SELECT
        *,
        FLOOR(total_units * original_tokens / total_tokens) AS base_units,
        MOD(total_units * original_tokens, total_tokens) AS remainder
    FROM weights
    WHERE total_tokens > 0
), ranked AS (
    SELECT
        *,
        SUM(base_units) OVER (PARTITION BY calibration_id) AS allocated_units,
        ROW_NUMBER() OVER (
            PARTITION BY calibration_id
            ORDER BY remainder DESC, original_tokens DESC, allocation_date ASC, id ASC
        ) AS remainder_rank
    FROM fractions
)
UPDATE admin_usage_calibration_daily_allocations a
SET balance_delta = SIGN(r.balance_delta) * (
    r.base_units + CASE
        WHEN r.remainder_rank <= r.total_units - r.allocated_units THEN 1
        ELSE 0
    END
) / 1000000
FROM ranked r
WHERE a.id = r.id;

COMMENT ON COLUMN admin_usage_calibration_daily_allocations.balance_delta
    IS '余额校准按原始 Token 用量占比分摊到该日的差额；NULL 表示该记录不承载余额归属。';
