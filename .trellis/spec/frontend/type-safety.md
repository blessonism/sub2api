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
- Keep candidate bindings account-scoped: `UpstreamRelayCandidate` and recommendation DTOs must not expose `target_group_id` / `target_group_name`.
- Do not add candidate upstream key fields to `UpstreamRelayConnector` or generic account API types; the mapping belongs to the upstream relay candidate form.
- Do not add upstream account balance fields to `UpstreamRelayCandidate`; candidate rows display the upstream relay user's real usage snapshot for `connector_id + upstream_group_id`, while connector rows display the upstream relay account balance.
- Treat missing candidate usage as unknown / not synced, not as zero usage.
- Add lightweight connector metrics refresh through `refreshConnectorMetrics(id)`, targeting `/connectors/:id/metrics/refresh`; do not reuse the full connector `sync` action for a balance/usage-only refresh.
- After metrics refresh succeeds, refresh connector and candidate state in the view so connector balance and candidate today usage update together.
- Treat metrics refresh detail collections from backend Go slices, such as `usage_detail.missing_groups`, as nullable or optional at the API boundary; normalize them to arrays before calling `.length`, `.slice`, `.some`, or rendering loops.
- Keep recommendation preview and persisted generation as separate methods: `previewRecommendations()` must call `/recommendations/preview`; `generateRecommendations()` must call `/recommendations`.
- Normalize recommendation policy `sort_fields` before submit so duplicate fields are removed and missing default sort fields are appended.
- When showing "auto monitoring running" state, read from the last loaded/saved monitoring policy snapshot, not the editable form, so unsaved checkbox changes are not presented as active backend runner state.
- Add daily usage history through `listUsageHistory()`, targeting `/usage-history`, with `UpstreamRelayGroupUsageHistory` fields aligned to backend JSON: `usage_date`, `connector_id`, `connector_name`, `upstream_group_id`, `group_name`, `platform`, `actual_cost`, `total_tokens`, and `checked_at`.
- Keep usage-history filters typed as query params: `start_date`, `end_date`, `connector_id`, `upstream_group_id`, `search`, `page`, and `page_size`.
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

---

## Forbidden Patterns

<!-- any, type assertions, etc. -->

(To be filled by the team)
