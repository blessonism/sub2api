-- 恢复 groups 表高峰时段倍率列。
-- 192_group_time_rate_strategy.sql 在完成 peak_rate -> time_rate 数据迁移后无条件删除
-- peak_rate_* 列，但 ent schema 仍保留这些字段（创建分组时会显式写入默认值）。
-- 全新数据库按序执行迁移后会缺失这些列，导致 ent 创建分组失败；此处幂等恢复，
-- 对已执行过 192 的生产库同样安全（仅补回默认空列，不改变 time_rate 行为）。
ALTER TABLE groups ADD COLUMN IF NOT EXISTS peak_rate_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS peak_start VARCHAR(5) NOT NULL DEFAULT '';
ALTER TABLE groups ADD COLUMN IF NOT EXISTS peak_end VARCHAR(5) NOT NULL DEFAULT '';
ALTER TABLE groups ADD COLUMN IF NOT EXISTS peak_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0;
