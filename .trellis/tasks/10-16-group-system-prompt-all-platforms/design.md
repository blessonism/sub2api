# 技术方案

移除 OpenAI/Anthropic 两平台白名单，后端使用 domain.IsGroupPlatform，前端使用平台登记清单和 composite。数据库字段、DTO 和缓存结构不变，无新增迁移。

共享策略增加 Gemini systemInstruction.parts 适配，保留 Content/Part 额外字段与工具/缓存信息，支持 system_instruction 与 countTokens generateContentRequest 包裹。replace 在入口协议转换前处理，正文在最终系统指令组装后加入。Gemini 和 Antigravity 的既有身份放在前面，避免新策略清除或移动。Antigravity countTokens 原本固定 0；开启策略时改为对最终含身份和分组正文的请求使用已有 Gemini 估算器，关闭继续原行为。

复用已有 OpenAI/Claude 注入出口覆盖所有同族供应商；单独补齐 Grok 提前分流和 Gemini/Antigravity 独立网关，确保每次尝试从基础报文产生，不在重试时再次追加。组合分组沿鉴权冻结原分组配置，目标平台不覆盖。

TypeSafe / Jev 只接受 System One：逐问题对 questions.<id>.instructions 应用模式，字符串直接拼接，对象与数组保留结构；replace 只替换该指令区，不改 state、criteria 或问题类型。前端增加专用说明。

所有独立出口逐项复核：Grok Messages 的首次与 encrypted-content 重试、Grok WS HTTP bridge、Antigravity 模型 fallback 均绕过常规 builder，分别在最终构建前注入。Gemini 签名修复重新从 canonical Claude body 转换；countTokens 包裹请求先取内部 GenerateContentRequest 再加身份与正文估算。

测试以捕获模拟上游出站为核心，覆盖三模式、工具保留、内部重试和跨协议。前端全平台开关/保存行为和中英文说明同步更新。保持独立图片/音视频接口的既有范围。
