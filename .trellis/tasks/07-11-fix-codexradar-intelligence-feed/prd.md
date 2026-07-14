# 修复 Codex 雷达智力监控数据源适配

## Goal

恢复用户渠道状态页中的 GPT 智力监控，适配 Codex 雷达当前公开数据格式。

## Requirements

- 后端改用 Codex 雷达公开的 `current.json` 结构化数据源，不再依赖首页 HTML 图表文本。
- 保持现有前端快照响应契约不变，包括主模型、近期记录、对比模型和额度雷达。
- 对缺少 `model_iq`、主记录或无有效序列的数据返回现有不可用错误。
- 保留现有一小时缓存、请求超时、禁用环境代理和来源署名行为。

## Acceptance Criteria

- [x] 能解析当前 `current.json` 的 `model_iq` 数据，并把对象形式的 comparisons 转为稳定数组。
- [x] 近期记录缺少模型与推理等级时，从对应 latest/series 元数据补齐。
- [x] 非 2xx、畸形 JSON 和缺少必要智力数据时继续返回 `GPT_INTELLIGENCE_UNAVAILABLE`。
- [x] 后端相关单元测试通过，前端无需调整即可消费响应。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
