# 实现步骤
1. domain/schema/CRUD/auth snapshot/复制、迁移与 Ent。
2. 共享解析、HTTP/WS、候选资格、计费/source usage 字段。
3. 普通模型/Codex 合并。
4. 后台规则组件、类型/zh/en、usage 详情。
5. 路由/计费/数据库/缓存/WS 与前端定向测试；typecheck/lint/Ent 稳定性。
6. spec 和验证记录；按 2026-10-26 用户授权提交并合并到本地 custom/main，完成本任务归档和会话记录；不 push、不部署。

## 第二轮 review 修正
- 模型目录路由过滤后直接返回，避免白名单空集重新回退默认 Luna。
- WS 每轮识别新启用规则；只替换实际组快照，保留建连 Key 额度和用户限制。
- prefer_local 使用全局 quota auto-pause 与模型级冷却资格。
- 来源固定目录无可用账号时，普通列表和 Codex 仍显示有效目标规则模型。
- 每项先用回归测试复现失败，再修正并运行相关网关回归。
