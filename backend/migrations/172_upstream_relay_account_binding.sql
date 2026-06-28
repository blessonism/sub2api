-- 上游中继候选绑定从“上游分组 -> 本站分组”改为“上游分组 -> 项目账号”。
-- 已应用的历史迁移不得改写，因此在这里用增量迁移移除 target_group_id 语义。

DROP INDEX IF EXISTS uq_upstream_relay_candidate_account_group_active;
DROP INDEX IF EXISTS uq_upstream_relay_candidate_account_group;

-- 旧模型按“账号 + 本站分组”去重，切到账号维度后，同一账号可能出现多个等价活跃候选。
-- 保留最近更新的一条活跃候选，其余仅软删除，避免新唯一索引创建失败。
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY connector_id, account_id, upstream_group_id
               ORDER BY updated_at DESC, id DESC
           ) AS rn
    FROM upstream_relay_candidates
    WHERE deleted_at IS NULL
)
UPDATE upstream_relay_candidates c
SET deleted_at = NOW(),
    enabled = FALSE,
    updated_at = NOW()
FROM ranked r
WHERE c.id = r.id
  AND r.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_upstream_relay_candidate_connector_account_upstream_group_active
    ON upstream_relay_candidates (connector_id, account_id, upstream_group_id)
    WHERE deleted_at IS NULL;

ALTER TABLE upstream_relay_recommendation_suggestions
    DROP COLUMN IF EXISTS target_group_id;

ALTER TABLE upstream_relay_candidates
    DROP COLUMN IF EXISTS target_group_id;
