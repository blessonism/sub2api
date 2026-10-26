# 分组按模型转发契约

## 接口与持久化

- 管理员分组 CRUD：`model_group_routes: [{model, target_group_id, mode, enabled}]`，更新省略时保持，`[]` 清空。普通用户分组 DTO 不暴露规则。
- `mode` 为 `target_only`（默认）或 `prefer_local`；精确匹配客户端模型名，不支持通配符，同一模型只允许一条规则。禁止自指，仅 OpenAI standard 分组之间允许配置。
- `groups.model_group_routes` JSONB NOT NULL DEFAULT `[]`；migration `247_model_group_routes.sql` 不包含任何生产分组 ID 或业务启用逻辑。
- 规则须贯穿 Ent、repo、admin DTO、复制与 auth snapshot（版本 26）。专用 SQL trigger 将规则变更写入 `auth_cache_invalidation_outbox`；没有变化不发事件。

## 请求与计费

- HTTP 在原组白名单之后、handler/定价/额度预留/限流/选号之前调用共享 resolver；覆盖根路径别名与 Codex 直连路径。WS 在首帧读取模型后解析。
- `target_only` 直接进入目标组；`prefer_local` 按本组调度资格、模型、协议、隐私、账号绑定及利润门检查候选。探查须注入真实调度器使用的全局 quota auto-pause 设置，并检查 `IsSchedulableForModelWithContext`，不能把已达暂停阈值或模型冷却中的账号算作候选。不占并发、不写 sticky、不发上游；有候选但并发繁忙仍在本组排队。
- 决定分组后仅组内重试；单跳，不递归目标组规则。目标不可用、无权限、白名单不允许时明确拒绝。
- 使用请求私有 API Key/User 副本，保持原 Key ID、额度、用户身份。切组清空 `UserGroupRPMOverride`，重新读取目标 user×group 的 RPM；不得修改共享认证对象。
- 价格、专属倍率、分时倍率、利润门和 RPM 由实际组负责，单次扣费/计数。System Prompt 继续在认证时冻结原绑定组，转发组不能覆盖正文。
- HTTP/WS 的多个候选模型不得产生不同路由，冲突模型值应拒绝。

## 目录、WS 与统计

- 每次模型列表请求按当前用户权限合并规则模型，不能将用户权限结果放入按 group ID 共享的模型缓存。两组白名单均生效；单模型查询和普通列表一致。路由过滤后的空列表是最终结果，不得再次回退默认模型。
- Codex 对规则选中的模型取实际组能力元数据，保留原目录其他条目的元数据；最终目录重算 ETag。固定账号目录同样合并规则模型。
- 固定来源目录没有可用账号时，普通与 Codex 目录仍合并有效目标池中的规则模型；来源上游请求失败继续遵守原错误策略。
- WS 的实际上游分组固定于首帧；后续轮次刷新来源规则并确认相同实际组，改变分组则要求重连。建连时没有规则的连接也必须识别后来启用的规则。计费与利润门每轮使用目标组新快照，不能从原 Key 认证快照恢复到来源组收费；仅刷新实际组，不采用新认证快照里的 Key 额度与用户限制。
- `usage_logs.group_id` 为实际组，新增 `source_group_id` 可空，仅跨组时填来源；原 API Key ID 不变。插入列、参数类型/参数、静态占位符、批量 SQL、SELECT/scan 必须保持同顺序。历史 NULL 不回填。

## 必测行为

- 两种模式、停用/限流/不支持模型、用户账号绑定、白名单/权限、目标删除/订阅类型、非递归。
- HTTP 流式/非流式实际上游、WS passthrough/dedicated 多轮调价与换组拒绝；原组提示词不被目标覆盖。
- 实际组倍率和用户倍率、原 Key 额度、单次扣费/RPM、source_group_id；SQL shape 和真实 PostgreSQL 往返。
- CRUD/省略/清空/复制、auth 投影/JSON 缓存/版本拒绝/失效、后台模式编辑、模型列表/单模型/Codex 元数据/ETag。
