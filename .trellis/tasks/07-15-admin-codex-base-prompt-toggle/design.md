# 技术设计

## 边界与数据流

管理员页面表单 → 管理员设置更新 DTO → 设置服务写入 `settings` → 运行时设置缓存 → OpenAI 请求转发/ OAuth 转换 → `reqBody.instructions`。

复用现有 Gateway forwarding 设置的全链路，不新增表或迁移。新增两个全局设置键：

- `enable_codex_base_prompt_injection`：默认 `true`。
- `codex_base_prompt`：默认空字符串；空值表示使用现有内嵌 Prompt。

## 运行时行为

在 OpenAI 请求转发补齐 `instructions` 前读取设置服务缓存；OAuth 转换调用方同时将开关映射为 `SkipDefaultInstructions`，避免关闭后由后续转换再次补齐。转换函数继续保持纯函数；其默认 Prompt 选择函数增加可选的自定义正文参数，非空时直接使用自定义正文，空值时保持现有按模型选择逻辑。

仅影响“instructions 为空时的默认补齐”。已有客户端 `instructions`、messages 转换出的 instructions、强制模板和 Spark 能力提示不被清理。默认关闭开关时不填充默认正文，但仍保留 `ensureCodexOAuthInstructionsField` 对字段形状的兼容处理。

## 设置缓存与失败默认

扩展现有 `cachedGatewayForwardingSettings`，沿用 60 秒 TTL、singleflight 和数据库失败时的安全默认值：启用 Base Prompt 注入，自定义正文为空。这样升级后行为与当前版本一致。

## 管理员界面

在现有 Gateway forwarding 区域复用 `Toggle` 和文本域样式，增加开关与多行编辑框。设置响应按运行时模型选择分组返回只读的内置 Prompt，页面提供模型选择、全文查看和“载入内置正文”操作；该只读字段不进入更新请求。开关关闭时编辑框仍可查看和编辑，避免切换开关丢失草稿；保存时 trim 自定义正文，空值恢复内置 Prompt。中英文文案各增加标签、说明和恢复内置提示。

## 兼容与风险

- 旧数据库没有新键时使用默认值，不需要迁移。
- 自定义正文是管理员配置，按现有设置接口权限保护；不写入请求日志。
- 单一全局正文覆盖所有模型，避免在本次功能中复制四份 Prompt 编辑状态。
