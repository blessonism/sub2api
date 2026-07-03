# Admin Balance Manual Calibration Leaderboard Hint

## Goal

管理员在用户用量页手动校准 Token 和余额后，管理员 Token 排行榜展开用户详情时，既要提示手动校准 Token，也要提示手动校准余额，避免管理员误以为余额变化只来自正常使用或发放。

## Requirements

- 在管理员 Token 排行榜用户详情中展示余额手动校准净变化。
- 余额提示仅在当前筛选范围适合展示校准汇总时出现，与现有 Token 校准提示保持一致。
- 前端接口类型与后端响应字段保持一致。
- 中英文文案补齐，视觉风格复用现有校准提示。

## Acceptance Criteria

- [ ] 详情接口返回 `calibration_balance_delta`。
- [ ] 当 `calibration_balance_delta` 非 0 时，展开行显示余额校准提示。
- [ ] 当余额校准为正数或负数时，使用与现有 token 校准一致的正负颜色。
- [ ] 现有 Token 校准提示不回退。

## Definition of Done

- 针对性后端测试覆盖详情字段。
- 前端类型检查通过。
- 变更遵守下游 fork 分支边界。

## Technical Approach

复用 `admin_usage_calibrations.balance_delta` 作为余额校准来源，在管理员排行榜详情接口按目标用户和当前时间范围求和，返回给前端展示。前端在 `TokenLeaderboardView.vue` 中复用现有校准提示样式，并新增 i18n 文案。

## Decision (ADR-lite)

**Context**: 现有排行榜详情只返回 `calibration_tokens`，因此前端没有余额校准数据可提示。

**Decision**: 在详情接口增加 `calibration_balance_delta`，而不是在前端推断余额变化。

**Consequences**: 字段语义清晰，展示逻辑与现有 Token 校准一致；后续如果排行榜主行也要展示校准来源，可以沿用同一字段口径扩展。

## Out of Scope

- 不改变排行榜排序口径。
- 不把余额校准拆分到 API Key、分组或模型维度。
- 不调整普通用户可见页面。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 已读取 `.trellis/spec/frontend/index.md` 和 `.trellis/spec/frontend/type-safety.md`。
- 相关文件：
  - `backend/internal/pkg/usagestats/usage_log_types.go`
  - `backend/internal/repository/usage_log_repo.go`
  - `backend/internal/handler/admin/dashboard_handler.go`
  - `frontend/src/api/admin/dashboard.ts`
  - `frontend/src/views/admin/TokenLeaderboardView.vue`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`
