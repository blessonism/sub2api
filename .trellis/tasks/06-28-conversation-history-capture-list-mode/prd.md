# brainstorm: 对话历史采集黑白名单模式

## Goal

让管理员在对话历史采集中可以选择“黑名单模式”或“白名单模式”，并把当前手填 ID 的配置体验优化成更顺滑、可理解、低误操作的交互。目标是既支持默认广泛采集后排除指定主体，也支持只采集指定用户或 API Key 的更安全策略。

## What I already know

- 用户希望对话历史采集有两种模式：黑名单和白名单。
- 用户明确提到“交互希望丝滑一些”，说明不能只在现有表单里增加更多逗号分隔输入。
- 当前后端 `ConversationCaptureConfig` 已有 `excluded_user_ids` 和 `excluded_api_key_ids`，本质是黑名单能力。
- 当前后端采集决策位于 `ConversationCaptureService.DecideForEndpoint`：开启采集、端点匹配、排除名单、采样率命中后才采集。
- 当前前端管理页位于 `frontend/src/views/admin/ConversationHistoryView.vue`，采集规则区域使用数字输入和逗号分隔 ID 文本框。
- 当前前端 API 类型位于 `frontend/src/api/admin/conversations.ts`，需要和后端配置结构同步。
- 当前中英文文案分别位于 `frontend/src/i18n/locales/zh.ts` 和 `frontend/src/i18n/locales/en.ts`。

## Assumptions (temporary)

- “黑名单模式”含义：默认按端点和采样率采集，命中排除用户或排除 API Key 时不采集。
- “白名单模式”含义：默认不采集，只有命中允许用户或允许 API Key 时才进入采样与采集。
- 白名单与黑名单互斥展示，避免管理员同时维护两套名单造成心智负担。
- 为兼容既有配置，默认模式应保持黑名单，并继续读取已有 `excluded_user_ids` / `excluded_api_key_ids`。

## Open Questions

- 暂无阻塞问题。

## Requirements (evolving)

- 后端配置需要显式表达采集名单模式，例如 `subject_filter_mode: blacklist | whitelist`。
- 后端配置需要支持白名单集合，例如 `included_user_ids` 和 `included_api_key_ids`。
- 白名单模式下，只要命中允许用户 ID 或允许 API Key ID 中任意一个，就允许进入采样与采集。
- 后端校验需要保证所有名单 ID 为正数，并去重归一化。
- 后端采集决策需要在采样率前完成名单模式判断。
- 前端采集规则区域需要用模式选择控件表达“默认采集后排除”与“默认不采集，只允许指定对象”。
- 前端名单录入需要比逗号输入更顺手，至少支持粘贴多个 ID、回车添加、标签化展示、逐个删除、空态提示。
- 文案需要解释当前模式对新增请求的影响，避免误以为会修改已有历史数据。

## Acceptance Criteria (evolving)

- [ ] 管理员可以在对话历史采集设置中切换黑名单/白名单模式。
- [ ] 黑名单模式保持现有行为：名单为空时按采样率采集，命中排除名单时跳过。
- [ ] 白名单模式下，名单为空时不采集任何主体，并在 UI 中给出明确提示。
- [ ] 白名单模式下，同时配置用户 ID 和 API Key ID 时，命中任意一个名单即可采集。
- [ ] 用户 ID 和 API Key ID 名单支持添加、粘贴、删除、去重和保存后回显。
- [ ] 保存配置后，后端返回归一化后的模式与名单。
- [ ] 旧配置未包含新字段时，系统默认进入黑名单模式且不改变既有采集行为。
- [ ] 单元测试覆盖黑名单、白名单、空白名单、无效 ID 和采样顺序。
- [ ] 前端测试覆盖模式切换、名单编辑和保存 payload。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 不在本任务中批量迁移或重写已有对话历史数据。
- 不在本任务中改变导出任务、质量标注、会话合并/拆分逻辑。
- 不在本任务中实现复杂的用户/API Key 搜索弹窗，除非现有组件和接口可以低成本复用。

## Technical Notes

- 下游二开边界：`.trellis/spec/guides/downstream-fork-workflow.md` 已加入 implement/check 上下文。
- 主要后端候选文件：
  - `backend/internal/service/conversation_capture_models.go`
  - `backend/internal/service/conversation_capture_service.go`
  - `backend/internal/config/config.go`
  - 相关 handler 与 service 测试
- 主要前端候选文件：
  - `frontend/src/api/admin/conversations.ts`
  - `frontend/src/views/admin/ConversationHistoryView.vue`
  - `frontend/src/views/admin/__tests__/ConversationHistoryView.spec.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`

## Candidate UX Direction

- 推荐采用分段控制：
  - 黑名单：默认采集，排除以下对象。
  - 白名单：默认不采集，只采集以下对象。
- 名单编辑推荐采用“标签输入”：
  - 支持输入 `1,2,3` 后自动拆成标签。
  - 支持回车添加单个 ID。
  - 支持删除单个标签。
  - 支持展示数量摘要，例如“已排除 3 个用户、2 个 API Key”。
- 当白名单模式且两个名单都为空时，显示醒目的轻量提示：当前不会采集任何新对话。
