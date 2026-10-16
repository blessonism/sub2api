-- 手动展示值与真实探测、用量及分组计费配置分开保存；空对象为全自动。
ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS display_overrides JSONB NOT NULL DEFAULT '{}';
