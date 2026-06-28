-- 上游中继候选映射显式绑定上游 API Key。
-- 上游 key 可能脱敏，不能稳定通过本地账号 key/name 反查，因此绑定关系归属于候选映射。

ALTER TABLE upstream_relay_candidates
    ADD COLUMN IF NOT EXISTS upstream_api_key_id BIGINT,
    ADD COLUMN IF NOT EXISTS upstream_api_key_name VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS upstream_api_key_masked VARCHAR(255) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_upstream_relay_candidates_upstream_api_key
    ON upstream_relay_candidates (connector_id, upstream_api_key_id)
    WHERE deleted_at IS NULL AND upstream_api_key_id IS NOT NULL;
