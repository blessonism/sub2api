-- 智商探针结果表：糖果问题等固定题目对（账号, 模型）目标的探测结果。
-- 保留 30 天，由 OpsCleanupService 每日物理删。
CREATE TABLE IF NOT EXISTS model_iq_probe_results (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT,
    account_name TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    protocol TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    override_status TEXT,
    error_category TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    expected_answer TEXT NOT NULL,
    actual_answer TEXT,
    latency_ms BIGINT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cache_read_tokens INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_iq_probe_results_model_created
    ON model_iq_probe_results (model, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_iq_probe_results_account_created
    ON model_iq_probe_results (account_id, created_at DESC);
