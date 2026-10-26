# 第二轮 Review

本轮发现并修复 5 类 P2 问题。全部先增加测试复现，再修正实现；没有调整生产配置。

1. **路由过滤后的空目录被默认模型填回。** 来源白名单只允许 Luna，而目标停用、删除或用户权限被撤销时，普通目录先正确移除了 Luna，随后又从默认目录补回，单模型查询也会误报可用。`gateway_handler.go` 现在直接返回路由过滤结果，空集同样是最终结果。回归：`TestModelGroupRoutesCatalogDoesNotRestoreUnavailableRouteFromDefaults`，覆盖三种不可用情况及列表/单模型查询。
2. **WS 漏掉建连后新启用的规则。** 原判断只读取建连时是否有规则，后来启用 target_only 后仍会继续向原池发送。`resolveModelGroupRouteWSTurn` 每轮读取当前来源配置，需要改变实际组时要求重连。回归：`TestModelGroupRoutesWebSocketLateEnableRequiresReconnect`，passthrough/dedicated 均断言第二轮不发上游。
3. **WS 刷新路由时替换了完整 Key/User 快照。** 原逻辑将新认证对象作为计费对象，建连时冻结的额度 10 会变成 999，用户限制也会更新。现在只把当前实际组赋给连接快照的私有副本。回归：`TestModelGroupRoutesWebSocketRefreshKeepsConnectionIdentityAndLimits`，目标调价生效，同时额度、用户对象、分组与来源保持连接契约。
4. **prefer_local 候选资格不完整。** 探查漏注入全局 quota auto-pause 设置，也未检查模型级冷却，会错误留在已经无法真正调度的本组。现在使用调度器相同全局设置和 `IsSchedulableForModelWithContext`。回归：`TestModelGroupRoutesPreferLocalUsesGlobalQuotaAutoPause` 与 `TestModelGroupRoutesPreferLocalUsesEligibilityWithoutAcquisition/model_rate_limited`；修正前均错误选中 51，修正后选中 40。
5. **来源固定目录无账号时掩盖健康目标目录。** 普通模型和 Codex 固定目录在来源账号缺失时均提前返回 503，没有合并目标池的可用 Luna。现在只在来源无可用目录账号时使用空来源合并，原上游失败策略保留。回归：`TestModelGroupRoutesPinnedCatalogWithoutLocalCandidates`，ordinary/codex 均由 503 修正为包含 Luna 的 200。

## 验证

```sh
cd backend
go test -tags=unit ./internal/service ./internal/handler ./internal/server/routes ./internal/server/middleware -run 'ModelGroup|Codex.*Models|Models.*Codex|OrdinaryPinnedModels|OpenAIResponsesWebSocket|RefreshOpenAIWSTurn|Gateway.*Models' -count=1
```

service、handler、routes 通过；middleware 此筛选没有命中测试。覆盖新增回归、HTTP 流式/非流式、WS 多轮调价和换组、现有普通固定目录/Codex 策略及路由注册。

- golangci-lint v2.14.0 按本次功能 tracked/untracked 变更 patch 检查：`0 issues`；排除原有 IQ 探针工作区变更。
- `git diff --check` 与 Trellis implement/check context validation 通过。

本轮没有修改 schema、repository 或前端；它们沿用上一轮已通过的真实数据库集成、Ent 生成、前端类型/构建/定向测试结果。

没有发现尚未修复的本次功能问题；本轮范围不是全仓库审计。
