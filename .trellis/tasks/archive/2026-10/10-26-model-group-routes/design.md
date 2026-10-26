# 技术设计
- domain JSON 规则 + groups JSONB，贯穿 Ent/repository/service/admin DTO/auth snapshot/复制，提升 snapshot version。
- 共享解析，以原 Key/模型选有效组。HTTP auth 后、业务处理前解析；WS 首帧后解析。复制请求私有 Key/User，保留来源上下文并清原组 RPM override。
- prefer_local 复用 OpenAI 候选资格过滤，不占槽/写会话/发上游；有效组确定后冻结，调度/利润门/预留/扣费同源。
- 模型列表合并规则模型，权限与两组白名单过滤；Codex 引入模型用目标元数据。
- WS 后续轮次检查实际组不变，并刷新其价格；换组需要重连。原组提示词冻结。
- usage 新 source_group_id 可空，group_id 继续实际组统计。additive 迁移；关闭规则回滚行为。
