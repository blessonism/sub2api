-- 上游中继监控自动推荐与自动应用配置。

ALTER TABLE upstream_relay_monitoring_policy
    ADD COLUMN IF NOT EXISTS auto_recommendation_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS recommendation_interval_minutes INTEGER NOT NULL DEFAULT 60,
    ADD COLUMN IF NOT EXISTS auto_apply_recommendations_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS max_auto_apply_suggestions INTEGER NOT NULL DEFAULT 20,
    ADD COLUMN IF NOT EXISTS max_auto_apply_priority_delta INTEGER NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS min_auto_apply_confidence VARCHAR(20) NOT NULL DEFAULT 'medium',
    ADD COLUMN IF NOT EXISTS allow_auto_apply_degraded_health BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE upstream_relay_monitoring_policy
    DROP CONSTRAINT IF EXISTS chk_upstream_relay_monitoring_policy_recommendation_interval,
    ADD CONSTRAINT chk_upstream_relay_monitoring_policy_recommendation_interval CHECK (
        recommendation_interval_minutes > 0
    );

ALTER TABLE upstream_relay_monitoring_policy
    DROP CONSTRAINT IF EXISTS chk_upstream_relay_monitoring_policy_auto_apply_limits,
    ADD CONSTRAINT chk_upstream_relay_monitoring_policy_auto_apply_limits CHECK (
        max_auto_apply_suggestions > 0
        AND max_auto_apply_priority_delta >= 0
    );

ALTER TABLE upstream_relay_monitoring_policy
    DROP CONSTRAINT IF EXISTS chk_upstream_relay_monitoring_policy_auto_apply_confidence,
    ADD CONSTRAINT chk_upstream_relay_monitoring_policy_auto_apply_confidence CHECK (
        min_auto_apply_confidence IN ('high', 'medium', 'low', 'unknown')
    );
