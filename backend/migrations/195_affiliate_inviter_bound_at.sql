-- 邀请关系需要独立的绑定时间，不能用返利资料创建时间代替。

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS inviter_bound_at TIMESTAMPTZ;

-- 旧数据没有精确绑定时间。使用最后更新时间作为保守上界，避免把活动开始后绑定的关系误算为历史邀请。
UPDATE user_affiliates
SET inviter_bound_at = updated_at
WHERE inviter_id IS NOT NULL
  AND inviter_bound_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_affiliates_inviter_bound_at
    ON user_affiliates (inviter_id, inviter_bound_at)
    WHERE inviter_id IS NOT NULL;
