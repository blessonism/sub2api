# 分组按模型转发

## 目标与已确认决策
用户保持原 OpenAI 余额分组 Key，按精确模型选择目标 OpenAI 余额分组。生产原组 51 不降智、目标组 40 luna；模型 gpt-5.6-luna / gpt-6-luna。生产发布单独确认。

## 要求
1. model_group_routes: model / target_group_id / mode(target_only|prefer_local) / enabled；默认空，省略更新不变，复制保留。
2. 两种模式逐条选择；prefer_local 仅发送前无合格本组候选时转发，失败不跨组重试；并发繁忙沿组内排队。
3. 实际组控制调度、价格、用户专属及分时倍率、利润门、RPM；原用户/Key 限额保持，仅扣费/计数一次，切组清原组 RPM 快照。
4. 两组白名单均生效，目标启用且用户可访问；禁止自指/重复模型/跨平台/订阅组；单跳。
5. HTTP 文本三入口及计数/别名、Responses WS 覆盖；System Prompt 仍冻结原组。
6. 模型列表/单模型/Codex manifest 合并，能力来自实际组；WS 需要换组时重连，计费组每轮刷新。
7. usage group_id 为实际组，source_group_id 可空保存原组；详情展示来源。

## 验收
- 后台增删改/模式/回显/复制和 zh/en 完整。
- 路由模式、不可用/权限/白名单/模型支持边界通过测试。
- HTTP/WS 模拟上游确认实际组、倍率、原 Key 限额、单次扣费和 RPM。
- 数据库持久化/认证投影/缓存版本与失效、Ent 二次生成、模型列表/manifest 验证。
- Go 定向与数据库集成测试、前端类型/lint/定向测试；不可用环境明确记录。

## 默认与边界
首期精确匹配、OpenAI standard、文本；默认 target_only。不硬编码生产 ID，不通过迁移改生产业务配置。保留其他未提交 IQ 探针工作。2026-10-26 用户已授权本地提交并合并到 custom/main；不 push、不部署生产。
