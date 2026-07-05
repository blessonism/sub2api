ALTER TABLE upstream_relay_candidates
    DROP CONSTRAINT IF EXISTS chk_upstream_relay_candidate_protocol;

ALTER TABLE upstream_relay_candidates
    ADD CONSTRAINT chk_upstream_relay_candidate_protocol
    CHECK (probe_protocol IN ('chat_completions', 'responses', 'anthropic'));
