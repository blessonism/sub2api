# 分组自定义 System Prompt 注入

## 目标
管理员为 OpenAI/Codex、Claude 分组配置独立的 system prompt 注入，支持前置、追加、替换，默认关闭。

## 已确认需求
- 配置 system_prompt_injection: { enabled, mode, content }，mode 为 prepend/append/replace，默认 false/append/空串。
- 开启正文不得为空白；关闭保留正文；更新省略不修改；复制分组复制配置。
- 配置仅管理员 DTO 返回。其他平台、composite、独立音视频和图片接口不启用。
- 覆盖 Responses、Chat Completions、Messages、流式/非流式、Codex WebSocket、token 计数及协议桥接。
- 前置/追加保留客户端指令及结构；替换清除客户端 instructions、system 和 developer 指令，保留其他消息、工具定义及工具结果。
- 全局提示词及协议身份逻辑继续生效；分组正文保持最终 system 语义，不能被 Claude OAuth 降为普通对话消息。
- 从未注入的请求生成每次出站内容，重试、故障切换、WS 多轮不得叠加；不使用正文匹配去重。
- 使用 API Key 原绑定分组策略，HTTP 请求及 WS 连接内冻结；更新对新请求/新连接生效。
- 替换不重写上游已保存历史。用户已授权本地提交并合并到 custom/main；不含生产部署或 push。

## 验收
- CRUD、复制、数据库真实读写及认证投影/缓存保留配置；用户 DTO 不泄露。
- 三协议与三模式覆盖空/多条/结构化指令，替换不误删 additional_tools 等非消息项。
- 模拟上游验证 API Key/OAuth/透传/桥接的最终报文；流式与普通请求一致。
- 重试及 WS 每轮最多注入一次，token 计数包括分组正文。
- 前端创建/编辑共用配置组件，关闭保留正文、空白校验，zh/en 对齐。
- 定向单元/集成测试、前端类型检查、相关 lint/i18n 通过；环境缺失明确记录。
