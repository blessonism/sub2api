# 已授权的本地提交与合并计划

代码修复、检查与规格同步已完成。2026-10-26 用户回复「合并到custom main」，授权本功能本地提交、合并 custom/main 及前述 Trellis 收尾；不包含 push 或生产部署。

## 已授权执行顺序

1. `feat(gateway): 支持按模型转发分组`：本功能后端、迁移、前端、测试、规格和本任务文档。
2. `Merge feature/model-group-routes: 分组按模型转发与 Luna 接入`：合并到本地 custom/main，并在干净临时检出验证。
3. `chore(task): archive model-group-routes`：仅归档本 Trellis 任务（由 task.py archive 生成）。
4. `chore: record journal`：仅记录本次会话，引用第一项工作提交（由 add_session.py 生成）。

影响仅限 feature/model-group-routes 和 custom/main 本地历史；不 push、不部署、不调整生产配置。若要撤销已提交内容，以 git revert 对应提交回滚，不使用 reset 或 amend。

## 第一项提交：完整文件

- `.trellis/spec/backend/index.md`
- `.trellis/spec/backend/model-group-routes.md`
- `backend/ent/group.go`
- `backend/ent/group/group.go`
- `backend/ent/group_create.go`
- `backend/ent/group_update.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `backend/ent/schema/group.go`
- `backend/ent/schema/usage_log.go`
- `backend/ent/usagelog.go`
- `backend/ent/usagelog/usagelog.go`
- `backend/ent/usagelog/where.go`
- `backend/ent/usagelog_create.go`
- `backend/ent/usagelog_update.go`
- `backend/internal/domain/model_group_route.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/model_group_routes.go`
- `backend/internal/handler/model_group_routes_test.go`
- `backend/internal/handler/openai_codex_models_handler.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_gateway_handler_test.go`
- `backend/internal/handler/openai_models_handler.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/group_repo.go`
- `backend/internal/repository/model_group_routes_integration_test.go`
- `backend/internal/repository/usage_log_repo_insert.go`
- `backend/internal/repository/usage_log_repo_insert_shape_unit_test.go`
- `backend/internal/repository/usage_log_repo_query.go`
- `backend/internal/repository/usage_log_repo_request_type_test.go`
- `backend/internal/repository/usage_log_session_id_unit_test.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/routes/gateway_model_allowlist_test.go`
- `backend/internal/service/admin_group.go`
- `backend/internal/service/admin_group_duplicate.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/api_key.go`
- `backend/internal/service/api_key_auth_cache.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/internal/service/group.go`
- `backend/internal/service/model_group_routes.go`
- `backend/internal/service/model_group_routes_test.go`
- `backend/internal/service/openai_gateway_usage.go`
- `backend/internal/service/usage_log.go`
- `backend/migrations/247_model_group_routes.sql`
- `frontend/src/components/admin/group/ModelGroupRouteFields.vue`
- `frontend/src/components/admin/group/__tests__/ModelGroupRouteFields.spec.ts`
- `frontend/src/components/admin/usage/UsageTable.vue`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/user/UsageView.vue`
- `.trellis/tasks/10-26-model-group-routes/` 内本任务文档与示例配置（包括本计划）。

## 第一项提交：仅指定 hunks

- `frontend/src/i18n/locales/en/custom.ts`：仅 usage.modelGroupRoute 与 admin.groups.modelGroupRoutes；同文件原有 candyProbe / modelIqMonitor 改动保持未提交。
- `frontend/src/i18n/locales/zh/custom.ts`：仅 usage.modelGroupRoute 与 admin.groups.modelGroupRoutes；同文件原有 candyProbe / modelIqMonitor 改动保持未提交。

提交前按该清单逐项检查暂存 diff；禁止用 git add . 或把混合文件整体加入暂存。

## 不纳入本次提交的其他工作区内容

- `.trellis/spec/backend/model-iq-probe.md`
- `backend/internal/handler/admin/model_iq_probe_handler.go`
- `backend/internal/handler/model_iq_probe_user_handler.go`
- `backend/internal/repository/model_iq_probe_repo.go`
- `backend/internal/repository/model_iq_probe_repo_integration_test.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/service/account_test_service.go`
- `backend/internal/service/model_iq_probe_runner.go`
- `backend/internal/service/model_iq_probe_service.go`
- `frontend/src/api/__tests__/modelIqProbe.spec.ts`
- `frontend/src/api/admin/modelIqProbe.ts`
- `frontend/src/api/modelIqProbe.ts`
- `frontend/src/components/user/__tests__/CandyProbePanel.spec.ts`
- `frontend/src/components/user/monitor/CandyProbePanel.vue`
- `frontend/src/views/admin/ModelIqMonitorView.vue`
- `frontend/src/views/user/ChannelStatusV1View.vue`
- `.omo/`
- `.trellis/tasks/09-17-sync-upstream-main-preserve-custom-20260917/`
- `.trellis/tasks/10-26-model-iq-probe/`
- `.trellis/tasks/archive/2026-10/10-14-deploy-ovh-v0-2-16/`
- `.trellis/workspace/openai-group-scheduler-profile-review-relay.md`
- `.trellis/workspace/openai-group-scheduler-profile-review.md`
- `.zcode/`
- `backend/internal/handler/admin/model_iq_probe_handler_test.go`
- `backend/internal/handler/model_iq_probe_user_handler_test.go`
- `backend/internal/repository/model_iq_probe_repo_test.go`
- `backend/internal/service/model_iq_probe_regression_test.go`
- `backend/internal/service/model_iq_probe_run_state.go`
- `frontend/preview.html`
- `frontend/src/preview/`
- `frontend/src/views/admin/__tests__/ModelIqMonitorView.spec.ts`
- `frontend/vite.preview.config.ts`

这些内容属于原有 IQ 探针、预览、历史任务或其他工作；保持原样。本计划不归档其他任务。

## 检查依据

- 功能验收与既有检查：verification.md。
- 第二轮五类 P2 的修正与回归：review-2.md。
- 本轮再次确认 git diff --check 与任务 implement/check context validation 通过。
- 上轮检查后本轮仅整理提交计划及收尾状态，没有新增代码变更，因此不重复运行已通过的相同测试。

## 确认来源

- `.trellis/workflow.md` Phase 3.4 要求展示提交计划并一次确认后执行。
- `.trellis/spec/guides/downstream-fork-workflow.md`「高风险操作」明确包含 git commit，并要求确认前说明影响范围与回滚方式。

## 本地执行结果

- 功能提交：c9335e5358df44d026ae597aaf680bae6e6c4da2。
- custom/main 合并提交：c387b6d8ac2df8134090a3f8e95e32f5151016cd。
- 干净合并检出的 Go 定向、前端类型检查和 74 项定向测试通过；详细说明见 verification.md。
