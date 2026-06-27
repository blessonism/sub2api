-- 164 已在本地库应用后又继续演进过；后续增量必须放到新迁移，避免改写已应用迁移。

ALTER TABLE upstream_relay_connectors
    ADD COLUMN IF NOT EXISTS refresh_token_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS login_email_encrypted TEXT;

-- 候选通道使用软删除后，同一个账号/目标分组应允许重新创建一条新的未删除记录。
ALTER TABLE upstream_relay_candidates
    DROP CONSTRAINT IF EXISTS uq_upstream_relay_candidate_account_group;

DROP INDEX IF EXISTS uq_upstream_relay_candidate_account_group;

CREATE UNIQUE INDEX IF NOT EXISTS uq_upstream_relay_candidate_account_group_active
    ON upstream_relay_candidates (account_id, target_group_id)
    WHERE deleted_at IS NULL;
