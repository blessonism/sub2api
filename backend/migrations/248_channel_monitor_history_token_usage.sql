-- 渠道监控历史增加探测 token 消耗列。
-- usage 来自响应体（OpenAI/Anthropic/Gemini 的 usage 字段，含缓存命中）；
-- 上游未返回 usage 或流在终态前被截断（如超限、超时）时保持 NULL。
ALTER TABLE channel_monitor_histories
    ADD COLUMN IF NOT EXISTS input_tokens INT,
    ADD COLUMN IF NOT EXISTS output_tokens INT,
    ADD COLUMN IF NOT EXISTS cache_read_tokens INT;
