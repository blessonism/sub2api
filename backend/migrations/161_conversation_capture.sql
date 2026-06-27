-- OpenAI 对话历史结构化采集（Phase 1）
-- 默认只保存可查询的 session / turn，不实现 raw archive 或后台导出任务。

CREATE TABLE IF NOT EXISTS conversation_sessions (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(128) NOT NULL UNIQUE,
    trajectory_id VARCHAR(128),
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    account_id BIGINT,
    provider VARCHAR(32) NOT NULL DEFAULT 'openai',
    model VARCHAR(200) NOT NULL,
    request_path VARCHAR(200) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    turn_count INTEGER NOT NULL DEFAULT 0,
    source_request_count INTEGER NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    quality_status VARCHAR(32) NOT NULL DEFAULT 'unchecked',
    exportable BOOLEAN NOT NULL DEFAULT FALSE,
    capture_status VARCHAR(32) NOT NULL DEFAULT 'captured',
    session_source VARCHAR(32) NOT NULL DEFAULT 'single_turn',
    retention_until TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_sessions_session_id ON conversation_sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_user_id ON conversation_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_api_key_id ON conversation_sessions(api_key_id);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_model ON conversation_sessions(model);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_quality_status ON conversation_sessions(quality_status);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_exportable ON conversation_sessions(exportable);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_started_at ON conversation_sessions(started_at);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_created_at ON conversation_sessions(created_at);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_user_started ON conversation_sessions(user_id, started_at);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_api_key_started ON conversation_sessions(api_key_id, started_at);

CREATE TABLE IF NOT EXISTS conversation_turns (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(128) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    upstream_request_id VARCHAR(128),
    client_request_id VARCHAR(128),
    turn_index INTEGER NOT NULL,
    provider VARCHAR(32) NOT NULL DEFAULT 'openai',
    model VARCHAR(200) NOT NULL,
    request_path VARCHAR(200) NOT NULL,
    request_messages JSONB NOT NULL DEFAULT '[]'::jsonb,
    response_messages JSONB NOT NULL DEFAULT '[]'::jsonb,
    tools JSONB NOT NULL DEFAULT '[]'::jsonb,
    usage JSONB NOT NULL DEFAULT '{}'::jsonb,
    meta JSONB NOT NULL DEFAULT '{}'::jsonb,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    stream BOOLEAN NOT NULL DEFAULT FALSE,
    client_disconnect BOOLEAN NOT NULL DEFAULT FALSE,
    truncated BOOLEAN NOT NULL DEFAULT FALSE,
    quality_status VARCHAR(32) NOT NULL DEFAULT 'unchecked',
    exportable BOOLEAN NOT NULL DEFAULT FALSE,
    parse_status VARCHAR(32) NOT NULL DEFAULT 'failed',
    parse_error VARCHAR(500),
    dedupe_hash VARCHAR(128) NOT NULL,
    payload_preview TEXT,
    payload_compressed BYTEA,
    retention_until TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT conversation_turns_session_turn_unique UNIQUE (session_id, turn_index)
);

CREATE INDEX IF NOT EXISTS idx_conversation_turns_session_id ON conversation_turns(session_id);
CREATE INDEX IF NOT EXISTS idx_conversation_turns_request_id ON conversation_turns(request_id);
CREATE INDEX IF NOT EXISTS idx_conversation_turns_client_request_id ON conversation_turns(client_request_id);
CREATE INDEX IF NOT EXISTS idx_conversation_turns_dedupe_hash ON conversation_turns(dedupe_hash);
CREATE INDEX IF NOT EXISTS idx_conversation_turns_created_at ON conversation_turns(created_at);
CREATE INDEX IF NOT EXISTS idx_conversation_turns_session_turn ON conversation_turns(session_id, turn_index);
