-- 可选的展示统计分组；存量监控不按名称自动关联。
ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL;
