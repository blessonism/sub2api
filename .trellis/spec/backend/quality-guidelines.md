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

### Scenario: Campaign rank rewards with vacant places

#### Scope / Trigger
- Trigger: changing `calculateCampaignRewards` or campaign rank-weight validation.

#### Contracts
- `rank_weights` describes the relative weights of configured places and totals 100 across `rank_reward_count`.
- When fewer users rank than `rank_reward_count`, normalize only the occupied places' weights so vacant places do not reserve reward money.
- When all configured places are occupied, payouts stay equal to `rank_pool × rank_weight ÷ 100`.
- Keep integer-cent flooring and record only the resulting cent-level remainder in `rounding_residual_cents`.

#### Tests Required
- Cover a partially filled leaderboard and assert rank rewards plus the flooring remainder equal the rank pool.
- Preserve a full-leaderboard case so normalization cannot change established percentages.

### Scenario: Admin affiliate invite leaderboard

#### 1. Scope / Trigger
- Trigger: changing the admin-only global affiliate leaderboard or its lifetime recharge metrics.

#### 2. Signatures
- Endpoint: `GET /api/v1/admin/affiliates/leaderboard`.
- Query: `page`, `page_size`, `search`.
- Response row: `rank`, `user_id`, `email`, `username`, `aff_code`, `invite_count`, `all_credit_amount`, `payment_redeem_amount`.

#### 3. Contracts
- Rank by `user_affiliates.aff_count DESC`, then `payment_redeem_amount DESC`, then `user_id ASC`.
- `all_credit_amount` sums invited users' `users.total_recharged`.
- `payment_redeem_amount` sums positive, used balance redeem codes. Payment fulfillment already creates such a code, so never add payment orders again.
- Compute the global rank before applying search; filtered results retain their original rank.

#### 4. Validation & Error Matrix
- Invalid or missing pagination -> existing defaults (`page=1`, `page_size=20`).
- `page_size > 100` -> clamp to 100 at the handler boundary.
- Repository failure -> return through `response.ErrorFrom`.

#### 5. Good/Base/Bad Cases
- Good: payment-generated and standalone balance codes are each counted once.
- Base: inviters with `aff_count=0` and soft-deleted inviters do not appear.
- Bad: joining invitees and redeem codes before aggregation, which multiplies both totals.

#### 6. Tests Required
- Assert separate credit/redeem aggregates, stable ranking, retained global rank after search, pagination arguments, and response field alignment with frontend types.

#### 7. Wrong vs Correct

Wrong:
```sql
SUM(payment_orders.amount) + SUM(redeem_codes.value)
```

Correct:
```sql
SUM(redeem_codes.value) FILTER (WHERE status = 'used' AND type = 'balance' AND value > 0)
```

### Scenario: Usage log schema columns across downstream merges

#### 1. Scope / Trigger
- Trigger: adding, removing, reordering, or merging fields persisted in `usage_logs`.

#### 2. Signatures
- Source schema: `backend/ent/schema/usage_log.go`.
- Insert contract: `usageLogInsertArgTypes`, insert column lists, placeholders, and `prepareUsageLogInsert` in `usage_log_repo_insert.go`.
- Read contract: `usageLogSelectColumns` and `scanUsageLog` in `usage_log_repo_query.go`.

#### 3. Contracts
- Insert columns, argument types, prepared arguments, and static placeholders must have the same count and order.
- Select columns, scan destinations, and `service.UsageLog` assignments must have the same count and order.
- After merging schema fields, regenerate Ent with `go generate ./ent`; do not hand-edit generated files.

#### 4. Validation & Error Matrix
- Insert count/order mismatch -> fail repository tests before commit; never rely on PostgreSQL to expose it in production.
- Select/scan mismatch -> fail scan fixture tests with the exact destination count.
- Ent output differs after regeneration -> schema merge is incomplete.

#### 5. Good/Base/Bad Cases
- Good: parallel fields such as `visible_rate_multiplier` and `long_context_billing_applied` survive as an ordered union through schema, insert, query, service, DTO, and frontend types.
- Base: nullable historical fields scan to `nil`; boolean fields use their schema default.
- Bad: appending a column and argument while leaving a static query ending at the previous placeholder number.

#### 6. Tests Required
- Assert `len(prepareUsageLogInsert(log).args) == len(usageLogInsertArgTypes)`.
- Cover single and best-effort inserts plus a scan fixture containing every persisted field.
- Run `go generate ./ent` twice and require the second run to leave no diff.

#### 7. Wrong vs Correct

Wrong:
```go
// 55 columns and args, but the query still ends at $54.
VALUES ($1, /* ... */ $54)
```

Correct:
```go
VALUES ($1, /* ... */ $54, $55)
```

### Scenario: Admin global user concurrency floor

#### 1. Scope / Trigger
- Trigger: changing the admin operation that raises the minimum concurrency of all users.

#### 2. Signatures
- Endpoint: `POST /api/v1/admin/users/batch-concurrency`.
- Floor payload: `{"all":true,"concurrency":<positive integer>,"mode":"floor"}`.
- Repository operation: `BatchRaiseConcurrencyFloor(ctx, userIDs, value)`.

#### 3. Contracts
- `all=true` covers every non-soft-deleted account, including disabled users and admins.
- Floor mode only raises users below the requested value; equal or higher values remain unchanged.
- The response keeps the existing `{"affected": number}` shape and counts only rows actually raised.
- Existing `set` and `add` modes keep their current behavior.

#### 4. Validation & Error Matrix
- Floor value `< 1` -> `400 concurrency must be at least 1 in floor mode`.
- Missing target users with `all=false` -> existing `400 user_ids is required unless all=true`.
- Empty user set or no value below the floor -> success with `affected: 0`.

#### 5. Good/Base/Bad Cases
- Good: floor `5` changes concurrency `2` to `5` while concurrency `8` remains `8`.
- Base: every user is already at least `5`, so no row changes and `affected` is zero.
- Bad: read current values and later overwrite selected rows with `5`, which can lower a concurrent update to `8`.

#### 6. Tests Required
- Repository SQL test asserts atomic `GREATEST(concurrency, $1)` plus `concurrency < $1` and soft-delete filtering.
- Handler test covers active, disabled, and admin users for `all=true`, plus invalid floor rejection.
- Frontend API/view tests cover the payload, integer validation, success count, and list refresh.

#### 7. Wrong vs Correct

Wrong:
```sql
UPDATE users SET concurrency = $1 WHERE id = ANY($2)
```

Correct:
```sql
UPDATE users SET concurrency = GREATEST(concurrency, $1)
WHERE id = ANY($2) AND deleted_at IS NULL AND concurrency < $1
```

### Scenario: Admin all-user balance reduction

#### 1. Scope / Trigger
- Trigger: changing the admin operation that divides every non-soft-deleted user's wallet balance by one factor.

#### 2. Signatures
- Preview endpoint: `POST /api/v1/admin/users/balance-reduction/preview`.
- Execute endpoint: `POST /api/v1/admin/users/balance-reduction`, with the required `Idempotency-Key` header.
- Request: `{"factor":"<decimal string>"}`; summary fields are `operation_id`, `factor`, `user_count`, `affected_users`, `current_total`, `reduced_total`, and `reduction_total`.

#### 3. Contracts
- `factor` stays a decimal string across the API and repository boundary; it is greater than `1`, has at most 8 fractional digits, and is cast to PostgreSQL `numeric` for calculation.
- Both endpoints cover exactly `users.deleted_at IS NULL`, including administrators and disabled users, and change only `users.balance`.
- Preview is informational. Execution recalculates from locked current rows and stores `GREATEST(ROUND(old_balance / factor, 8), 0)`.
- Balance updates and one `admin_balance` redeem-code delta per changed user occur in the same SQL statement. Audit notes contain the shared `operation_id` and factor.
- Replaying the same idempotency key and request returns the stored result without dividing balances again; reusing the key with a different request conflicts.

#### 4. Validation & Error Matrix
- Empty, non-decimal, non-finite, or more than 8 fractional digits -> `400 invalid factor`.
- `factor <= 1` -> `400 factor must be greater than 1`.
- Missing execution idempotency key -> existing admin idempotency validation error.
- SQL update or audit failure -> return an error with no balance or audit row committed.
- No eligible users or no changed balances -> success with zero affected users.

#### 5. Good/Base/Bad Cases
- Good: factor `2.5` divides active, disabled, and administrator balances using database decimal arithmetic, then records each non-zero delta.
- Base: zero balances stay zero and do not create zero-value audit rows.
- Bad: applying previewed values during execution, which overwrites intervening consumption or recharge.
- Bad: paging through users or calling the single-user balance endpoint repeatedly, which permits partial completion.

#### 6. Tests Required
- Service tests cover decimal normalization, every rejection class, preview forwarding, operation ID/audit notes, cache invalidation, and idempotent response replay.
- Repository SQL tests assert the soft-delete filter, ordered `FOR UPDATE`, numeric division with `GREATEST`/`ROUND`, and `admin_balance` insertion in the same CTE statement.
- Frontend API/view tests cover string payloads, client validation, preview confirmation, stable retry key, actual-result messaging, and list refresh.

#### 7. Wrong vs Correct

Wrong:
```go
for _, user := range preview.Users {
    setBalance(user.ID, user.PreviewedBalance)
}
```

Correct:
```sql
WITH targets AS (SELECT id, balance FROM users WHERE deleted_at IS NULL FOR UPDATE)
UPDATE users SET balance = GREATEST(ROUND(targets.balance / $1::numeric, 8), 0) FROM targets WHERE users.id = targets.id;
```

### Scenario: Campaign historical invite weighting

#### 1. Scope / Trigger
- Trigger: changing invite-campaign qualification, historical recharge attribution, leaderboard invite counts, or reward settlement inputs.
- This flow crosses affiliate relationships, payment/redeem records, campaign snapshots, public/admin DTOs, and settlement; source duplication or integer coercion directly changes rewards.

#### 2. Signatures
- Config DB field: `campaign_config_versions.historical_invite_ratio NUMERIC(12,8)`, internal range `0..1`.
- Snapshot DB table: `campaign_historical_invite_snapshots`, unique on `(campaign_id, invitee_user_id)`.
- Public leaderboard fields: `activity_valid_invite_count`, `historical_valid_invite_count`, `historical_weighted_invite_count`, and fractional `valid_invite_count`.

#### 3. Contracts
- Current-campaign invites count as `1` only after registration and qualifying recharge within `[start_at, end_at)`.
- Historical invites require an affiliate relationship and qualifying cumulative balance recharge strictly before `start_at`.
- Completed balance payment orders and used balance redeem codes are eligible sources. A redeem code referenced by `payment_orders.recharge_code` must not also be counted as a standalone redeem source.
- Historical snapshot membership, qualification amounts, and cutoff are immutable after activation and never inject the current campaign reward pool. The ratio snapshot may change only before freeze/final settlement, in the same transaction as the campaign edit.
- Leaderboard, distance calculations, frozen snapshots, and settlement must preserve fractional counts.

#### 4. Validation & Error Matrix
- Ratio `< 0` or `> 1` -> `CAMPAIGN_INVALID_CONFIG`.
- Missing or unpublished config -> do not create a historical snapshot.
- Snapshot transaction failure -> return the error and do not write the completion marker.
- Existing final settlement or payout batch -> reject ratio changes with `CAMPAIGN_SETTLEMENT_LOCKED`.
- Duplicate snapshot execution -> succeed without adding rows.

#### 5. Good/Base/Bad Cases
- Good: `3` campaign invites plus `5` historical invites at `0.30` produces `4.5` effective invites.
- Base: ratio `0` keeps existing campaign ranking and settlement behavior.
- Bad: summing a completed payment order and the redeem code generated for that same order.
- Bad: casting weighted invite counts to integer before ranking or settlement.

#### 6. Tests Required
- Repository: source de-duplication SQL, strict start-time cutoff, qualification threshold, unique invitee, transaction completion marker, and repeat-call idempotency.
- Service: ratio validation, fractional ordering, contribution weight, distance, freeze, and final settlement.
- Frontend: percentage normalization (`30% -> 0.30`), payload contract, fractional display, and historical breakdown.

#### 7. Wrong vs Correct

Wrong:
```sql
SELECT value FROM redeem_codes;
```

Correct:
```sql
SELECT value FROM redeem_codes rc
WHERE NOT EXISTS (SELECT 1 FROM payment_orders po WHERE po.recharge_code = rc.code);
```

### Scenario: Durable announcement email broadcasts

#### 1. Scope / Trigger
- Trigger: changing administrator announcement email broadcast creation, recipient selection, delivery processing, retry, or progress APIs.

#### 2. Signatures
- Management APIs: `GET|POST /api/v1/admin/announcements/:id/email-broadcast`, `GET .../deliveries`, and `POST .../retry-failed`.
- Persistence: `announcement_email_broadcasts` owns one immutable message snapshot per announcement; `announcement_email_deliveries` owns one recipient snapshot per user.
- Worker entrypoint: `AnnouncementEmailBroadcastRepository.ClaimNext(ctx, leaseUntil)`.

#### 3. Contracts
- Only an announcement active at server time can create a broadcast, and `announcement_id` is unique across broadcasts.
- Recipients are active, non-deleted users with valid non-reserved email addresses who match `AnnouncementTargeting` using non-expired active subscriptions.
- Sent recipients are immutable. Retry changes only `failed` deliveries back to `pending` and preserves `attempt_count`.
- The worker claims rows with `FOR UPDATE SKIP LOCKED`; result writes update the delivery and broadcast counters in one transaction.
- Existing broadcasts block hard deletion of their announcement. User deletion keeps the email snapshot and clears only `user_id`.

#### 4. Validation & Error Matrix
- Inactive announcement -> `ANNOUNCEMENT_EMAIL_NOT_ACTIVE`.
- Existing broadcast -> `ANNOUNCEMENT_EMAIL_BROADCAST_EXISTS`.
- No eligible recipients -> `ANNOUNCEMENT_EMAIL_NO_RECIPIENTS`.
- More than 10,000 recipients -> `ANNOUNCEMENT_EMAIL_TOO_MANY_RECIPIENTS`.
- Retry without failed deliveries -> `ANNOUNCEMENT_EMAIL_NO_FAILURES`.
- Delete after broadcast creation -> `ANNOUNCEMENT_EMAIL_BROADCAST_DELETE_BLOCKED`.

#### 5. Good/Base/Bad Cases
- Good: an expired processing lease is reclaimed, increments `attempt_count`, and reaches one terminal counter update.
- Base: a completed broadcast returns progress and delivery history without recalculating recipients.
- Bad: loading subscriptions with `status='active'` but not checking `expires_at`, which emails users whose announcement is no longer visible.

#### 6. Tests Required
- Service: reserved/invalid email exclusion and safe Markdown rendering without raw HTML.
- Repository integration: create, claim, expired-lease recovery, failure, retry, success counters, migration shape, and delete restriction.
- API/frontend: all four paths, send confirmation, running-state polling, failed retry, zh/en keys, and type alignment.

#### 7. Wrong vs Correct

Wrong:
```go
if subscription.Status == SubscriptionStatusActive { groups[subscription.GroupID] = struct{}{} }
```

Correct:
```go
if subscription.Status == SubscriptionStatusActive && subscription.ExpiresAt.After(now) { groups[subscription.GroupID] = struct{}{} }
```

### Scenario: Versioned authentication cache snapshots

#### 1. Scope / Trigger
- Trigger: adding, removing, or changing fields serialized in `APIKeyAuthSnapshot` or its nested user/group snapshots.

#### 2. Contracts
- Every serialized schema change must increment `apiKeyAuthSnapshotVersion`, so an older L1/L2 entry cannot be accepted with silently missing authorization or billing fields.
- When merging branches that independently used the same next version for different fields, the combined schema must advance to a new version rather than keeping either branch's number.
- Snapshot construction and restoration must carry the same field set in both directions.

#### 3. Tests Required
- Snapshot round-trip tests cover new fields.
- Cache lookup tests reject entries whose version predates the combined schema.

### Scenario: Persistent lottery campaign visibility and winner disclosure

#### 1. Scope / Trigger
- Trigger: changing lottery campaign feature selection, public visibility, entry availability, or winner list APIs.

#### 2. Signatures
- Persistent marker: `lottery_campaigns.is_featured` (at most one row is selected by `SetFeaturedLotteryCampaign`).
- Public selection: `GET /api/v1/lottery-campaigns/active`.
- Public winners: `GET /api/v1/lottery-campaigns/:id/winners`.
- Admin winners: `GET /api/v1/admin/lottery-campaigns/:id/winners`, with the existing batch-scoped route retained.

#### 3. Contracts
- A published featured campaign remains publicly readable outside its start/end window and takes priority over newer active campaigns until an admin changes the featured selection.
- Public readability and entry availability are separate decisions. Featured status never reopens enrollment before `start_at` or after `end_at` / the draw window.
- Public winner DTOs contain only `masked_email`, `prize_name`, `reward_amount_cents`, `entry_date`, `is_current_round`, and `created_at`, and only successful winners.
- `LotteryMyData.round_completed` comes from the current/final draw batch terminal status, so rounds with zero winners can still present an unambiguous completed state.
- Daily winner history must not complete or highlight the next round. The service marks `is_current_round` against the server-timezone result date; frontend round UI filters on that field.
- Admin winner DTOs may include operational identifiers and payout status but must stay behind admin routes.

#### 4. Validation & Error Matrix
- Feature a non-published campaign -> `LOTTERY_FEATURED_INVALID`.
- Read a draft/cancelled/archived or ended non-featured campaign through public detail/winner APIs -> `LOTTERY_CAMPAIGN_NOT_FOUND`.
- Enroll in a featured campaign outside its entry window -> `LOTTERY_ENTRIES_CLOSED`.

#### 5. Good/Base/Bad Cases
- Good: an ended published featured campaign shows persisted masked winners while all entry writes remain closed.
- Base: when no campaign is featured, the active endpoint returns the normal published campaign inside its time window.
- Bad: implementing entry availability by calling a visibility helper that treats featured campaigns as timelessly visible.
- Bad: returning the admin `LotteryWinner` DTO from a public handler.

#### 6. Tests Required
- Repository: featured campaign is selected even after `end_at` and wins priority ordering.
- Service: ended featured winner data is readable, no entry upsert occurs, and non-published campaigns cannot be featured.
- Service: daily historical winners are not marked current, and a successful zero-winner batch reports `round_completed=true`.
- API/frontend: campaign-level admin winner path, persistent action availability, and masked public winner rendering.

#### 7. Wrong vs Correct

Wrong:
```go
return lotteryCampaignVisible(campaign, now) // may be true solely because is_featured=true
```

Correct:
```go
return campaign.Status == LotteryStatusPublished && lotteryCampaignInWindow(campaign, now)
```

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

### Scenario: Admin user list usage sorting

#### 1. Scope / Trigger
- Trigger: adding or changing usage-related sort keys for the admin user list.
- This flow crosses frontend sort controls, admin user list query parameters, repository SQL aggregation, and usage statistics shown in the table.

#### 2. Signatures
- Query params: `sort_by`, `sort_order`, `page`, `page_size`.
- Supported global sort keys: `usage_today`, `usage_total`.
- Supported platform sort keys: `usage_anthropic_today`, `usage_anthropic_total`, `usage_openai_today`, `usage_openai_total`, `usage_gemini_today`, `usage_gemini_total`, `usage_antigravity_today`, `usage_antigravity_total`.
- Repository entrypoint: `UserRepository.ListWithFilters(ctx, pagination.PaginationParams, service.UserListFilters)`.

#### 3. Contracts
- Usage sorting must be applied before `OFFSET/LIMIT`; never sort only the already paginated user slice.
- `usage_today` uses the same local-day boundary as dashboard/user usage statistics.
- `usage_total` keeps the UI label but means the same rolling 30-day window used by `DashboardService.GetBatchUserUsageStats`.
- Global usage sorting must include `usage_logs.actual_cost` plus negative `admin_usage_calibrations.balance_delta` as calibration spend.
- Platform usage sorting uses `usage_logs.actual_cost` only, because balance calibrations have no platform attribution.
- Tie-break by user id in the same direction as the requested sort order to keep pagination deterministic.

#### 4. Validation & Error Matrix
- Unknown usage sort key -> fall back to the normal user-list sort handling.
- Invalid or missing `sort_order` -> use the existing pagination sort-order normalization.
- No matching usage rows or calibration rows -> aggregate as zero, not `NULL`.

#### 5. Good/Base/Bad Cases
- Good: page 1 with `sort_by=usage_total&sort_order=desc&page_size=1` returns the highest 30-day spender among all matching users.
- Base: users with no usage logs still appear in deterministic order with a zero usage value.
- Bad: fetching one page by `created_at` and then sorting that page by usage in frontend state.

#### 6. Tests Required
- Repository integration test: usage sorting happens before pagination.
- Repository integration test: global 30-day sorting includes negative balance calibration spend.
- Frontend typecheck: admin user list passes the selected usage sort key through list query state.

#### 7. Wrong vs Correct

Wrong:
```go
users = sortCurrentPageByUsage(users)
```

Correct:
```go
params.SortBy = "usage_total"
repo.ListWithFilters(ctx, params, filters)
```

---

### Scenario: Admin token usage auto policy APIs

#### 1. Scope / Trigger
- Trigger: adding or changing the admin feature that assigns user group-specific rate multipliers from rolling token usage.
- This feature crosses database migrations, repository SQL, service policy decisions, admin handlers, frontend API types, and a management UI. It requires code-spec depth because contract drift can overwrite manual billing configuration or remove group access incorrectly.

#### 2. Signatures
- Route prefix: `/api/v1/admin/token-usage-policies`.
- Required endpoints: `GET /`, `POST /`, `GET /:id`, `PUT /:id`, `DELETE /:id`, `POST /:id/preview`, `POST /:id/run`, `POST /:id/clear`, `GET /:id/runs`, `GET /:id/runs/:run_id/changes`.
- DB tables: `token_usage_auto_policies`, `token_usage_auto_policy_tiers`, `token_usage_auto_assignments`, `token_usage_auto_runs`, `token_usage_auto_run_changes`, `token_usage_auto_user_totals`.
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
- 常驻档位：`token_usage_auto_policy_tiers.is_resident = TRUE` 的档位按用户**全历史累计、全站口径**（忽略策略筛选）判断，复用 `condition_mode`（`token` / `actual_cost` / `both`）；非常驻档位继续按近 7/30 天窗口判断。
- 生效倍率：同一用户分别选出滚动命中档位与常驻命中档位，取两者 `rate_multiplier` 更低者为生效档位（`tier_id`）；倍率相等时优先记常驻档位。仅常驻命中时保留常驻倍率，**两者都不命中才清除**。
- 累计数据：`token_usage_auto_user_totals` 按用户保存全站累计值，`last_processed_id` 记录已纳入统计的最大 `usage_logs.id`；刷新 SQL 只处理 `ul.id > last_processed_id` 的增量，首见用户等价全历史计算，禁止按 `created_at` 做水位（异步落库会漏算）。
- 累计刷新在事务内用 `pg_advisory_xact_lock` 串行化，水位读取与增量累加必须原子可见，防止不同策略并发执行时重复累加同一批日志。
- 常驻档位同样遵循 `conflict_mode`（`manual_priority` 下不覆盖手动倍率）并受自动倍率封顶；assignment/run_changes 记录 `resident_tier_id` 与 `total_token_usage` / `total_actual_cost` 双口径审计字段。
- `token_usage_auto_run_changes.tier_condition_mode` / `resident_tier_condition_mode` 未命中时必须写 `NULL`，禁止写空串：CHECK 只允许 `NULL` 或 `'token'/'actual_cost'/'both'`，空串会让整次策略执行事务回滚。

#### 4. Validation & Error Matrix
- Invalid policy id -> `400 INVALID_POLICY_ID`.
- `window_days` not in `7,30` -> `400 INVALID_WINDOW_DAYS`.
- Empty tiers -> `400 EMPTY_TIERS`.
- Duplicate tier threshold（唯一键含 `is_resident`）-> `400 DUPLICATE_TIER_THRESHOLD`。
- Tier multiplier `<= 0`, NaN, or infinity -> `400 INVALID_TIER_RATE`.
- Duplicate enabled policy for a target group -> repository must surface the database unique constraint as an error/409-style conflict where applicable.
- Policy already running -> `409 POLICY_ALREADY_RUNNING`.
- Deleting a policy with real automatic assignments -> `400 POLICY_HAS_ASSIGNMENTS`; pure manual-skip takeover records should not block deletion.
- `POST /:id/clear` while another run is active -> `409 POLICY_ALREADY_RUNNING`.

#### 5. Good/Base/Bad Cases
- Good: a user with 30-day usage above the highest threshold receives only the target group-specific rate multiplier, and preview shows the same change without writing any rows.
- Good: a user with low window usage but all-time cumulative usage above the resident threshold keeps the resident rate; a lower rolling tier rate temporarily wins when window usage rises, then falls back to resident without clearing.
- Base: a managed user whose window usage drops below every rolling tier but still matches the resident tier produces no clear and keeps the resident rate.
- Bad: clearing a user when the rolling tier misses but the resident tier still matches, or letting `selectTokenUsageTier` see resident tiers and break window-only policies.
- Good: run history renders summary counts first and fetches paginated user-level changes only when the admin expands one run.
- Base: a user below the lowest tier and managed by the policy is cleared; `rpm_override` and unrelated groups remain untouched.
- Base: an admin clears a policy that contains manual takeover records; the policy assignment is removed so deletion can proceed, while the manually edited multiplier remains unchanged.
- Bad: a first-time policy run overwrites an existing manual group rate under `manual_priority`.
- Bad: inserting into `user_allowed_groups(updated_at)` when the join table does not define that column.
- Bad: returning every run's `changes` from `GET /:id/runs` or linking run-change `target_group_id` to `groups(id) ON DELETE CASCADE`.
- Bad: 把未命中档位的 `tier_condition_mode` / `resident_tier_condition_mode` 写成空串；PostgreSQL CHECK 会拒绝该行并回滚整次策略执行。

#### 6. Tests Required
- Service unit tests: defaults/validation, tier selection, downgrade, clear, explicit policy clearing, preview no-write, manual-priority skip for existing assignments, and manual-priority skip before first assignment.
- Service unit tests: resident/window selector separation, effective-rate min, resident floor retention, clear only when both dimensions miss, normalize uniqueness including `is_resident`.
- Repository tests: `RefreshUserUsageTotals` upsert delta + read-back; new tier/assignment/run_changes columns survive SELECT/SCAN round-trip.
- Repository tests: 未命中档位时 `tier_condition_mode` / `resident_tier_condition_mode` 绑定 `NULL`；命中时绑定 `token` / `actual_cost` / `both`。
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

Wrong:
```go
// 未命中常驻档位时把 Go 空串直接写入 CHECK 列。
change.ResidentTierConditionMode // ""
```

Correct:
```sql
NULLIF($18, '')  -- resident_tier_condition_mode
NULLIF($14, '')  -- tier_condition_mode
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
- Usage history endpoint: `GET /api/v1/admin/upstream-relay-group-monitors/usage-history`.
- Usage history table: `upstream_relay_group_usage_history` with one row per `usage_date + connector_id + upstream_group_id`.
- Full connector sync may aggregate upstream real usage from the upstream Sub2API ordinary-user usage records endpoint (`GET /api/v1/usage`) using the connector's upstream login/session. Lightweight metrics refresh may refresh connector balance and candidate usage only when each candidate has an explicit upstream API key binding.
- Lightweight metrics refresh endpoint: `POST /api/v1/admin/upstream-relay-group-monitors/connectors/:id/metrics/refresh`.
- Token usage expression: `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`.
- Cost expression: `actual_cost`.

#### 3. Contracts
- Connector rows represent the upstream relay login account; show upstream account balance only on connector surfaces.
- Candidate rows represent an account-scoped local mapping to an upstream group; show today's usage snapshot for `connector_id + upstream_group_id`, not upstream account balance.
- Candidate rows own the upstream API key mapping (`upstream_api_key_id` plus display-only name/masked key); do not store this connector-specific mapping in `accounts.credentials`.
- Resolve each bound Key's current `group_id` from upstream `/api/v1/keys` before aggregating today's usage. The saved candidate `upstream_group_id` is editable configuration/history and must not be the runtime source of truth after the upstream Key changes group. Historical-date finalization uses the saved candidate group because no historical Key-group API is available.
- Candidate list and recommendation DTOs should decorate the effective current Key group in memory; do not silently rewrite the saved candidate mapping. Repeated bindings of one upstream Key must contribute one usage total.
- Candidate usage window follows the local configured timezone day when refreshed from local statistics; full sync may still follow the date sent to the upstream ordinary-user `/usage` endpoint.
- Full connector sync may refresh group visibility/rates and upsert snapshots. Lightweight metrics refresh must not update snapshot today-usage fields from local logs; it may update them from upstream `/api/v1/usage/stats` using candidate `upstream_api_key_id`.
- Lightweight metrics refresh must not call upstream `/groups/available`, upstream `/groups/rates`, `UpsertSnapshots`, snapshot-change insertion, or stale snapshot marking.
- When lightweight usage refresh succeeds, existing snapshots with no matching usage record for today should get zero usage; when usage refresh fails or a candidate lacks explicit upstream API key binding, preserve existing snapshot usage values and report a sanitized `usage_error`.
- If no snapshot exists for a connector/group, lightweight refresh cannot create it; an admin must run one full sync first.
- Missing, incomplete, unauthorized, or too-large upstream usage pagination during full sync must leave `today_actual_cost`, `today_total_tokens`, and `today_usage_checked_at` null. Lightweight metrics refresh must preserve existing snapshot usage values when usage refresh is unavailable. Unknown usage must not be coerced to zero.
- Successful usage refresh must upsert daily history for existing snapshots with known usage. A successful refresh with no usage for an existing snapshot records `actual_cost=0` and `total_tokens=0`; failed or unavailable usage refresh must not create zero history rows.
- Same-day usage history refreshes must update the existing `usage_date + connector_id + upstream_group_id` row, not append per-refresh samples.
- Frontend field names must stay aligned with backend JSON: `today_actual_cost`, `today_total_tokens`, `today_usage_checked_at`, `upstream_account_balance`, `upstream_account_balance_checked_at`, `upstream_api_key_id`, `upstream_api_key_name`, `upstream_api_key_masked`.

#### 4. Validation & Error Matrix
- Missing connector profile balance -> connector balance fields are null; candidate usage is independent.
- No reliable usage snapshot for candidate connector/group -> candidate usage fields are null, not zero.
- Candidate account changes -> upstream usage snapshot remains keyed by `connector_id + upstream_group_id`; local account priority does not affect candidate usage.
- Remote upstream profile failure during sync -> do not fail rate snapshot sync solely because balance refresh failed.
- Lightweight metrics refresh account-scoped usage limitation -> return a successful refresh result when balance refresh succeeds, include a sanitized `usage_error`, and preserve candidate usage fields for that connector.
- Lightweight metrics refresh on a connector with no snapshots -> balance may refresh, but candidate usage remains unavailable until a full sync creates snapshots.
- Invalid usage history date format -> `400 UPSTREAM_RELAY_INVALID_USAGE_DATE`.
- Usage history `start_date > end_date` -> `400 UPSTREAM_RELAY_INVALID_USAGE_DATE_RANGE`.

#### 5. Good/Base/Bad Cases
- Good: the connector tab shows `$12.34` synced at a timestamp for the upstream relay account.
- Good: the candidate tab shows today's locally aggregated `actual_cost` and four-token total for `connector_id + upstream_group_id`.
- Good: clicking "Refresh Usage / Balance" refreshes connector balance without changing group-rate snapshots, candidate usage, or producing rate-change history.
- Good: refreshing the same connector twice on the same local date updates one usage-history row per group.
- Base: a known group with no successful usage on a successful refresh stores zero in daily history.
- Base: a candidate with unavailable upstream usage displays a clear not-synced state.
- Bad: persisting a failed usage refresh as `actual_cost=0`, which makes unknown usage look like real zero usage.
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
- Migration/repository test: usage history table has the daily connector/group unique key and upsert overwrites same-day rows.
- Service test: successful usage refresh persists daily history, zero known usage persists as zero, and failed/unknown usage does not persist zero rows.
- Handler/routes test: `GET /usage-history` is registered and validates date filters.
- Frontend check: candidate API type excludes connector balance fields and includes today's usage fields; connector API type includes balance fields.
- Frontend check: candidate table renders the usage column, connector table renders account balance, connector row exposes a lightweight refresh button, and usage-history tab calls `listUsageHistory()`.

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

Wrong:
```sql
INSERT INTO upstream_relay_group_usage_history (usage_date, connector_id, upstream_group_id, actual_cost)
VALUES ($1, $2, $3, 0) -- used after an unavailable usage refresh
```

Correct:
```sql
INSERT INTO upstream_relay_group_usage_history (...)
VALUES (...)
ON CONFLICT (usage_date, connector_id, upstream_group_id) DO UPDATE SET actual_cost=EXCLUDED.actual_cost
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

### Scenario: User-facing external metric snapshot APIs

#### 1. Scope / Trigger
- Trigger: adding or changing an authenticated user-facing API that exposes third-party or public external metrics, such as GPT intelligence snapshots shown inside channel status pages.
- These APIs cross external collection, service normalization, handler/route registration, frontend API parsing, and UI error states. They need code-spec depth because a third-party payload change can otherwise break an unrelated local dashboard.

#### 2. Signatures
- Route pattern: `GET /api/v1/channel-monitors/<metric-name>` for metrics displayed with the user channel monitor surface.
- Handler pattern: check the same feature flag as the owning user surface, call a dedicated service method, and return with `response.Success` / `response.ErrorFrom`.
- Service pattern: expose `GetSnapshot(ctx)` and keep external HTTP collection behind the backend; do not let frontend call third-party metric URLs directly.
- Static metric routes must be registered before parameterized monitor routes such as `/:id/status`.

#### 3. Contracts
- Backend response must be a normalized DTO owned by this project, not a raw third-party payload.
- External requests must use a short timeout, a bounded response body, and an in-process cache or stronger cache to avoid high-frequency third-party traffic.
- Publicly rendered HTML may be parsed only for data already visible on the public page; do not bypass authorization-only APIs or embed third-party secrets in frontend code.
- Fields unavailable from the public source should be returned as nullable fields, not fabricated zeros.
- Cached snapshots returned to callers must be cloned when they contain mutable slices or pointer fields.
- Frontend API functions should request the backend route and then run runtime parsing/normalization at the API boundary.

#### 4. Validation & Error Matrix
- Feature disabled -> return a clear unavailable/not found error for the metric while the rest of the channel monitor page can still render.
- Third-party HTTP error, timeout, or invalid payload -> return service unavailable using a project error type.
- Missing public metric markers -> return service unavailable and keep the previous UI error state isolated to the metric panel.
- Legacy payload wrapper still present -> frontend parser may accept it for compatibility tests, but runtime fetching should still go through the backend.

#### 5. Good/Base/Bad Cases
- Good: `GET /api/v1/channel-monitors/gpt-intelligence` returns a normalized snapshot parsed from public HTML and cached for a TTL.
- Base: token breakdown is not present in the public source, so `total_tokens` and `output_tokens` are `null`.
- Bad: frontend fetches `https://third-party.example/current.json` directly and assumes a vendor-specific field exists.
- Bad: a static route is added after `/:id/status`, causing the metric name to be parsed as a monitor id.

#### 6. Tests Required
- Service test covers parsing the public payload and the unavailable path when expected markers are missing.
- Service test covers cache/snapshot clone behavior when mutable pointers or slices are returned.
- Handler/routes test proves the static metric route is registered before parameterized channel monitor routes.
- Frontend API test proves the backend endpoint path is used and legacy/direct normalized payloads parse correctly.

#### 7. Wrong vs Correct

Wrong:
```typescript
fetch('https://codexradar.com/current.json')
```

Correct:
```typescript
apiClient.get('/channel-monitors/gpt-intelligence')
```

Wrong:
```go
monitors.GET("/:id/status", h.ChannelMonitor.GetStatus)
monitors.GET("/gpt-intelligence", h.ChannelMonitor.GetGptIntelligence)
```

Correct:
```go
monitors.GET("/gpt-intelligence", h.ChannelMonitor.GetGptIntelligence)
monitors.GET("/:id/status", h.ChannelMonitor.GetStatus)
```

---

### Scenario: Admin lottery campaign management APIs

#### 1. Scope / Trigger
- Trigger: adding or changing Token lottery campaign management behavior, including edit, publish/cancel, featured activity-center selection, hard delete, draw scheduling, prize tiers, entries, batches, or winners.
- These APIs cross migrations, repository queries, service validation, admin handlers/routes, user activity-center selection, frontend API types, and admin UI. They need code-spec depth because contract drift can hide campaigns, break draw eligibility, or remove audit history.

#### 2. Signatures
- Admin route prefix: `/api/v1/admin/lottery-campaigns`.
- Required admin endpoints: `GET /`, `POST /`, `GET /:id`, `PUT /:id`, `DELETE /:id`, `POST /:id/publish`, `POST /:id/cancel`, `POST /:id/feature`, `POST /:id/sync-entries`, `POST /:id/draw`, `GET /:id/draw-batches`, `GET /:id/draw-batches/:batch_id/winners`.
- User route: `GET /api/v1/lottery-campaigns/active` returns the currently visible campaign for the activity center.
- Core tables: `lottery_campaigns`, `lottery_prize_tiers`, `lottery_entries`, `lottery_draw_batches`, `lottery_winners`.

#### 3. Contracts
- `LotteryCampaign` responses include snake_case fields matching frontend types, including `is_featured`.
- Published lottery campaigns remain editable through the same full config payload as draft campaigns.
- `POST /:id/feature` marks exactly one lottery campaign as featured; `/lottery-campaigns/active` prefers featured campaigns but still requires `status='published'` and current time inside `[start_at, end_at]`.
- Hard delete intentionally deletes the selected campaign and cascading lottery records; do not silently convert it to archive/cancel behavior.
- Prize tier edits must not break existing winner references. If historical winners can reference old tiers, preserve old tier rows through an active/inactive marker rather than deleting rows that may be referenced.

#### 4. Validation & Error Matrix
- Invalid campaign id -> `LOTTERY_CAMPAIGN_NOT_FOUND` / not found style error.
- Feature a campaign that is not published or not currently in-window -> `LOTTERY_FEATURED_INVALID`.
- `end_at <= start_at` -> `LOTTERY_INVALID_CONFIG`.
- Single draw without `draw_at`, or with `draw_at` outside the campaign window -> `LOTTERY_INVALID_CONFIG`.
- Daily draw with invalid `HH:mm`, or no scheduled draw time inside the campaign window -> `LOTTERY_INVALID_CONFIG`.
- Empty prize tiers or non-positive winner/reward values -> `LOTTERY_INVALID_CONFIG`.

#### 5. Good/Base/Bad Cases
- Good: two published in-window lottery campaigns exist and the featured one is returned from `/lottery-campaigns/active`.
- Good: editing a published campaign replaces visible prize tiers while old winner rows can still resolve their historical prize tier.
- Base: no featured campaign is active, so the active endpoint falls back to deterministic time/id ordering.
- Bad: updating a published campaign is rejected solely because `status != 'draft'`.
- Bad: deleting prize tiers during edit causes existing `lottery_winners.prize_tier_id` rows to violate foreign keys.

#### 6. Tests Required
- Service tests cover time-window validation for single and daily draw campaigns, including one-day windows.
- Service test covers feature validation for published/in-window versus future or unpublished campaigns.
- Repository/API test covers featured campaign ordering for the active query.
- Frontend API test covers admin delete and feature endpoint paths when those methods are introduced or changed.

#### 7. Wrong vs Correct

Wrong:
```sql
DELETE FROM lottery_prize_tiers WHERE campaign_id = $1
```

Correct:
```sql
UPDATE lottery_prize_tiers SET is_active = FALSE WHERE campaign_id = $1
```

Wrong:
```sql
ORDER BY start_at ASC, id ASC
```

Correct:
```sql
ORDER BY is_featured DESC, start_at ASC, id ASC
```

---

### Scenario: Upstream relay monitoring partial usage refresh

#### 1. Scope / Trigger
- Trigger: changing upstream relay connector metrics refresh, candidate API-key bindings, daily usage snapshots, Runner finalization errors, or the admin monitoring result UI.
- This flow crosses repository snapshot writes, service aggregation, admin JSON responses, frontend API types, and user-facing recovery guidance.

#### 2. Signatures
- Aggregate refresh: `POST /api/v1/admin/upstream-relay-group-monitors/refresh`.
- Connector metrics refresh: `POST /api/v1/admin/upstream-relay-group-monitors/connectors/:id/metrics/refresh`.
- Repository write: `UpdateSnapshotTodayUsage(ctx, connectorID, usageByGroup, checkedAt, complete)`; `complete=false` updates known groups only.
- Usage detail response includes `status`, `total_groups`, `updated_groups`, `missing_groups`, compatibility field `issue`, full list `issues`, and `checked_at`.

#### 3. Contracts
- Refresh each candidate API-key binding independently. A missing key or one upstream request failure records a structured issue and does not stop other valid bindings.
- `issues` is the complete issue list; `issue` mirrors the first item for compatibility. Stable issue fields are `code`, `candidate_id`, `account_id`, and `upstream_group_id`; `message` is technical detail.
- When any binding for a group fails, omit that group's aggregate from `usageByGroup` so incomplete totals cannot overwrite the last known snapshot.
- Partial snapshot/history writes update known groups only and preserve unknown groups. A complete refresh may write zero for snapshot groups with no usage.
- Runner finalization failures expose `connector_id`, `connector_name`, `date`, and `reason`; raw reasons remain technical detail in the UI.

#### 4. Validation & Error Matrix
- No candidate bindings -> `usage_detail.status=skipped`, issue code `no_candidate_bindings`, no existing usage snapshot is cleared.
- Candidate missing API key -> issue code `missing_upstream_api_key_binding`; other valid groups continue.
- Upstream key usage request fails -> issue code `upstream_usage_request_failed`; other valid keys continue.
- Bound Key is no longer visible -> issue code `upstream_api_key_not_visible`; Key has no current group -> `upstream_api_key_group_unavailable`. Do not silently fall back to the saved candidate group.
- Some groups updated and some missing -> `usage_detail.status=partial`.
- No groups updated and at least one group failed -> `usage_detail.status=failed`.
- Snapshot missing for an otherwise valid binding -> `missing_groups[].reason=no_snapshot`; request a full connector sync.

#### 5. Good/Base/Bad Cases
- Good: group A lacks a key, group B refreshes successfully, the response reports one updated and one skipped group, and group B is persisted.
- Base: a connector has no candidate bindings; balance may refresh while usage is skipped without clearing old values.
- Bad: returning on the first malformed candidate and losing all valid groups in the same connector.
- Bad: writing an empty map as a complete refresh and replacing previously known usage with zero.

#### 6. Tests Required
- Service test covers one invalid candidate plus one valid candidate and asserts the valid group is fetched and persisted.
- Service test covers total usage failure and asserts an empty partial write has a check time while the prior snapshot cost, Token count, and check time remain unchanged.
- API/frontend types cover nullable collections and both `issue` / `issues` fields.
- View tests cover localized cause, impact, next step, repair entry, repair-triggered refresh, and folded technical details.
- Runner test asserts structured failed connector/date/reason data survives through `Status()`.

#### 7. Wrong vs Correct

Wrong:
```go
if binding.UpstreamAPIKeyID == 0 {
    return nil, fmt.Errorf("missing key")
}
```

Correct:
```go
issues = append(issues, UpstreamRelayMetricsIssueDetail{
    Code: upstreamRelayMetricsIssueMissingAPIKeyBinding,
    CandidateID: binding.CandidateID,
    AccountID: binding.AccountID,
    UpstreamGroupID: binding.UpstreamGroupID,
})
continue
```

Wrong:
```go
UpdateSnapshotTodayUsage(ctx, connectorID, partialUsage, checkedAt, true)
```

Correct:
```go
UpdateSnapshotTodayUsage(ctx, connectorID, partialUsage, checkedAt, false)
```

---

### Scenario: Admin announcement read-status sorting

#### 1. Scope / Trigger
- Trigger: changing the admin announcement read-status list ordering or pagination.

#### 2. Signatures
- Route: `GET /api/v1/admin/announcements/:id/read-status`.
- Default query: `sort_by=read_at&sort_order=desc`.
- Repository entrypoint: `UserRepository.ListWithFilters`, with `UserListFilters.ReadStatusAnnouncementID` set to the route announcement ID.

#### 3. Contracts
- Read users sort by `announcement_reads.read_at` in the requested direction; unread users always follow read users.
- Ordering is applied before `OFFSET/LIMIT` and ties use `users.id ASC`.
- The join matches both `announcement_id` and `user_id`; the response schema remains unchanged.

#### 4. Validation & Error Matrix
- Missing or non-positive announcement ID -> existing handler validation error.
- `sort_by=read_at` without `ReadStatusAnnouncementID` -> normal user-list fallback ordering, without an unscoped read join.

#### 5. Good/Base/Bad Cases
- Good: page 1 contains the most recently read users and unread users begin only after all read users.
- Base: when nobody has read the announcement, users remain stable by ID.
- Bad: fetch a page by email and sort only that page by `read_at` in the service or frontend.

#### 6. Tests Required
- Handler test asserts the `read_at desc` defaults and announcement ID propagation.
- Repository query test asserts the scoped left join, nulls-last expression, stable tie-break, and ordering before pagination.
- Frontend test asserts the initial API request uses `read_at desc`.

#### 7. Wrong vs Correct

Wrong:
```go
sort.Slice(pageUsers, byReadAt)
```

Correct:
```sql
ORDER BY ar.read_at IS NULL, ar.read_at DESC, users.id ASC LIMIT $1 OFFSET $2
```

---

### Scenario: Admin balance calibration accounting dates

#### 1. Scope / Trigger
- Trigger: changing admin usage calibration creation, balance-spend aggregation, user usage sorting, or calibration daily allocations.

#### 2. Signatures
- Audit total: `admin_usage_calibrations.balance_delta NUMERIC(18,6)` and `created_at` as the operation time.
- Direct spend audit: `consumption_mode`, `consumption_input_value`, `consumption_before_value`, `consumption_after_value`, `consumption_delta`, and the `consumption_*` date/timezone fields.
- Daily attribution: `admin_usage_calibration_daily_allocations.balance_delta NUMERIC(18,6)` and `allocation_date` as the accounting date.
- Repository reads: `SumBalanceSpent`, `SumBalanceSpentByUsers`, and `sumBalanceCalibrationsByTimeRange`.

#### 3. Contracts
- A combined Token and balance calibration allocates the balance delta by each day's original Token share.
- Allocation uses integer micro-units and deterministic largest remainders; daily values sum exactly to the audit total.
- Date-filtered statistics read daily balance allocations first. A main audit row is read by `created_at` only when no non-null daily balance allocation exists.
- Legacy balance calibration only treats negative balance deltas as spend; positive legacy balance deltas do not affect spend.
- For a direct spend calibration, signed `consumption_delta` always affects reported spend and `balance_delta = -consumption_delta` updates the wallet in the opposite direction.
- Spend target mode calculates from authoritative range spend inside the repository transaction; frontend preview values are informational only.

#### 4. Validation & Error Matrix
- Token range has no original usage -> reject through `ADMIN_USAGE_CALIBRATION_NO_ORIGINAL_USAGE`; do not create a balance allocation with guessed weights.
- Daily allocation exists -> exclude the matching main row with `NOT EXISTS`, otherwise the adjustment is counted twice.
- Legacy balance-only row has no daily allocation -> retain the `created_at` fallback.
- Spend decrease below zero or spend increase beyond available wallet balance -> reject the whole transaction.

#### 5. Good/Base/Bad Cases
- Good: a historical three-day combined calibration appears on those three accounting dates and not on the operation date.
- Good: setting range spend from `10` to `14` records `consumption_delta=4`, `balance_delta=-4`, and deducts wallet balance by `4`.
- Base: a balance-only legacy calibration remains visible in the operation-time range.
- Bad: filter every balance calibration directly on `admin_usage_calibrations.created_at`.

#### 6. Tests Required
- Assert positive and negative micro-unit allocation, deterministic remainder order, and exact total preservation.
- Assert a combined calibration writes daily balance deltas in the same transaction as the audit and wallet update.
- Assert single-user, batch-user, leaderboard detail, and user-list sorting use daily attribution without main-row duplication.

#### 7. Wrong vs Correct

Wrong:
```sql
SELECT SUM(-balance_delta) FROM admin_usage_calibrations WHERE created_at >= $1;
```

Correct:
```sql
SELECT balance_delta FROM admin_usage_calibration_daily_allocations WHERE allocation_date >= $1::date
UNION ALL SELECT balance_delta FROM admin_usage_calibrations WHERE NOT EXISTS (...);
```

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
