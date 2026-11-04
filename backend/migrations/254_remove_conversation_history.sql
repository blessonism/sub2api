-- 对话历史模块退役。执行前完成专属数据与 conversation-exports/ 对象备份。
-- 保留 161–163 历史迁移，使用默认 RESTRICT 阻止未知依赖被级联删除。
-- 迁移执行器负责将以下操作及迁移记录放入同一事务。

DROP TABLE IF EXISTS conversation_export_jobs;
DROP TABLE IF EXISTS conversation_turns;
DROP TABLE IF EXISTS conversation_sessions;

DELETE FROM settings WHERE key = 'conversation_capture_config';
