-- 上游中转连接器：记录最近一次同步到的远端登录账号余额快照。

ALTER TABLE upstream_relay_connectors
    ADD COLUMN IF NOT EXISTS upstream_account_balance NUMERIC(20, 8),
    ADD COLUMN IF NOT EXISTS upstream_account_balance_checked_at TIMESTAMPTZ;
