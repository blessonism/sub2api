ALTER TABLE token_usage_auto_runs
    DROP CONSTRAINT IF EXISTS chk_token_usage_auto_runs_run_type;

ALTER TABLE token_usage_auto_runs
    ADD CONSTRAINT chk_token_usage_auto_runs_run_type
    CHECK (run_type IN ('preview', 'manual', 'scheduled', 'clear'));
