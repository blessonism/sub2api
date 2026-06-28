# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

### Scenario: User-visible usage ranking/statistics APIs

#### 1. Scope / Trigger
- Trigger: adding or changing an authenticated user-facing API that aggregates another users' usage data, such as token leaderboards or usage rankings.
- These APIs cross storage, service, handler, frontend API types, and UI. They require code-spec depth because a small contract drift can leak user identity or show incorrect ranks.

#### 2. Signatures
- Route pattern: `GET /api/v1/usage/dashboard/<feature>`.
- Handler pattern: get the auth subject with `middleware.GetAuthSubjectFromContext`; return `response.Unauthorized` when absent.
- Service pattern: expose a user-scoped method that receives `ctx`, current `userID`, and an explicit time window.
- Repository pattern: return internal rows for aggregation results, not public response DTOs.

#### 3. Contracts
- Public response fields must include only display-safe identity fields.
- For ranking APIs, return both the displayed ranking slice and a current-user item such as `my_rank`, even when the current user is outside the displayed Top N.
- Internal repository rows may contain raw identifiers or emails, but they must use `json:"-"` and must not be returned from handlers.
- Email masking belongs in the service layer before building the public DTO.

#### 4. Validation & Error Matrix
- Missing auth subject -> `401 User not authenticated`.
- Repository query failure -> wrap with feature context in the service and use `response.ErrorFrom` in the handler.
- Empty usage data -> return an empty ranking plus a safe current-user item instead of an error.
- Current user outside Top N -> return Top N unchanged and fill `my_rank` from the extra current-user query row.

#### 5. Good/Base/Bad Cases
- Good: `ranking` contains Top10 rows sorted by the agreed metric, while `my_rank` contains the current user even if rank is `37`.
- Base: no usage today returns `ranking: []` and a masked current-user identity with zero metrics.
- Bad: reusing an admin response type that exposes `email` to ordinary users.

#### 6. Tests Required
- Repository test: ranking order, limit handling, and current-user-outside-Top-N behavior.
- Service test: raw emails are converted to masked emails and public DTOs do not expose raw identity fields.
- Handler/API contract test: authenticated route is registered and returns the expected response shape.

#### 7. Wrong vs Correct

Wrong:
```go
response.Success(c, rows) // rows contain raw email/user IDs for internal aggregation
```

Correct:
```go
response.Success(c, publicResponse) // publicResponse contains masked_email only
```

### Scenario: Admin dashboard operational metrics

#### 1. Scope / Trigger
- Trigger: adding or changing fields returned by `GET /api/v1/admin/dashboard/stats`.
- These fields feed the admin dashboard directly and often mix pre-aggregated usage data with real-time entity/accounting totals.

#### 2. Signatures
- Handler: `DashboardHandler.GetStats`.
- Repository: `UsageLogRepository.GetDashboardStats(ctx)` and `GetDashboardStatsWithRange(ctx, start, end)`.
- Response field names must stay snake_case and aligned with `frontend/src/types/index.ts`.

#### 3. Contracts
- `today_active_users`: same compatibility scope as legacy `active_users`.
- `yesterday_active_users`: previous local dashboard day from `usage_dashboard_daily.active_users`.
- Token-active users must be based on effective billed usage (`usage_logs.actual_cost > 0`), not just request rows.
- `total_user_balance`: sum of `users.balance` for non-deleted users.
- `subscription_remaining_value`: active, non-deleted users' subscriptions prorated by remaining validity time.

#### 4. Validation & Error Matrix
- Missing daily aggregate row -> return zero for that field.
- Invalid dashboard range (`end <= start`) -> return error before querying.
- Subscription without matching order/plan price -> contributes zero instead of failing the whole dashboard.

#### 5. Good/Base/Bad Cases
- Good: a zero-cost failed request increases request totals but does not increase active user counts.
- Base: no subscription orders or plans returns `subscription_remaining_value: 0`.
- Bad: counting `COUNT(DISTINCT user_id)` across all usage logs as token-active users.

#### 6. Tests Required
- Handler test asserts new response fields are emitted.
- Repository/integration test covers balance pool, yesterday active users, subscription order pricing, plan fallback, and zero-cost active-user exclusion.

#### 7. Wrong vs Correct

Wrong:
```sql
SELECT COUNT(DISTINCT user_id) FROM usage_logs
```

Correct:
```sql
SELECT COUNT(DISTINCT user_id) FROM usage_logs WHERE actual_cost > 0
```

---

### Scenario: Admin token usage auto policy APIs

#### 1. Scope / Trigger
- Trigger: adding or changing the admin feature that assigns user group-specific rate multipliers from rolling token usage.
- This feature crosses database migrations, repository SQL, service policy decisions, admin handlers, frontend API types, and a management UI. It requires code-spec depth because contract drift can overwrite manual billing configuration or remove group access incorrectly.

#### 2. Signatures
- Route prefix: `/api/v1/admin/token-usage-policies`.
- Required endpoints: `GET /`, `POST /`, `GET /:id`, `PUT /:id`, `DELETE /:id`, `POST /:id/preview`, `POST /:id/run`, `POST /:id/clear`, `GET /:id/runs`, `GET /:id/runs/:run_id/changes`.
- DB tables: `token_usage_auto_policies`, `token_usage_auto_policy_tiers`, `token_usage_auto_assignments`, `token_usage_auto_runs`, `token_usage_auto_run_changes`.
- Rate write target: `user_group_rate_multipliers(user_id, group_id).rate_multiplier`; do not update `rpm_override`.
- Group grant target: `user_allowed_groups(user_id, group_id)`; only write columns that exist in the schema (`user_id`, `group_id`, `created_at`).

#### 3. Contracts
- Token usage is the sum of `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`.
- Only successful billed usage participates in tier matching: `usage_logs.actual_cost > 0`.
- Policy filters affect aggregation only: `group_id`, `model`, `request_type`, `billing_type`; they must not implicitly change the target group.
- A policy has one fixed `target_group_id` and many ordered tiers; only one enabled policy may target the same group.
- `manual_priority` must not overwrite an existing manual `rate_multiplier`, including the first time a user is seen before any auto assignment exists.
- `grant_group_and_rate` may add the target group but must not remove any existing groups. Clearing may remove only the group grant that the same policy created.
- Clearing an auto rate must set `rate_multiplier` back to the captured `previous_rate_multiplier` when present, otherwise set it to `NULL`; preserve `rpm_override`.
- `GET /:id/runs` must return run summary rows only; user-level change details must be loaded from paginated `GET /:id/runs/:run_id/changes`.
- Successful real runs must apply rate/group changes, persist `token_usage_auto_run_changes`, and update the run summary in one transaction so audit failure cannot leave applied configuration without matching history.
- `token_usage_auto_run_changes.target_group_id` is an audit snapshot, not a live `groups` relationship; do not add a cascading `groups` foreign key that can delete history when a group is removed.
- Explicit policy clearing must create a `run_type='clear'` run, persist `clear` run changes, and remove only this policy's assignment footprint. For manual takeover rows, clearing may remove the assignment record and this policy's own group grant, but must not overwrite the current manual `rate_multiplier`.
- Manual takeover reason matters during clearing: a row whose only takeover reason is that the policy-granted group access was manually removed should still clear/restore the policy-owned rate footprint, while a real manual rate edit must preserve the current manual `rate_multiplier`.
- Explicit policy clearing must disable the policy and clear `next_run_at` in the same transaction as apply/audit/summary, so the scheduler cannot automatically re-apply the policy after an admin clears it.

#### 4. Validation & Error Matrix
- Invalid policy id -> `400 INVALID_POLICY_ID`.
- `window_days` not in `7,30` -> `400 INVALID_WINDOW_DAYS`.
- Empty tiers -> `400 EMPTY_TIERS`.
- Duplicate tier threshold -> `400 DUPLICATE_TIER_THRESHOLD`.
- Tier multiplier `<= 0`, NaN, or infinity -> `400 INVALID_TIER_RATE`.
- Duplicate enabled policy for a target group -> repository must surface the database unique constraint as an error/409-style conflict where applicable.
- Policy already running -> `409 POLICY_ALREADY_RUNNING`.
- Deleting a policy with real automatic assignments -> `400 POLICY_HAS_ASSIGNMENTS`; pure manual-skip takeover records should not block deletion.
- `POST /:id/clear` while another run is active -> `409 POLICY_ALREADY_RUNNING`.

#### 5. Good/Base/Bad Cases
- Good: a user with 30-day usage above the highest threshold receives only the target group-specific rate multiplier, and preview shows the same change without writing any rows.
- Good: run history renders summary counts first and fetches paginated user-level changes only when the admin expands one run.
- Base: a user below the lowest tier and managed by the policy is cleared; `rpm_override` and unrelated groups remain untouched.
- Base: an admin clears a policy that contains manual takeover records; the policy assignment is removed so deletion can proceed, while the manually edited multiplier remains unchanged.
- Bad: a first-time policy run overwrites an existing manual group rate under `manual_priority`.
- Bad: inserting into `user_allowed_groups(updated_at)` when the join table does not define that column.
- Bad: returning every run's `changes` from `GET /:id/runs` or linking run-change `target_group_id` to `groups(id) ON DELETE CASCADE`.

#### 6. Tests Required
- Service unit tests: defaults/validation, tier selection, downgrade, clear, explicit policy clearing, preview no-write, manual-priority skip for existing assignments, and manual-priority skip before first assignment.
- Repository or integration tests: token aggregation uses the four-token sum, `actual_cost > 0` filtering, filter predicates, one-running-run constraint, deletion blocking ignores pure manual takeover rows, run summaries omit change details, run changes are paginated and scoped to the requested policy/run, successful runs persist apply/audit/summary atomically, and grant/clear preserves unrelated group and RPM state.
- Handler/routes tests: all admin endpoints are registered under the prefix and use admin middleware.
- Frontend checks: API types match backend JSON names, page defaults match product defaults, preview groups create/update/downgrade/clear/skip results, and `pnpm typecheck` passes.

#### 7. Wrong vs Correct

Wrong:
```go
// manual_priority 下首次遇到已有手动倍率时直接覆盖。
changeType = TokenUsagePolicyChangeUpdate
```

Correct:
```go
// manual_priority 下首次已有手动倍率也视为人工接管。
changeType = TokenUsagePolicyChangeSkipManual
```

Wrong:
```sql
INSERT INTO user_allowed_groups (user_id, group_id, created_at, updated_at) VALUES (...)
```

Correct:
```sql
INSERT INTO user_allowed_groups (user_id, group_id, created_at) VALUES (...)
```

---

### Scenario: Admin delayed-apply suggestions

#### 1. Scope / Trigger
- Trigger: adding or changing an admin feature that generates persisted suggestions in one action and applies them later after explicit confirmation.
- These flows cross migrations, repository transactions, service cache invalidation, handlers, and frontend API types. They need code-spec depth because task configuration can change between generation and apply.

#### 2. Signatures
- DB run/audit table must snapshot the write scope used by the run, such as `target_group_id`.
- Suggestion rows must persist both previous and proposed values, such as `old_priority` and `new_priority`.
- Service apply method should return the applied run/audit DTO including the snapshot scope used for cache invalidation.

#### 3. Contracts
- Preview/run generation must not mutate the target configuration.
- Apply must use the run snapshot scope, not the current mutable task/policy scope.
- Apply must update only rows represented by pending suggestions from the requested run.
- Apply must be atomic: target write, suggestion audit, and run audit are committed together.
- Apply must verify the current target value still equals the suggestion's captured old value before overwriting.

#### 4. Validation & Error Matrix
- Non-success run -> `400`.
- Already applied run -> `409`.
- No pending suggestions -> `400`.
- Suggested target row missing from the run snapshot scope -> `400`.
- Current target value changed after the run -> `409`.

#### 5. Good/Base/Bad Cases
- Good: run created for group 7 still applies only group 7 even if the task is later edited to group 8.
- Good: if an admin manually changes priority after a run, applying that stale run returns conflict instead of overwriting the manual change.
- Base: a run with no changes remains viewable but cannot be applied.
- Bad: applying a run by joining the mutable task table and reading its current `target_group_id`.

#### 6. Tests Required
- Repository test: apply uses run snapshot scope and updates only suggestion accounts.
- Repository test: stale current value returns conflict and rolls back.
- Service test: cache invalidation uses the run snapshot scope returned by apply.
- Cross-layer check: frontend API type includes the snapshot scope field returned by backend JSON.

#### 7. Wrong vs Correct

Wrong:
```sql
SELECT t.target_group_id FROM tasks t JOIN runs r ON r.task_id = t.id
```

Correct:
```sql
SELECT r.target_group_id FROM runs r WHERE r.id = $1
```

---

### Scenario: Admin upstream relay group monitoring

#### 1. Scope / Trigger
- Trigger: adding or changing the admin upstream relay group monitoring feature that lists connectors, candidate mappings, group-rate snapshots, probes, or priority suggestions.
- This feature crosses remote upstream credentials, remote user usage snapshots, repository SQL, service DTOs, admin handlers, frontend API types, and the management UI. It needs code-spec depth because connector-account state and candidate-mapping usage can look similar but mean different things.

#### 2. Signatures
- Connector DTO: `UpstreamRelayConnector` may expose `upstream_account_balance` and `upstream_account_balance_checked_at`.
- Candidate DTO: `UpstreamRelayCandidate` must expose nullable `today_actual_cost`, `today_total_tokens`, and `today_usage_checked_at`, and must not expose connector account balance fields.
- Candidate list SQL: read today's usage snapshot from `upstream_relay_group_rate_snapshots` by `connector_id = candidate.connector_id` and `upstream_group_id = candidate.upstream_group_id`; it must not aggregate `usage_logs` inline while listing candidates.
- Full connector sync may aggregate upstream real usage from the upstream Sub2API ordinary-user usage records endpoint (`GET /api/v1/usage`) using the connector's upstream login/session. Lightweight metrics refresh may refresh connector balance and candidate usage only when each candidate has an explicit upstream API key binding.
- Lightweight metrics refresh endpoint: `POST /api/v1/admin/upstream-relay-group-monitors/connectors/:id/metrics/refresh`.
- Token usage expression: `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`.
- Cost expression: `actual_cost`.

#### 3. Contracts
- Connector rows represent the upstream relay login account; show upstream account balance only on connector surfaces.
- Candidate rows represent an account-scoped local mapping to an upstream group; show today's usage snapshot for `connector_id + upstream_group_id`, not upstream account balance.
- Candidate rows own the upstream API key mapping (`upstream_api_key_id` plus display-only name/masked key); do not store this connector-specific mapping in `accounts.credentials`.
- Candidate usage window follows the local configured timezone day when refreshed from local statistics; full sync may still follow the date sent to the upstream ordinary-user `/usage` endpoint.
- Full connector sync may refresh group visibility/rates and upsert snapshots. Lightweight metrics refresh must not update snapshot today-usage fields from local logs; it may update them from upstream `/api/v1/usage/stats` using candidate `upstream_api_key_id`.
- Lightweight metrics refresh must not call upstream `/groups/available`, upstream `/groups/rates`, `UpsertSnapshots`, snapshot-change insertion, or stale snapshot marking.
- When lightweight usage refresh succeeds, existing snapshots with no matching usage record for today should get zero usage; when usage refresh fails or a candidate lacks explicit upstream API key binding, preserve existing snapshot usage values and report a sanitized `usage_error`.
- If no snapshot exists for a connector/group, lightweight refresh cannot create it; an admin must run one full sync first.
- Missing, incomplete, unauthorized, or too-large upstream usage pagination during full sync must leave `today_actual_cost`, `today_total_tokens`, and `today_usage_checked_at` null. Lightweight metrics refresh must preserve existing snapshot usage values when usage refresh is unavailable. Unknown usage must not be coerced to zero.
- Frontend field names must stay aligned with backend JSON: `today_actual_cost`, `today_total_tokens`, `today_usage_checked_at`, `upstream_account_balance`, `upstream_account_balance_checked_at`, `upstream_api_key_id`, `upstream_api_key_name`, `upstream_api_key_masked`.

#### 4. Validation & Error Matrix
- Missing connector profile balance -> connector balance fields are null; candidate usage is independent.
- No reliable usage snapshot for candidate connector/group -> candidate usage fields are null, not zero.
- Candidate account changes -> upstream usage snapshot remains keyed by `connector_id + upstream_group_id`; local account priority does not affect candidate usage.
- Remote upstream profile failure during sync -> do not fail rate snapshot sync solely because balance refresh failed.
- Lightweight metrics refresh account-scoped usage limitation -> return a successful refresh result when balance refresh succeeds, include a sanitized `usage_error`, and preserve candidate usage fields for that connector.
- Lightweight metrics refresh on a connector with no snapshots -> balance may refresh, but candidate usage remains unavailable until a full sync creates snapshots.

#### 5. Good/Base/Bad Cases
- Good: the connector tab shows `$12.34` synced at a timestamp for the upstream relay account.
- Good: the candidate tab shows today's locally aggregated `actual_cost` and four-token total for `connector_id + upstream_group_id`.
- Good: clicking "Refresh Usage / Balance" refreshes connector balance without changing group-rate snapshots, candidate usage, or producing rate-change history.
- Base: a candidate with unavailable upstream usage displays a clear not-synced state.
- Bad: returning `upstream_account_balance` from `UpstreamRelayCandidate`.
- Bad: aggregating `usage_logs` directly in the candidate list query instead of refreshing snapshot fields first.
- Bad: calling upstream admin dashboard APIs for candidate usage when only ordinary upstream user credentials are available.

#### 6. Tests Required
- Repository test: candidate select query does not include `upstream_account_balance`.
- Repository test: candidate select query does not aggregate local `usage_logs`.
- Service test: upstream ordinary-user `/usage` pages aggregate by `group_id`, `actual_cost`, and the four-token sum.
- Service test: lightweight metrics refresh does not call upstream `/api/v1/usage`, full snapshot sync, `UpsertSnapshots`, or stale marking behavior.
- Repository test: candidate creation/update no longer joins `account_groups` or requires `target_group_id`.
- Repository test: recommendation apply updates `accounts.priority`, not `account_groups.priority`.
- Service test: incomplete usage records and pagination overflow clear today usage and report usage unavailable.
- Repository scan test: nullable `today_actual_cost`, `today_total_tokens`, and `today_usage_checked_at` map to the candidate DTO in the correct scan order.
- Repository test: lightweight today-usage update sets zero for existing snapshots absent from a successful upstream usage page, and sets null on unavailable usage.
- Frontend check: candidate API type excludes connector balance fields and includes today's usage fields; connector API type includes balance fields.
- Frontend check: candidate table renders the usage column, connector table renders account balance, and connector row exposes a lightweight refresh button.

#### 7. Wrong vs Correct

Wrong:
```go
type UpstreamRelayCandidate struct {
    UpstreamAccountBalance *float64 `json:"upstream_account_balance,omitempty"`
}
```

Correct:
```go
type UpstreamRelayCandidate struct {
    TodayActualCost     *float64  `json:"today_actual_cost,omitempty"`
    TodayTotalTokens    *int64    `json:"today_total_tokens,omitempty"`
    TodayUsageCheckedAt *time.Time `json:"today_usage_checked_at,omitempty"`
}
```

Wrong:
```sql
SELECT SUM(actual_cost) FROM usage_logs WHERE account_id = c.account_id
```

Correct:
```sql
SELECT s.today_actual_cost, s.today_total_tokens, s.today_usage_checked_at
FROM upstream_relay_group_rate_snapshots s
WHERE s.connector_id = c.connector_id AND s.upstream_group_id = c.upstream_group_id
```

---

### Scenario: Admin upstream relay recommendation policy preview

#### 1. Scope / Trigger
- Trigger: adding or changing global recommendation policy or preview behavior for admin upstream relay group monitoring.
- This feature crosses migration, repository, service ranking logic, admin handlers, frontend API types, and UI controls. It needs code-spec depth because preview must be a no-write simulation while saved policy must drive persisted recommendation runs.

#### 2. Signatures
- Route prefix: `/api/v1/admin/upstream-relay-group-monitors`.
- Policy endpoints: `GET /recommendation-policy`, `PUT /recommendation-policy`.
- Preview endpoint: `POST /recommendations/preview`.
- Persisted run endpoint remains: `POST /recommendations`.
- DB singleton table: `upstream_relay_recommendation_policy` with `id = 1`.
- Policy fields: `snapshot_freshness_minutes`, `usage_delta_freshness_minutes`, `probe_freshness_minutes`, `min_success_rate`, `min_sample_size`, `exclude_consecutive_failures`, `priority_start`, `priority_step`, `sort_fields`.
- Supported `sort_fields`: `rate_asc`, `success_rate_desc`, `latency_asc`.

#### 3. Contracts
- Missing policy row returns the default policy rather than an error.
- Default policy must stay equivalent or close to the previous hard-coded recommendation behavior.
- `POST /recommendations/preview` accepts an optional policy payload; when omitted, it uses the saved/default global policy.
- Preview returns `suggestions` and `exclusions` with reason codes and human-readable reasons.
- Preview must not insert into `upstream_relay_recommendation_runs` or `upstream_relay_recommendation_suggestions`.
- Preview must not update `accounts.priority` or any account scheduling configuration.
- `POST /recommendations` must read the saved/default global policy and persist a normal recommendation run using the computed suggestions.

#### 4. Validation & Error Matrix
- Freshness windows `<= 0` -> `400 UPSTREAM_RELAY_INVALID_POLICY_FRESHNESS`.
- `min_success_rate < 0`, `> 1`, NaN, or infinity -> `400 UPSTREAM_RELAY_INVALID_POLICY_SUCCESS_RATE`.
- `min_sample_size < 1` -> `400 UPSTREAM_RELAY_INVALID_POLICY_SAMPLE_SIZE`.
- `priority_step < 1` -> `400 UPSTREAM_RELAY_INVALID_POLICY_PRIORITY_STEP`.
- Empty `sort_fields` -> default sort order is used.
- Duplicate or unsupported `sort_fields` -> `400 UPSTREAM_RELAY_INVALID_POLICY_SORT_FIELDS`.
- Missing auth subject on policy update or persisted run generation -> `401 User not authenticated`.

#### 5. Good/Base/Bad Cases
- Good: admin previews an unsaved policy and sees both recommended priority changes and excluded candidates without creating a recommendation run.
- Good: after saving `priority_start=100` and `priority_step=5`, a persisted recommendation run uses priorities `100, 105, ...`.
- Base: no saved policy row returns default thresholds and sort fields.
- Bad: preview calls `CreateRecommendationRun`.
- Bad: preview silently drops excluded candidates without explaining why.
- Bad: frontend lets duplicate sort fields through and relies on backend rejection for a normal control flow.

#### 6. Tests Required
- Service test: preview does not call repository run-creation or apply methods.
- Service test: persisted generation uses saved policy for priority start, step, filtering, and sort order.
- Service test: validation rejects invalid freshness, success rate, sample size, priority step, and duplicate/unsupported sort fields.
- Repository test: singleton policy upsert preserves `id=1`, `updated_by`, and array `sort_fields`.
- Handler/routes test: policy and preview routes are registered and update requires admin auth.
- Frontend API test: preview endpoint path differs from persisted generate endpoint path.

#### 7. Wrong vs Correct

Wrong:
```go
preview := buildPreview(candidates, policy)
return repo.CreateRecommendationRun(ctx, run, preview.Suggestions)
```

Correct:
```go
preview := buildPreview(candidates, policy)
return &preview, nil
```

Wrong:
```typescript
sort_fields: ['rate_asc', 'rate_asc']
```

Correct:
```typescript
sort_fields: normalizeSortFields(policyForm.sort_fields)
```

---

### Scenario: Conversation capture export quality gate

#### 1. Scope / Trigger
- Trigger: adding or changing conversation capture, conversation history filtering, or training JSONL export behavior.
- This flow crosses capture parsing, service quality assessment, repository session summaries, admin handlers, and frontend filters. It needs code-spec depth because an unsafe export can leak malformed or structurally incomplete training data.

#### 2. Signatures
- Service quality gate: `AssessConversationTurnQuality(record ConversationTurnRecord) ConversationQualityAssessment`.
- Service export gate: `CanExportConversationTurn(turn ConversationTurn, req ConversationExportMessagesJSONLRequest) bool`.
- Repository export query: `ListExportableTurns(ctx, req)` may pre-filter candidates, but service-level `CanExportConversationTurn` remains the final gate.
- Session summary recalculation must aggregate turn-level `quality_status`, `quality_errors`, and `exportable` after turn writes or manual move/split/merge operations.

#### 3. Contracts
- Only `quality_status = clean` turns may default to `exportable = true`.
- `needs_review` and `rejected` turns must default to `exportable = false`.
- Hard export blockers are: `exportable=false`, `parse_status != success`, `truncated=true`, `client_disconnect=true`, or non-clean turn quality.
- Heuristic sessions are excluded by default; `include_heuristic=true` only allows them to continue through the remaining turn-level gate.
- `Tools` is not equivalent to a valid tool result. Empty assistant output may be exempted only when the tool chain contains a matching non-empty tool result.
- Session-level quality is a filter and summary contract, not the export authority. Export authority stays turn-level.
- Session quality aggregation priority is `rejected > needs_review > clean`; unknown or unchecked turn quality is conservative and must not make a session clean.
- Turn-level manual quality or exportable updates must recalculate the parent session summary in the same transaction.
- Bulk quality updates must normalize `needs_review` and `rejected` the same way as single quality updates; they must not leave those statuses exportable.
- Session-level export enablement may only cascade `exportable=true` to clean turns that also pass hard structural gates. Disabling session export may clear all child turns.

#### 4. Validation & Error Matrix
- Parse failure -> `rejected` with `parse_failed`.
- Truncated payload -> `rejected` with `truncated_payload`.
- Client disconnect -> `rejected` with `client_disconnect`.
- Missing request messages -> `rejected` with `missing_request_messages`.
- Missing response messages -> `rejected` with `missing_response_messages`.
- Heuristic session -> `needs_review` with `heuristic_session`.
- Tool call without matching result -> `needs_review` with `incomplete_tool_call_chain`.
- Tool result without matching call -> `needs_review` with `orphan_tool_result`.
- Empty assistant output without a complete valid tool-result chain -> `needs_review` with `empty_assistant_output`.

#### 5. Good/Base/Bad Cases
- Good: a parsed, complete user/assistant text turn in an explicit or responses session becomes `clean` and exportable.
- Good: a Responses turn with `function_call` plus matching non-empty `function_call_output` can pass the empty-assistant check when all other gates pass.
- Base: a heuristic session remains `needs_review` until manual action; export still requires `include_heuristic=true` and all other gates.
- Bad: treating any item in `Tools` as proof of valid tool output.
- Bad: using only session `quality_status=clean` to export turns without rechecking each turn.

#### 6. Tests Required
- Unit test `AssessConversationTurnQuality` for clean text, hard rejects, heuristic review, missing messages, empty assistant output, tool call without result, orphan tool result, and complete tool chains.
- Export tests for both synchronous JSONL and background export payload builders proving hard blockers still fail after manual `exportable=true`.
- Repository tests proving session summaries aggregate turn quality and clean filters still leave turn-level hard gates in place.
- Frontend tests proving `quality_status` filter is sent and `quality_errors` are visible.

#### 7. Wrong vs Correct

Wrong:
```go
if len(input.Tools) > 0 { skipEmptyAssistantReview() }
```

Correct:
```go
if toolChain.Complete && toolChain.HasNonEmptyResult { skipEmptyAssistantReview() }
```

Wrong:
```go
turns := repo.ListExportableTurns(ctx, req) // trusted as final export set
```

Correct:
```go
turns := FilterConversationExportableTurns(repoTurns, req) // final service gate
```

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
