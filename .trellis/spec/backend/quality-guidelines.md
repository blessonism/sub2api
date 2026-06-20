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

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
