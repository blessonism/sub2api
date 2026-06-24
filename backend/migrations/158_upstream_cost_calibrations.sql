CREATE TABLE IF NOT EXISTS upstream_cost_calibration_tasks (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    target_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    model VARCHAR(120) NOT NULL,
    adapter_type VARCHAR(40) NOT NULL DEFAULT 'manual',
    unit VARCHAR(32) NOT NULL DEFAULT 'credit',
    test_prompt TEXT NOT NULL DEFAULT 'ping',
    sample_count INTEGER NOT NULL DEFAULT 1,
    priority_start INTEGER NOT NULL DEFAULT 10,
    priority_step INTEGER NOT NULL DEFAULT 10,
    last_run_id BIGINT,
    last_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_upstream_cost_calibration_sample_count CHECK (sample_count BETWEEN 1 AND 10),
    CONSTRAINT chk_upstream_cost_calibration_priority_start CHECK (priority_start BETWEEN 1 AND 10000),
    CONSTRAINT chk_upstream_cost_calibration_priority_step CHECK (priority_step BETWEEN 1 AND 10000),
    CONSTRAINT chk_upstream_cost_calibration_adapter_type CHECK (adapter_type IN ('manual'))
);

CREATE TABLE IF NOT EXISTS upstream_cost_calibration_task_accounts (
    task_id BIGINT NOT NULL REFERENCES upstream_cost_calibration_tasks(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    adapter_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (task_id, account_id)
);

CREATE TABLE IF NOT EXISTS upstream_cost_calibration_runs (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES upstream_cost_calibration_tasks(id) ON DELETE CASCADE,
    target_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    total_accounts INTEGER NOT NULL DEFAULT 0,
    valid_accounts INTEGER NOT NULL DEFAULT 0,
    invalid_accounts INTEGER NOT NULL DEFAULT 0,
    suggestion_count INTEGER NOT NULL DEFAULT 0,
    applied BOOLEAN NOT NULL DEFAULT FALSE,
    applied_by BIGINT,
    applied_at TIMESTAMPTZ,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_upstream_cost_calibration_run_status CHECK (status IN ('running', 'success', 'failed'))
);

CREATE TABLE IF NOT EXISTS upstream_cost_calibration_results (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES upstream_cost_calibration_runs(id) ON DELETE CASCADE,
    task_id BIGINT NOT NULL REFERENCES upstream_cost_calibration_tasks(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    account_name VARCHAR(100) NOT NULL DEFAULT '',
    account_platform VARCHAR(50) NOT NULL DEFAULT '',
    current_priority INTEGER,
    before_balance NUMERIC(20, 8),
    after_balance NUMERIC(20, 8),
    cost_delta NUMERIC(20, 8),
    unit VARCHAR(32) NOT NULL DEFAULT 'credit',
    test_status VARCHAR(20) NOT NULL DEFAULT 'success',
    latency_ms INTEGER,
    valid BOOLEAN NOT NULL DEFAULT FALSE,
    rank INTEGER,
    suggested_priority INTEGER,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_upstream_cost_calibration_result_run_account UNIQUE (run_id, account_id),
    CONSTRAINT chk_upstream_cost_calibration_result_status CHECK (test_status IN ('success', 'failed', 'skipped'))
);

CREATE TABLE IF NOT EXISTS upstream_cost_calibration_suggestions (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES upstream_cost_calibration_runs(id) ON DELETE CASCADE,
    task_id BIGINT NOT NULL REFERENCES upstream_cost_calibration_tasks(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    old_priority INTEGER,
    new_priority INTEGER NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    applied BOOLEAN NOT NULL DEFAULT FALSE,
    applied_by BIGINT,
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_upstream_cost_calibration_suggestion_run_account UNIQUE (run_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_upstream_cost_calibration_tasks_group
    ON upstream_cost_calibration_tasks (target_group_id);

CREATE INDEX IF NOT EXISTS idx_upstream_cost_calibration_runs_task_created
    ON upstream_cost_calibration_runs (task_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_upstream_cost_calibration_results_run_rank
    ON upstream_cost_calibration_results (run_id, rank);

CREATE INDEX IF NOT EXISTS idx_upstream_cost_calibration_suggestions_run
    ON upstream_cost_calibration_suggestions (run_id, applied);
