# brainstorm: 上游中继候选显式绑定 API Key

## Goal

将上游中继 usage 刷新的 API Key 映射从“按本地账号 key/name 猜测”改为“候选映射显式绑定上游 API Key ID”，避免上游 key 脱敏、名称不一致或重名导致 `bound account ... api key id/name is not visible in upstream keys`。

## What I already know

- 连接器代表上游登录账号，负责上游站点、登录态和余额。
- 候选映射代表本地账号/分组到上游 group/key 的映射关系。
- 上游 `/api/v1/keys` 可能只返回脱敏 key，不能稳定用本地明文 key 反查上游 key。
- 当前 usage 刷新已改为调用 `/api/v1/usage/stats?api_key_id=...`，因此稳定主键应是上游 API Key ID。
- 当前工作树已有同一功能区的未提交改动，本任务需增量修改，不能重置或覆盖用户/并行任务改动。

## Assumptions

- 候选映射新增 `upstream_api_key_id`，可附带 `upstream_api_key_name` 和 `upstream_api_key_masked` 用于展示。
- 创建/编辑候选时由管理员从当前连接器可见的上游 key 列表选择 key。
- 历史候选没有显式 key 时，usage 刷新应报告该候选未绑定 key，而不是回退到不可靠的 key/name 猜测。

## Requirements

- 候选映射数据库字段承载上游 key 绑定，不写入连接器，也不写入通用 `accounts.credentials`。
- 后端 candidate DTO/input/repository/service 支持新增 key 绑定字段。
- 提供连接器维度的上游 key 列表读取能力，供前端候选表单选择。
- usage 刷新从候选绑定读取 `upstream_api_key_id`，不再依赖本地账号 key/name 猜测。
- 前端候选创建/编辑表单展示并提交上游 key 绑定字段。
- 更新前后端类型与 i18n，保持字段命名一致。
- 上游 key 展示值必须由后端强制脱敏后再返回/持久化，不能信任上游已脱敏。

## Acceptance Criteria

- [x] 候选创建/更新能保存上游 API Key ID 与展示信息。
- [x] 候选列表能返回绑定的上游 API Key 展示信息。
- [x] 刷新连接器指标时，已绑定 key 的候选可刷新今日 usage。
- [x] 未绑定 key 的候选给出明确 usage_error，不再提示“account key/name 不可见”。
- [x] 相关后端单测和前端类型/API 测试覆盖新增契约。

## Definition of Done

- Tests added/updated for backend service/repository/API path where appropriate.
- Frontend API types and candidate form stay aligned with backend JSON.
- Specs updated where previous monitoring guidance conflicts with explicit candidate key binding.
- Downstream fork workflow constraints followed.

## Out of Scope

- 不引入新的上游 key 创建能力。
- 不迁移生产数据或自动猜测历史候选 key 绑定。
- 不做远程部署、数据库实际迁移执行或 git push。

## Technical Notes

- Required downstream fork guide: `.trellis/spec/guides/downstream-fork-workflow.md`。
- Backend active guideline: `.trellis/spec/backend/quality-guidelines.md`。
- Frontend active guideline: `.trellis/spec/frontend/type-safety.md`。
- Existing service hotspot: `backend/internal/service/upstream_relay_group_monitoring.go`。
- Existing frontend hotspot: `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue`。

## Verification

- `cd backend && go test ./internal/service -run 'TestUpstreamRelay' -count=1`
- `cd backend && go test ./internal/repository -run 'TestUpstreamRelayRepository' -count=1`
- `cd backend && go test ./internal/server/routes -run 'TestUpstreamRelayGroupMonitoringRoutesAreRegistered' -count=1`
- `cd backend && go test ./internal/handler/admin -run 'TestUpstreamRelay' -count=1`
- `cd frontend && pnpm vitest run "src/api/__tests__/admin.upstreamRelayGroupMonitors.spec.ts"`
- `cd frontend && pnpm vitest run "src/views/admin/__tests__/UpstreamRelayGroupMonitoringView.spec.ts"`
- `cd frontend && pnpm typecheck`
