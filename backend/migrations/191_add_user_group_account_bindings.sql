CREATE TABLE IF NOT EXISTS user_group_account_bindings (
    user_id            BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id           BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    account_ids        BIGINT[] NOT NULL,
    fallback_to_group  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, group_id),
    CONSTRAINT user_group_account_bindings_non_empty CHECK (cardinality(account_ids) > 0)
);

CREATE INDEX IF NOT EXISTS idx_user_group_account_bindings_group_id
    ON user_group_account_bindings(group_id);

COMMENT ON TABLE user_group_account_bindings IS '用户在公开分组中的账号调度限制';
COMMENT ON COLUMN user_group_account_bindings.account_ids IS '允许调度的账号 ID 列表';
COMMENT ON COLUMN user_group_account_bindings.fallback_to_group IS '绑定账号无法立即承接时是否回退到分组其他账号';
