# Lottery Campaign Time Adjustment

## Goal

Allow admins to fully manage Token lottery campaigns after creation: edit full configuration after publishing, configure short windows such as one-day campaigns, delete/retire campaigns safely, and explicitly choose which campaign appears in the user activity center.

## What I already know

- The user wants to adjust lottery start and end time, including one-day lottery campaigns.
- User activity center shows lottery campaigns only when `/lottery-campaigns/active` returns a campaign.
- The active lottery query requires `status = 'published'`, `start_at <= now`, and `end_at >= now`.
- Publishing a lottery currently only changes status to `published`; it does not change `start_at`.
- The admin create dialog already has `start_at` and `end_at` inputs, but the admin lottery panel has no visible edit action even though the API module exposes `updateLotteryCampaign(id, payload)`.
- The invite campaign admin page has an edit dialog pattern for timeline fields that can guide the lottery admin UX.
- Admin lottery routes currently expose list/create/get/update/publish/cancel/sync/draw/batches/winners, but no `DELETE /admin/lottery-campaigns/:id`.
- Lottery-related tables use `ON DELETE CASCADE` from `lottery_campaigns` to prize tiers, entries, draw batches, and winners, so a hard delete would remove operational/audit history unless guarded.
- When multiple published lottery campaigns are active at the same time, the user active endpoint picks one by `ORDER BY start_at ASC, id ASC`; there is no explicit "current/featured campaign" switch.

## Assumptions (temporary)

- Admins should be able to edit all configuration fields after publish, including campaigns that already have entries or draws.
- Existing draw behavior must stay consistent: single-draw campaigns need `draw_at`, daily campaigns need `daily_draw_time`.
- If multiple published campaigns overlap, the explicitly selected campaign should win for user activity center display and participation.

## Open Questions

- None.

## Requirements (evolving)

- Add an admin-facing way to edit the full lottery campaign configuration after creation.
- The edit form should support name, description, rules, participation mode, draw schedule, prize mode, entry mode, token thresholds, entry limits, start/end time, draw time, daily draw time, and prize tiers.
- Published campaigns must remain fully editable; the system should not lock fields merely because the campaign is published, has entries, or has draw history.
- Add an admin-facing hard delete action for lottery campaigns.
- Hard delete must be allowed for any lottery campaign, including campaigns with entries, draw batches, winners, or balance grant audit rows.
- The UI must clearly warn admins that hard delete removes报名、开奖、中奖和余额发放审计记录 through cascading deletes.
- Add an admin-facing way to explicitly set which published in-window lottery campaign is featured in the user activity center.
- User participation endpoints should use the featured campaign when the user enters from activity center, and campaign-specific endpoints should continue to operate by campaign id.
- Support short campaigns, including a one-day window.
- Keep activity center visibility derived from the active window rules.
- Validate that `end_at` is after `start_at` and draw schedule settings remain compatible with the configured time window.
- Avoid changing production behavior outside lottery campaign management.

## Acceptance Criteria (evolving)

- [ ] Admin can open an existing lottery campaign and edit full campaign configuration.
- [ ] Saving changes calls the existing admin update endpoint or an equivalent backend contract.
- [ ] Published campaigns can be edited across all exposed fields without forcing cancellation/recreation.
- [ ] A published campaign whose updated window contains current time appears in the user activity center.
- [ ] A published campaign whose window is in the future remains hidden until `start_at`.
- [ ] A one-day campaign can be configured without manual database edits.
- [ ] Invalid time order or incompatible draw settings are blocked with a clear error.
- [ ] Admin can hard delete any lottery campaign.
- [ ] Hard delete removes the campaign and its cascading related records.
- [ ] The delete UI warns that related报名、开奖、中奖和余额发放审计记录 will be deleted.
- [ ] Admin can explicitly choose one eligible lottery campaign as the activity-center featured campaign.
- [ ] When multiple published in-window campaigns exist, `/lottery-campaigns/active` returns the featured one.
- [ ] zh/en i18n covers the edit action, dialog, validation, and success/error messages.

## Definition of Done

- Tests added or updated for the touched frontend/backend behavior where appropriate.
- Frontend typecheck and relevant backend tests pass.
- i18n keys updated for new labels/actions/messages.
- Task context keeps downstream fork workflow in implement/check JSONL.

## Out of Scope (explicit)

- Showing upcoming lottery campaigns to users before `start_at`.
- Changing winner drawing, entry eligibility, or balance grant rules unless required by time validation.
- Bulk migration or cleanup of existing lottery history beyond the selected campaign hard delete.
- Production database or deployment changes.

## Technical Notes

- Relevant files inspected:
  - `frontend/src/components/admin/activities/LotteryCampaignAdminPanel.vue`
  - `frontend/src/views/user/CampaignRewardsView.vue`
  - `frontend/src/api/admin/lotteryCampaigns.ts`
  - `backend/internal/service/lottery_campaign_service.go`
  - `backend/internal/repository/lottery_campaign_repo.go`
  - `backend/internal/handler/admin/lottery_campaign_handler.go`
- Required project guide: `.trellis/spec/guides/downstream-fork-workflow.md`.
- Relevant specs added to task context: frontend type safety and backend quality guidelines.

## Expansion Sweep

### Future evolution

- Lottery campaigns may later need pause/resume, templates, duplicate campaign, or separate "upcoming" display.
- Preserving the existing full update payload shape avoids inventing a second partial-edit contract unless validation risk requires it.

### Related scenarios

- Create and edit should stay consistent for start/end/draw schedule fields.
- Invite campaign admin already offers an edit dialog for timeline changes, so lottery should feel similar.

### Failure and edge cases

- Editing a published campaign to a future start should intentionally hide it from user activity center until that time.
- Because the product decision is "published campaigns remain fully editable", edits may change the meaning of existing entries/draw history; the UI should make the action explicit, and tests should preserve the intended backend contract.
- Hard deleting a campaign with entries, draw batches, or winners intentionally removes audit history because related tables cascade.
- Multiple overlapping published campaigns need deterministic visibility through an explicit featured/active selection.

## Decision (ADR-lite)

**Context**: The initial issue was inability to adjust the active window, but admins may need to revise the whole lottery setup after creating it.

**Decision**: Implement a full edit flow for existing lottery campaigns rather than a time-only editor.

**Consequences**: The UX is more flexible and can reuse the create payload shape, but implementation must define safe boundaries for published campaigns and campaigns that already have entries or draw results.

### Published campaign editing

**Context**: Admins need operational flexibility after publish, even if the campaign is already visible.

**Decision**: Published lottery campaigns remain fully editable across the same configuration surface as draft campaigns.

**Consequences**: This favors operational speed over immutable audit semantics. Existing entries/draws may have been generated under previous settings, so the implementation should keep campaign-specific history visible and avoid silently deleting history during edit.

### User activity center switching

**Context**: Multiple lottery campaigns may exist and overlap, but the user activity center currently auto-selects by earliest start time.

**Decision**: Add an explicit admin control to choose which eligible lottery campaign is shown/used by the user activity center.

**Consequences**: The backend needs a persisted featured selection or equivalent deterministic selector. The active endpoint should prefer that selection while still requiring the selected campaign to be published and within its active time window.

### Hard delete

**Context**: Admins want maximum freedom to remove lottery campaigns, even after entries or draws exist.

**Decision**: Allow hard delete for any lottery campaign.

**Consequences**: Related prize tiers, entries, draw batches, winners, and balance grant audit records are removed through existing cascading relationships. The UI must make this destructive behavior explicit before deletion.

## Technical Approach

- Reuse the existing admin update endpoint shape for full campaign editing where possible, adding frontend edit UI and any missing backend validation/tests.
- Add a persisted featured lottery campaign selector so the user active endpoint can prefer the admin-selected campaign when it is `published` and currently in-window.
- Add an admin hard-delete endpoint and frontend action with a destructive confirmation message.
- Keep user-facing activity-center behavior active-only: upcoming selected campaigns do not appear until their `start_at`.

## Implementation Plan

- Backend: add repository/service/handler support for featured selection and hard delete, and update active campaign selection logic.
- Frontend API/types: add delete and feature/switch methods while keeping request/response fields aligned with backend JSON.
- Admin UI: add edit, delete, and set-as-activity-center actions to the lottery campaign panel, with zh/en i18n.
- Verification: add or update targeted backend service/repository tests and frontend type/API tests where existing patterns allow.
