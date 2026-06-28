-- 上游中继监控全局配置：只负责采集频率和手动批量操作并发上限，不自动应用 priority。

CREATE TABLE IF NOT EXISTS upstream_relay_monitoring_policy (
    id SMALLINT PRIMARY KEY DEFAULT 1,
    auto_sync_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    sync_interval_minutes INTEGER NOT NULL DEFAULT 480,
    auto_probe_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    probe_interval_minutes INTEGER NOT NULL DEFAULT 10,
    failure_retry_interval_minutes INTEGER NOT NULL DEFAULT 15,
    sync_concurrency INTEGER NOT NULL DEFAULT 2,
    probe_concurrency INTEGER NOT NULL DEFAULT 5,
    updated_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_upstream_relay_monitoring_policy_singleton CHECK (id = 1),
    CONSTRAINT chk_upstream_relay_monitoring_policy_intervals CHECK (
        sync_interval_minutes > 0
        AND probe_interval_minutes > 0
        AND failure_retry_interval_minutes > 0
    ),
    CONSTRAINT chk_upstream_relay_monitoring_policy_concurrency CHECK (
        sync_concurrency > 0
        AND probe_concurrency > 0
    )
);
