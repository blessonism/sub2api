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
- Required endpoints: `GET /`, `POST /`, `GET /:id`, `PUT /:id`, `DELETE /:id`, `POST /:id/preview`, `POST /:id/run`, `GET /:id/runs`, `GET /:id/runs/:run_id/changes`.
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

#### 4. Validation & Error Matrix
- Invalid policy id -> `400 INVALID_POLICY_ID`.
- `window_days` not in `7,30` -> `400 INVALID_WINDOW_DAYS`.
- Empty tiers -> `400 EMPTY_TIERS`.
- Duplicate tier threshold -> `400 DUPLICATE_TIER_THRESHOLD`.
- Tier multiplier `<= 0`, NaN, or infinity -> `400 INVALID_TIER_RATE`.
- Duplicate enabled policy for a target group -> repository must surface the database unique constraint as an error/409-style conflict where applicable.
- Policy already running -> `409 POLICY_ALREADY_RUNNING`.
- Deleting a policy with real automatic assignments -> `400 POLICY_HAS_ASSIGNMENTS`; pure manual-skip takeover records should not block deletion.

#### 5. Good/Base/Bad Cases
- Good: a user with 30-day usage above the highest threshold receives only the target group-specific rate multiplier, and preview shows the same change without writing any rows.
- Good: run history renders summary counts first and fetches paginated user-level changes only when the admin expands one run.
- Base: a user below the lowest tier and managed by the policy is cleared; `rpm_override` and unrelated groups remain untouched.
- Bad: a first-time policy run overwrites an existing manual group rate under `manual_priority`.
- Bad: inserting into `user_allowed_groups(updated_at)` when the join table does not define that column.
- Bad: returning every run's `changes` from `GET /:id/runs` or linking run-change `target_group_id` to `groups(id) ON DELETE CASCADE`.

#### 6. Tests Required
- Service unit tests: defaults/validation, tier selection, downgrade, clear, preview no-write, manual-priority skip for existing assignments, and manual-priority skip before first assignment.
- Repository or integration tests: token aggregation uses the four-token sum, `actual_cost > 0` filtering, filter predicates, one-running-run constraint, run summaries omit change details, run changes are paginated and scoped to the requested policy/run, successful runs persist apply/audit/summary atomically, and grant/clear preserves unrelated group and RPM state.
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

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
