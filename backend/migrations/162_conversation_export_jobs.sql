-- OpenAI 对话导出后台任务（Phase 2）
-- 导出任务独立于备份记录，只复用 S3/R2 对象存储配置。

CREATE TABLE IF NOT EXISTS conversation_export_jobs (
    id BIGSERIAL PRIMARY KEY,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    filters JSONB NOT NULL DEFAULT '{}'::jsonb,
    format VARCHAR(32) NOT NULL DEFAULT 'messages_jsonl',
    encoding VARCHAR(32) NOT NULL DEFAULT 'zstd',
    session_count BIGINT NOT NULL DEFAULT 0,
    turn_count BIGINT NOT NULL DEFAULT 0,
    file_size BIGINT NOT NULL DEFAULT 0,
    s3_key VARCHAR(512),
    download_url_expires_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    error_message VARCHAR(500),
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_conversation_export_jobs_status ON conversation_export_jobs(status);
CREATE INDEX IF NOT EXISTS idx_conversation_export_jobs_created_by ON conversation_export_jobs(created_by);
CREATE INDEX IF NOT EXISTS idx_conversation_export_jobs_created_at ON conversation_export_jobs(created_at);
CREATE INDEX IF NOT EXISTS idx_conversation_export_jobs_expires_at ON conversation_export_jobs(expires_at);

CREATE INDEX IF NOT EXISTS idx_conversation_turns_response_id
ON conversation_turns ((meta->>'response_id'))
WHERE meta ? 'response_id';
