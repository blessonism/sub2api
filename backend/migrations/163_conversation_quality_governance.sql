-- OpenAI 对话归档 Phase 3：质量治理、脱敏导出与 raw archive 预留字段。
-- 本迁移只扩展结构化主路径，不启用 raw archive sink。

ALTER TABLE conversation_sessions
    ADD COLUMN IF NOT EXISTS quality_errors JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE conversation_turns
    ADD COLUMN IF NOT EXISTS quality_errors JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS raw_archive_key VARCHAR(512);
