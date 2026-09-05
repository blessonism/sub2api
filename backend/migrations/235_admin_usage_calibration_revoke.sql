ALTER TABLE admin_usage_calibrations
    ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS revoked_by BIGINT REFERENCES users(id);

CREATE INDEX IF NOT EXISTS idx_admin_usage_calibrations_active_target_created
    ON admin_usage_calibrations (target_user_id, created_at DESC)
    WHERE revoked_at IS NULL;
