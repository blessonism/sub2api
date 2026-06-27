# Phase 1 Review 修复方案

日期：2026-06-27

范围：OpenAI 对话历史查看与训练数据导出 Phase 1。本文只记录 review 发现和大致修复方案，不扩大到 Phase 2 能力。

## 结论

当前实现方向基本符合 Phase 1：主路径是 `conversation_sessions` / `conversation_turns`，只接入 OpenAI `chat/completions`，未实现 Responses 采集、raw archive、后台导出任务或 JSONL.zst。

进入 Phase 2 前建议先修复两个 P1：

1. 导出 `exportable` 语义漂移，可能漏导手动标记的 turn，也可能让未来新 turn 被 session 级开关隐式导出。
2. 同一显式 session 的首轮并发创建存在唯一约束竞争，可能丢失 turn。

另外有两个 P2 建议在 Phase 1 收尾一并处理：

1. turn 列表接口不应读取完整正文 JSONB。
2. 前端配置页缺少 `session_window_minutes` 控件。

## P1：导出 exportable 语义

### 问题

当前仓储导出条件使用：

```sql
(t.exportable = TRUE OR s.exportable = TRUE)
```

同时前端导出按钮固定传：

```ts
exportable: true
```

这个字段进入后端后会被解释为 session 过滤条件，即 `s.exportable = true`。

### 风险场景

1. 管理员只标记单个 turn 为可导出，session 仍为 `exportable=false`。
   - 前端导出传 `exportable=true`。
   - SQL 额外要求 `s.exportable=true`。
   - 已标记的 turn 被漏导。
2. 管理员把 session 标记为可导出后，该 session 后续又新增 turn。
   - 新 turn 默认 `t.exportable=false`。
   - 只要它解析成功、未截断、未断连，就会因为 `s.exportable=true` 被导出。
   - 这不符合“exportable 默认 false，只能由管理员手动标记”的安全边界。

### 目标语义

- `conversation_turns.exportable` 是导出的唯一硬门槛。
- `conversation_sessions.exportable` 只表示“管理员在某一时刻对 session 做了批量标记动作”或 UI 汇总状态，不应让未来 turn 自动进入导出。
- `SetSessionExportable(true)` 可以批量把当前已存在且 eligible 的 turns 物化为 `t.exportable=true`。
- `SetSessionExportable(false)` 可以批量把该 session 当前 turns 置为 `false`。
- 导出默认仍必须满足：
  - `t.exportable = TRUE`
  - `t.parse_status = 'success'`
  - `t.truncated = FALSE`
  - `t.client_disconnect = FALSE`
  - 默认排除 `s.session_source = 'heuristic'`
  - 默认按 `dedupe_hash` 去重

### 建议修复

1. 修改 `ListExportableTurns`：

```sql
WHERE t.exportable = TRUE
  AND t.parse_status = 'success'
  AND t.truncated = FALSE
  AND t.client_disconnect = FALSE
```

2. 不再把 `ConversationSessionFilters.Exportable` 直接复用到导出请求，或明确改名：
   - 列表页保留 `exportable` 作为 session 过滤。
   - 导出请求如需过滤 session 状态，使用 `session_exportable`。
   - 默认前端导出不传 session 级 `exportable=true`，只依赖后端的 turn 级硬门槛。
3. 前端 `exportJSONL()` 改为传当前筛选条件时排除 `exportable`，避免把 UI 列表筛选误用于导出硬门槛。
4. 增加回归测试：
   - turn exportable=true、session exportable=false 时可导出。
   - session exportable=true、turn exportable=false 时不可导出。
   - session 标记 true 只更新当前 eligible turns。
   - session 标记后新增的默认 turn 不会被导出，除非再次标记 turn/session。

## P1：显式 session 首轮并发创建

### 问题

当前 `UpsertTurn` 流程是：

1. `SELECT turn_count FROM conversation_sessions WHERE session_id = $1 FOR UPDATE`
2. 未命中则 `INSERT conversation_sessions`
3. 用 `currentTurnCount + 1` 插入 turn

如果同一 `session_id` 的两个 worker 同时处理首轮请求，两个事务都可能先查不到 session，其中一个事务插入成功，另一个事务插入时撞 `conversation_sessions.session_id` 唯一约束并整体失败。

### 风险场景

- 内部客户端使用同一个 `X-Conversation-ID` 并发发起多条请求。
- OpenAI 客户端重试或代理层并发补偿导致同一显式 conversation ID 同时进入 worker。
- 结果是其中一条 turn 丢失，session 聚合计数不完整。

### 目标语义

- 同一 `session_id` 的并发首轮请求不应因为 session 创建竞争丢失 turn。
- turn_index 必须在 session 级事务内递增，不能重复。
- 如果 turn insert 没有实际插入，不应更新 session 聚合计数。

### 建议修复

推荐方案：`INSERT ... ON CONFLICT DO NOTHING` + `SELECT ... FOR UPDATE`。

流程：

1. 先尝试插入 session：

```sql
INSERT INTO conversation_sessions (...)
VALUES (...)
ON CONFLICT (session_id) DO NOTHING
```

2. 再读取并锁定 session：

```sql
SELECT turn_count
FROM conversation_sessions
WHERE session_id = $1
FOR UPDATE
```

3. 插入 turn。
4. 检查 `RowsAffected()`：
   - `1`：更新 session 聚合计数。
   - `0`：说明 `(session_id, turn_index)` 已存在，应返回可观测错误或重试分配下一个 turn index；不要继续累加 session。

更稳妥的替代方案：

- 使用 `pg_advisory_xact_lock(hash(session_id))` 在事务内串行化同一 session 写入。
- 或捕获唯一约束错误后短重试一次完整事务。

### 测试建议

- repository 并发测试：同一 `session_id` 并发 `UpsertTurn` 两次，最终 session `turn_count=2`，turn_index 为 `1,2`。
- 冲突 no-op 测试：模拟 turn insert 未插入时，不更新 session token/cost/turn_count。

## P2：turn 列表避免读取大正文

### 问题

`ListTurnsBySessionID` 复用 `conversationTurnSelectSQL()`，会读取：

- `request_messages`
- `response_messages`
- `tools`
- `usage`
- `meta`

这些字段可能很大。列表页打开 session 时会分页读取完整正文，不符合“列表避免读取大正文”的设计目标。

### 建议修复

1. 新增轻量 DTO，例如 `ConversationTurnSummary`：
   - `id`
   - `session_id`
   - `request_id`
   - `turn_index`
   - `provider`
   - `model`
   - `stream`
   - `client_disconnect`
   - `truncated`
   - `quality_status`
   - `exportable`
   - `parse_status`
   - `parse_error`
   - `input_tokens`
   - `output_tokens`
   - `total_tokens`
   - `actual_cost`
   - `payload_preview`
   - `created_at`
2. `GET /sessions/:id/turns` 返回 summary。
3. `GET /turns/:id` 返回完整 `ConversationTurn`，前端点击某个 turn 再加载正文。
4. 前端详情面板先展示 preview 和状态，展开 turn 时再请求完整 messages。

### 测试建议

- repository 列表 SQL 不包含 `request_messages` / `response_messages`。
- handler/API 类型区分 summary 与 detail。
- 前端 typecheck 覆盖新类型。

## P2：前端配置补齐 session window

### 问题

后端和 PRD 均支持 `session_window_minutes`，但当前配置页未提供输入控件。

### 建议修复

- 在配置区增加 `session_window_minutes` 数字输入，最小值为 `1`。
- 保存时继续复用后端校验。
- i18n 增加中英文文案。

## 验证清单

修复后至少运行：

```bash
cd backend && go test ./internal/repository -run 'TestConversationCaptureRepository'
cd backend && go test ./internal/service -run 'TestConversationCapture|TestParseOpenAIChatCompletions|TestConversationExport'
cd backend && go test ./internal/handler -run 'TestConversationCapture|Test.*UsageRecord'
cd backend && go generate ./ent/...
cd frontend && pnpm typecheck
git diff --check
```

如果继续运行宽测试，当前工作区已知仍可能被其它任务失败阻塞：

- `usage_log_repo_request_type_test.go` insert 参数 / scan 计数相关失败。
- `TestGetUserGroupRateMultiplier_CacheHitAndNilRepo` 属于当前 `feature/group-visible-rate-multiplier` 分支主题。

## Phase 2 前置条件

进入 Phase 2 前建议满足：

- 导出硬门槛仅依赖 turn 级 `exportable=true`，不会自动导出未来新 turn。
- 同一显式 session 并发写入不丢 turn。
- turn 列表接口不读取完整大正文。
- 前端配置项覆盖 PRD 要求的核心运行时配置。
- 聚焦后端测试、ent 生成、前端 typecheck 均通过。
