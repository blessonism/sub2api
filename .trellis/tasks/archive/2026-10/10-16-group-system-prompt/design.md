# 技术设计

## 配置链路
使用 domain.GroupSystemPromptInjection 作为 JSONB/服务配置类型，service 提供别名。接通 Ent schema/生成文件、repository（含认证字段投影）、admin CRUD/duplicate、DTO、auth snapshot（版本提升与现有失效链路）。管理接口字段为 system_prompt_injection，更新使用指针区分省略。仅 openai/anthropic 允许 enabled=true。

## 请求链路
公共策略模块负责按客户端格式清理替换指令，以及按最终出站协议插入分组正文。请求绑定原 API Key 分组的不可变配置快照，保留未注入的 canonical body。账号级转换与重试各自生成独立 body；协议转换不再次注入。原客户端身份检测和会话哈希使用原始内容。Claude OAuth 先清除替换目标，之后继续身份兼容处理，分组正文在最终 system 组装、缓存约束/签名前加入。关闭时完全走原逻辑。

Responses 原生 instructions 与 input 中的 system/developer 消息共同组成系统指令序列；替换仅清理消息项，保留 additional_tools 等非消息数据。Chat Completions 在系统/developer 消息序列首尾插入独立 system 文本消息。Anthropic 保留 system 字符串/内容块；新正文无客户端缓存断点，身份前缀固定位置。

WS 两条转发路径覆盖首轮与后续 response.create，保留未注入帧用于重试，不向其他控制帧注入。计数路径共享策略。所有内部注入状态只放内存，不写入上游 JSON。

## 默认和兼容
配置关闭时保持现有行为；分组启用与全局设置独立，replace 仅清客户端指令。正文按原样存储，TrimSpace 只用于空白校验。不新增生产操作。数据库迁移只添加默认关闭的 JSONB 字段，旧分组无行为变化。
