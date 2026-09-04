ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS openai_scheduler_overrides JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.openai_scheduler_overrides IS
    'OpenAI/Codex 分组级调度覆盖：lb_top_k、TTFT/错误率/负载权重、ttft_max_ratio、sticky 逃逸阈值；空对象表示全部继承全局';
