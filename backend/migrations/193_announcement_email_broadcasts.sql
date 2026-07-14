-- 公告邮件群发任务与逐用户投递审计。

CREATE TABLE IF NOT EXISTS announcement_email_broadcasts (
    id BIGSERIAL PRIMARY KEY,
    announcement_id BIGINT NOT NULL UNIQUE REFERENCES announcements(id) ON DELETE RESTRICT,
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    total_count INTEGER NOT NULL DEFAULT 0,
    sent_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    CONSTRAINT announcement_email_broadcasts_status_valid
        CHECK (status IN ('pending', 'running', 'completed', 'partial_failed')),
    CONSTRAINT announcement_email_broadcasts_counts_valid
        CHECK (total_count >= 0 AND sent_count >= 0 AND failed_count >= 0 AND sent_count + failed_count <= total_count)
);

CREATE INDEX IF NOT EXISTS announcement_email_broadcasts_status_idx
    ON announcement_email_broadcasts(status);

CREATE TABLE IF NOT EXISTS announcement_email_deliveries (
    id BIGSERIAL PRIMARY KEY,
    broadcast_id BIGINT NOT NULL REFERENCES announcement_email_broadcasts(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    email VARCHAR(320) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    lease_expires_at TIMESTAMPTZ,
    error_message VARCHAR(500),
    last_attempt_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT announcement_email_deliveries_status_valid
        CHECK (status IN ('pending', 'processing', 'sent', 'failed')),
    CONSTRAINT announcement_email_deliveries_attempts_valid CHECK (attempt_count >= 0),
    CONSTRAINT announcement_email_deliveries_broadcast_user_unique UNIQUE (broadcast_id, user_id)
);

CREATE INDEX IF NOT EXISTS announcement_email_deliveries_claim_idx
    ON announcement_email_deliveries(status, lease_expires_at, id);
CREATE INDEX IF NOT EXISTS announcement_email_deliveries_broadcast_status_idx
    ON announcement_email_deliveries(broadcast_id, status, id);
CREATE INDEX IF NOT EXISTS announcement_email_deliveries_email_idx
    ON announcement_email_deliveries(broadcast_id, email);
