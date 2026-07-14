CREATE TABLE IF NOT EXISTS account_collections (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS account_collections_name_active_unique
    ON account_collections (LOWER(name)) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS account_collections_sort_order_idx
    ON account_collections (sort_order, id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS account_collection_members (
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    account_collection_id BIGINT NOT NULL REFERENCES account_collections(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, account_collection_id)
);

CREATE INDEX IF NOT EXISTS account_collection_members_collection_idx
    ON account_collection_members (account_collection_id, account_id);
