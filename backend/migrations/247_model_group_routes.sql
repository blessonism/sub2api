-- 仅添加结构；生产路由通过管理员配置启用。
ALTER TABLE groups ADD COLUMN IF NOT EXISTS model_group_routes JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS source_group_id BIGINT;

-- 专用触发器保证直接 SQL 更新转发规则也会失效原组认证快照。
CREATE OR REPLACE FUNCTION enqueue_model_group_routes_auth_invalidation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys k
    WHERE k.group_id = NEW.id AND k.deleted_at IS NULL AND k.key <> '';
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS model_group_routes_auth_invalidation ON groups;
CREATE TRIGGER model_group_routes_auth_invalidation
AFTER UPDATE OF model_group_routes ON groups
FOR EACH ROW WHEN (OLD.model_group_routes IS DISTINCT FROM NEW.model_group_routes)
EXECUTE FUNCTION enqueue_model_group_routes_auth_invalidation();
