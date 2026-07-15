# 管理员配置 Codex Base Prompt 注入

## Goal

让管理员可以在管理后台控制适用的 OpenAI/Codex 请求是否自动注入服务端内置 Codex Base Prompt，并查看、编辑注入正文。

## Confirmed Facts

- 当前默认行为由 `backend/internal/service/openai_codex_transform.go:1224` 的 `applyInstructions` 控制：当 `instructions` 缺失、为空或仅含空白时注入默认 Prompt。
- 默认 Prompt 内容来自 `backend/internal/pkg/openai/instructions.txt`，非 Codex 模型还会按模型选择专用内嵌版本。
- 管理员设置已有完整的“布尔设置 → API DTO → 数据库存储 → 运行时缓存”链路，可复用 `enable_claude_oauth_system_prompt_injection` 的模式。
- 现有 `forced_codex_instructions_template_file` 是部署配置，不是管理员设置；本需求不改变其语义。
- 当前工作区存在其他未提交改动，实施时不得覆盖或回滚这些改动。

## Requirements

- 新增一个全局管理员设置，控制默认 Codex Base Prompt 是否注入；覆盖当前会自动补齐该 Prompt 的 OpenAI 请求，不限于 OAuth 账号。
- 管理员可以在同一设置区域查看并编辑一个全局自定义 Base Prompt 正文；非空自定义正文作为默认注入内容，清空后恢复内置按模型选择的 Prompt。
- 设置接口按运行时模型分组提供内置 Prompt 的只读正文，页面可选择 Codex、GPT-5.1、GPT-5.2 或最新 fallback 查看并载入为自定义正文；只读正文不参与保存。
- 设置默认值必须为启用，以保持现有行为不变。
- 关闭后，仅跳过服务端“默认 Base Prompt”补齐；客户端明确提供的 `instructions`、客户端 `system` 转换结果、强制模板和其他安全/能力提示不应被误删。
- 管理员可以在设置页面查看当前状态、切换并保存；保存后无需重启即可在运行时生效，允许沿用现有设置缓存延迟。
- 非管理员用户和现有设置接口的其他行为保持不变。

## Acceptance Criteria

- [ ] 管理员设置 API 返回并接受该开关，数据库持久化后刷新设置仍保留。
- [ ] 管理员设置 API 返回并接受自定义 Prompt 正文，数据库持久化后刷新设置仍保留。
- [ ] 管理员页面在网关转发设置区域显示清晰的中文/英文开关和说明。
- [ ] 管理员页面提供可滚动、多行编辑框，能区分“使用自定义 Prompt”和“清空后恢复内置 Prompt”。
- [ ] 管理员页面可查看内置 Codex Prompt 全文，并可一键载入编辑框作为自定义起点。
- [ ] 开启时，`instructions` 为空的适用 OAuth Codex 请求继续注入原有模型匹配的 Base Prompt。
- [ ] 开启且自定义正文非空时，适用请求注入自定义正文；清空自定义正文后恢复原有模型匹配逻辑。
- [ ] 关闭时，同类请求不再自动填充 Base Prompt；请求中已有的客户端 instructions 仍保留。
- [ ] 现有 Claude OAuth Prompt 开关、强制 Codex 模板和非相关请求链路不受影响。
- [ ] 添加后端运行时/转换测试及管理员设置页面测试，覆盖默认值、开关关闭和保存回显。

## Out Of Scope

- 不提供按用户、分组、模型或账号粒度的开关。
- 不提供按模型分别维护多份自定义 Prompt；一个全局自定义正文覆盖所有模型的默认注入内容。
- 不改变 Antigravity 或 Claude OAuth 专用 Prompt 注入逻辑。
