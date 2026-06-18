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

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
