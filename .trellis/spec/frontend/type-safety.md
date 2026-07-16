# Type Safety

> Type safety patterns in this project.

---

## Overview

<!--
Document your project's type safety conventions here.

Questions to answer:
- What type system do you use?
- How are types organized?
- What validation library do you use?
- How do you handle type inference?
-->

(To be filled by the team)

---

## Type Organization

<!-- Where types are defined, shared types vs local types -->

(To be filled by the team)

---

## Validation

<!-- Runtime validation patterns (Zod, Yup, io-ts, etc.) -->

(To be filled by the team)

---

## Common Patterns

<!-- Type utilities, generics, type guards -->

### Pattern: User dashboard API-backed pages

When adding a user-facing page backed by `/usage/dashboard/*` APIs:

- Define the response interfaces in `frontend/src/api/usage.ts` next to the request function.
- Keep frontend field names aligned with backend JSON names; do not remap security-sensitive fields such as `masked_email` into ambiguous names like `email`.
- Add route metadata in `frontend/src/router/index.ts`, including `requiresAuth`, `titleKey`, and `descriptionKey`.
- Add the page to `frontend/src/components/layout/AppSidebar.vue` through `buildSelfNavItems` when both regular users and admins' "My Account" section should see it.
- Add route prefetch coverage when the page should be eagerly prefetched.
- Add zh/en i18n keys for route titles, navigation labels, loading/error/empty states, and metric labels.

Required tests:

- API test verifies the endpoint path and typed method.
- View test covers loading, success, empty/error states, and important display fields.
- Sidebar/prefetch tests cover new route visibility and prefetch registration.

### Pattern: Upstream Relay Monitoring API Types

When updating `frontend/src/api/admin/upstreamRelayGroupMonitors.ts`:

- Keep connector-only fields on `UpstreamRelayConnector`, including `upstream_account_balance` and `upstream_account_balance_checked_at`.
- Keep candidate-mapping usage fields on `UpstreamRelayCandidate`, including nullable `today_actual_cost`, `today_total_tokens`, and `today_usage_checked_at`.
- Keep candidate-mapping upstream key fields on `UpstreamRelayCandidate`, including nullable `upstream_api_key_id` plus display-only `upstream_api_key_name` and `upstream_api_key_masked`.
- Keep `UpstreamRelayAPIKeyOption.group_id` aligned with the backend string ID. Selecting a Key should update the candidate form's group from this current upstream value; missing `group_id` retains the manual fallback.
- Keep candidate bindings account-scoped: `UpstreamRelayCandidate` and recommendation DTOs must not expose `target_group_id` / `target_group_name`.
- Do not add candidate upstream key fields to `UpstreamRelayConnector` or generic account API types; the mapping belongs to the upstream relay candidate form.
- Do not add upstream account balance fields to `UpstreamRelayCandidate`; candidate rows display the upstream relay user's real usage snapshot for `connector_id + upstream_group_id`, while connector rows display the upstream relay account balance.
- Treat missing candidate usage as unknown / not synced, not as zero usage.
- Add lightweight connector metrics refresh through `refreshConnectorMetrics(id)`, targeting `/connectors/:id/metrics/refresh`; do not reuse the full connector `sync` action for a balance/usage-only refresh.
- After metrics refresh succeeds, refresh connector and candidate state in the view so connector balance and candidate today usage update together.
- Treat metrics refresh detail collections from backend Go slices, such as `usage_detail.missing_groups`, as nullable or optional at the API boundary; normalize them to arrays before calling `.length`, `.slice`, `.some`, or rendering loops.
- Treat `usage_detail.issues` as the complete machine-readable problem list and `usage_detail.issue` as its first-item compatibility field. Branch on stable `code`, use `candidate_id` / `account_id` / `upstream_group_id` only to locate the affected mapping, and render localized business guidance. Keep sanitized backend `message` values behind an expandable technical-detail affordance instead of exposing them as the primary prompt. Raw-message parsing is compatibility fallback only.
- A missing or failing candidate API key must not abort other valid candidate bindings. Render root groups with their stable issue code (`missing_upstream_api_key_binding`, `upstream_api_key_not_visible`, `upstream_api_key_group_unavailable`, `upstream_usage_request_failed`), report successfully updated groups separately, and reserve `usage_refresh_aborted` for groups that genuinely could not be attributed to a more specific issue.
- For partial refreshes, treat groups absent from the returned known-usage set as unchanged/unknown; never replace their existing snapshot values with zero. A `partial` result may therefore include both `updated_groups > 0` and non-empty `missing_groups`.
- Keep recommendation preview and persisted generation as separate methods: `previewRecommendations()` must call `/recommendations/preview`; `generateRecommendations()` must call `/recommendations`.
- Normalize recommendation policy `sort_fields` before submit so duplicate fields are removed and missing default sort fields are appended.
- When showing "auto monitoring running" state, read from the last loaded/saved monitoring policy snapshot, not the editable form, so unsaved checkbox changes are not presented as active backend runner state.
- Add daily usage history through `listUsageHistory()`, targeting `/usage-history`, with `UpstreamRelayGroupUsageHistory` fields aligned to backend JSON: `usage_date`, `connector_id`, `connector_name`, `upstream_group_id`, `group_name`, `platform`, `actual_cost`, `total_tokens`, and `checked_at`.
- Keep usage-history filters typed as query params: `start_date`, `end_date`, `connector_id`, `upstream_group_id`, `search`, `include_zero_usage`, `page`, and `page_size`.
- Add zh/en i18n keys for every table column introduced in `UpstreamRelayGroupMonitoringView.vue`.

Required checks:

- `pnpm typecheck` passes.
- A targeted key scan confirms every `tM('candidates.*')` and `tM('connectors.*')` key used by the view exists in both locale files.
- API tests cover both preview and persisted generate endpoint paths, the connector metrics refresh endpoint path, and the usage-history endpoint path.
- View tests cover the usage-history tab loading/rendering path and required zh/en i18n keys.

### Pattern: Backend-normalized external metric snapshots

When adding a frontend API for external metrics displayed inside user pages:

- Fetch through the project backend route, not the third-party public URL.
- Keep interfaces in the API module that owns the feature, and align field names with backend JSON names.
- Parse unknown API data with runtime guards before exposing it to views.
- Preserve nullable unknown fields as `null`; do not convert unavailable external data to zero.
- If a legacy vendor wrapper is supported, keep it in the parser for compatibility tests only. The runtime fetch path should still use the backend route.

Required checks:

- API test verifies the backend endpoint path, abort signal, and timeout forwarding.
- Parser test covers backend-normalized payloads, legacy wrapped payloads, invalid payload rejection, and nullable optional fields.

### Pattern: Admin lottery campaign API types

When updating `frontend/src/api/lotteryCampaigns.ts` or `frontend/src/api/admin/lotteryCampaigns.ts`:

- Keep `LotteryCampaign` fields aligned with backend JSON names, including `is_featured`, `prize_tiers`, `draw_schedule_type`, `daily_draw_time`, and all timestamp fields.
- Add admin API methods for each backend management action instead of calling raw `apiClient` from Vue components. Current action paths include `/admin/lottery-campaigns/:id/feature` and `DELETE /admin/lottery-campaigns/:id`.
- Use `/admin/lottery-campaigns/:id/winners` for an activity-level admin winner list; keep `/draw-batches/:batch_id/winners` for batch-scoped tools. Never reuse the admin `LotteryWinner` response in user-facing winner views.
- Treat `is_featured` as persistent user-page selection, not as an active-time-window signal. The admin feature action is available for any `published` campaign, while enrollment controls still depend on backend entry-window state.
- For user lottery round state, use backend `round_completed` and `LotteryPublicWinner.is_current_round`; never infer completion from a non-empty campaign-wide winner history.
- Reuse the full `LotteryCampaignRequest` payload for create and edit so published campaign edits stay contract-compatible with backend validation.
- Keep destructive copy in i18n, not hardcoded component strings, and include zh/en keys for edit, hard delete, cascade warning, feature selection, and validation messages.
- Validate obvious time errors in the admin UI before submit, but keep backend validation authoritative.

Required checks:

- API test verifies create, update, publish, cancel, feature, hard delete, sync, draw, and activity-level winner endpoint paths.
- Admin component test covers an ended published campaign being selectable for persistent display and rendering its winner records.
- `pnpm typecheck` passes after adding fields to `LotteryCampaign`.
- Targeted i18n scan or component review confirms every `admin.lotteryCampaigns.*` key used by the panel exists in both locale files.

### Pattern: Downstream i18n locale overlays

更新 `frontend/src/i18n/locales/**` 时，保留下游定制文案和上游模块化文案的边界：

- 每个语言入口使用 `frontend/src/i18n/locales/<locale>/index.ts` 组装上游模块，并通过 `mergeLocale(base, custom)` 合并下游覆盖。
- 下游专属或覆盖上游的 key 放在 `frontend/src/i18n/locales/<locale>/custom.ts`，不要恢复旧的单文件 `en.ts` / `zh.ts`。
- `custom.ts` 只保存二开差异，避免复制整棵上游 locale tree；需要修改上游模块时优先确认是否属于真正的通用文案。
- `frontend/src/i18n/locales/mergeLocale.ts` 负责递归合并对象树，overlay 的字符串或非对象值覆盖 base。
- zh/en 必须同步添加同名 key，组件中新增 `t()` / `tM()` 调用时同步补齐测试覆盖。

Required checks:

- `pnpm typecheck` passes.
- `pnpm exec vitest run src/i18n/__tests__/localesNoKeyCollision.spec.ts src/i18n/__tests__/opsLocaleKeys.spec.ts` passes.
- When a component introduces new i18n surfaces, run or add the closest component test that renders those keys.

---

## Forbidden Patterns

<!-- any, type assertions, etc. -->

(To be filled by the team)
