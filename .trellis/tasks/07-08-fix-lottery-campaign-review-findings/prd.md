# fix lottery campaign review findings

## Goal

Fix the second-round review findings for Token lottery campaigns so draws cannot happen before their scheduled time, lottery payouts refresh balance caches consistently with existing admin grants, and user enrollment cannot mutate a closed draw window.

## What I Already Know

- The lottery campaign feature is currently implemented in backend service/repository/handler files and related frontend activity-center files.
- Second-round review identified three actionable backend risks:
  - Manual draw can create a successful batch before the scheduled draw time.
  - Lottery payout updates `users.balance` directly without invalidating auth or billing balance caches.
  - User `/me` and `/enroll` can write entries for campaigns or draw windows that should no longer accept entries.
- Existing admin balance grants invalidate caches after a successful transaction.
- This downstream fork requires task context to include `.trellis/spec/guides/downstream-fork-workflow.md` for implement and check phases.

## Assumptions

- No UI redesign is needed for this fix; backend will enforce the invariants.
- Manual draw should mean "draw now if due", not "force early draw".
- Existing failed/partial/stale processing batches may still be retried after their scheduled time.
- `/me` may keep returning read data for a campaign id, but it should not create/update entries unless the activity is published, active, and the current window accepts entries.

## Requirements

- Reject non-recovery draws when `scheduled_draw_at` is in the future.
- Preserve retry behavior for failed, partial, and stale processing batches.
- After a successful lottery balance grant, invalidate the same user balance/auth caches used by admin balance grants.
- Prevent user entry writes for unpublished, inactive, ended, or closed draw windows.
- Prevent post-draw enrollment from affecting recovered batches.
- Add focused regression tests for the fixed behavior.

## Acceptance Criteria

- [ ] Future manual draw requests fail and do not create a draw batch.
- [ ] Due draws and eligible retry draws still work.
- [ ] Successful lottery payout invalidates affected balance caches.
- [ ] Duplicate/idempotent payout does not duplicate cache invalidation.
- [ ] User enrollment after a single draw time is rejected.
- [ ] User `/me` does not upsert entries when the activity is not write-eligible.
- [ ] Targeted backend tests pass.

## Definition of Done

- Tests added or updated for service/repository behavior.
- Targeted Go tests pass within the normal local timeout.
- No unrelated worktree changes are reverted.
- Downstream fork workflow context remains attached to implement/check records.

## Out of Scope

- Frontend redesign or new controls for forced early draw.
- Database schema changes unless proven necessary.
- Public winner-list changes.

## Technical Notes

- Backend specs read:
  - `.trellis/spec/backend/index.md`
  - `.trellis/spec/backend/quality-guidelines.md`
  - `.trellis/spec/guides/index.md`
  - `.trellis/spec/guides/downstream-fork-workflow.md`
- Current branch is `feature/user-activity-center`; task metadata uses base `custom/main` and branch `fix/lottery-campaign-review-findings`.
- `origin/main...upstream/main` currently reports `0 436`; upstream push URL is disabled.
