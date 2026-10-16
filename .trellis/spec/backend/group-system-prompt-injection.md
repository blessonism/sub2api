# 分组自定义 System Prompt 注入契约

## 1. Scope / Trigger

修改分组 system prompt 配置、认证分组快照、文本协议转换、HTTP/WS 转发、token 计数或独立图片 Responses 桥时，遵守本契约。首版仅支持原 API Key 绑定的 `openai` / `anthropic` 分组；`composite` 和其他平台不启用。

## 2. Signatures

- 现有管理员分组创建、更新、查询接口：`system_prompt_injection: { enabled: boolean, mode: "prepend" | "append" | "replace", content: string }`。
- `groups.system_prompt_injection`：JSONB NOT NULL，数据库默认 `{"enabled":false,"mode":"append","content":""}`。
- `domain.GroupSystemPromptInjection` / `service.GroupSystemPromptInjection`；更新输入使用指针，区分未提供与关闭。
- `WithGroupSystemPromptInjection(ctx, group)` 冻结原分组配置；`PrepareGroupSystemPromptBody` 清客户端指令；`InjectGroupSystemPromptBody` 向最终出站协议添加正文。

## 3. Contracts

- 管理端默认关闭，关闭保留正文、复制分组复制配置，更新省略字段不改变配置。标准模式和 simple mode 均保留该字段。只有 `AdminGroup` 返回配置，普通用户分组 DTO 不返回。
- 正文原样存储；空模式归一化为 `append`。配置经过 Ent、repository 分组映射和认证投影、auth snapshot 往返不得丢失。添加字段时提升 snapshot 版本；写入复用组失效及 durable invalidation outbox。
- `prepend` / `append` 只增加一份分组正文，保留原指令和结构。`replace` 在协议转换前删除客户端 `instructions`、`system`、消息型 `system/developer` 项；非消息 `additional_tools`、工具调用/结果及其他历史消息保留。
- 全局默认指令和协议必需身份继续生效。Claude OAuth 分组正文在 mimicry 后进入最终 `system`，不能被迁移到普通对话消息；真实 Claude Code 的固定身份也必须保留，身份块中的业务后缀不属于身份。
- HTTP 每次尝试及 WS 每轮都从未注入的 canonical body 派生。准备标记仅在当前调用链 context 传递，不能写回下一账号尝试的认证上下文。最终出口插入不按正文匹配去重。
- WS 配置冻结于连接建立时，后续轮次不换策略。`response.create` 每轮只注入一次，其他控制帧不新增正文；replace 模式须清理 `session.update` 带来的指令继承入口。
- 原始身份识别、会话哈希使用原请求；usage 指纹和推理档位使用本次尝试最终请求视图。token 计数复用同一策略，包括本地估算兜底。
- 独立图片请求即使复用 Responses builder，也通过 `isOpenAIImagesSelfBuiltRequest` 排除；普通 Responses 中的图片工具仍属于文本对话请求。

## 4. Validation & Error Matrix

| 输入 | 结果 |
| --- | --- |
| 缺省配置 | 关闭 / append / 空正文 |
| 更新未提供配置 | 保留已保存配置 |
| enabled 且正文仅空白 | 400 `INVALID_SYSTEM_PROMPT_INJECTION` |
| 未知 mode（含关闭状态） | 400 `INVALID_SYSTEM_PROMPT_INJECTION` |
| 其他平台 enabled | 400 `INVALID_SYSTEM_PROMPT_INJECTION` |
| 关闭，正文或模式仍有草稿 | 正常保存并保留 |
| 运行时关闭 | 原始字节直接返回 |
| 开启时相关请求字段结构非法 | 沿现有 HTTP/WS 错误路径返回，不能静默跳过注入 |

## 5. Good / Base / Bad Cases

- Good：OAuth 先生成身份/全局 system blocks，之后只追加一次分组正文；上游重试重新使用未注入 body。
- Base：历史分组迁移后关闭；simple mode 修改名称且省略配置时正文保持；已有 WS 用原快照，新连接用新配置。
- Bad：将分组正文放在 mimicry 之前导致它降为 user 消息；逐次修改 canonical body 导致重复；把所有带 `developer` 的对象都删除；公开 DTO 泄露管理员正文。

## 6. Tests Required

- 纯策略：三协议 × 三模式、有无原指令、多条指令、结构化块/cache metadata、Responses 单对象 input、工具数据保留、身份混合业务文本替换、重复正文不做匹配去重。
- 最终报文：API Key/OAuth/透传及双向协议桥；流式和非流式；换账号/同账号重试；两条 WS 实际多轮出站和控制帧；计数输入。
- 数据：CRUD/省略/关闭/复制/simple mode、真实数据库默认和 auth 查询投影、缓存 JSON 往返/旧版本拒绝/保存后失效、SQL durable 失效、普通 DTO 不返回配置。
- 前端：创建与编辑、三模式、关闭保留草稿、空白校验、切换平台、zh/en 对齐。Docker 不可用时，repository integration 必须记为未验证，不能把退出码 0 当成实际通过。

## 7. Wrong vs Correct

```go
// Wrong：正文先进入兼容转换，被移动成普通对话；重试又在旧 body 上追加。
body = inject(body)
body = rewriteSystemForNonClaudeCode(body, originalSystem)

// Correct：客户端替换清理与最终正文插入分阶段，重试保留 canonical 输入。
attempt := prepare(canonicalBody)
attempt = normalizeAndConvert(attempt)
wire := inject(attempt)
send(wire)
```
