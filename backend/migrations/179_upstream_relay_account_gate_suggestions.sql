-- 上游中继账号流量闸门：让推荐记录可承载 priority 调整、账号暂停和账号恢复。

ALTER TABLE upstream_relay_recommendation_suggestions
    ADD COLUMN IF NOT EXISTS action_type VARCHAR(40) NOT NULL DEFAULT 'priority_update',
    ADD COLUMN IF NOT EXISTS old_schedulable BOOLEAN,
    ADD COLUMN IF NOT EXISTS new_schedulable BOOLEAN;

ALTER TABLE upstream_relay_recommendation_suggestions
    ALTER COLUMN new_priority DROP NOT NULL;

ALTER TABLE upstream_relay_recommendation_suggestions
    DROP CONSTRAINT IF EXISTS chk_upstream_relay_recommendation_suggestion_action_type,
    ADD CONSTRAINT chk_upstream_relay_recommendation_suggestion_action_type CHECK (
        action_type IN ('priority_update', 'account_pause', 'account_resume')
    );

ALTER TABLE upstream_relay_recommendation_suggestions
    DROP CONSTRAINT IF EXISTS chk_upstream_relay_recommendation_suggestion_action_payload,
    ADD CONSTRAINT chk_upstream_relay_recommendation_suggestion_action_payload CHECK (
        (
            action_type = 'priority_update'
            AND new_priority IS NOT NULL
            AND old_schedulable IS NULL
            AND new_schedulable IS NULL
        )
	        OR (
	            action_type = 'account_pause'
	            AND new_priority IS NULL
	            AND old_schedulable = TRUE
	            AND new_schedulable = FALSE
	        )
	        OR (
	            action_type = 'account_resume'
	            AND new_priority IS NULL
	            AND old_schedulable = FALSE
	            AND new_schedulable = TRUE
	        )
	    );

CREATE TABLE IF NOT EXISTS upstream_relay_account_gate_states (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    original_schedulable BOOLEAN NOT NULL DEFAULT TRUE,
    pause_reason_code VARCHAR(80) NOT NULL DEFAULT '',
    pause_reason TEXT NOT NULL DEFAULT '',
    source_run_id BIGINT REFERENCES upstream_relay_recommendation_runs(id) ON DELETE SET NULL,
    source_suggestion_id BIGINT REFERENCES upstream_relay_recommendation_suggestions(id) ON DELETE SET NULL,
    paused_by BIGINT,
    paused_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    restored_by BIGINT,
    restored_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upstream_relay_account_gate_states_active
    ON upstream_relay_account_gate_states (active, paused_at DESC);
